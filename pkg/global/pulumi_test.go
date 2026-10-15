package global

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/sst/sst/v3/pkg/flag"
	"github.com/sst/sst/v3/pkg/process"
)

func fakePulumi(t *testing.T, dir, version string) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("shell fixture")
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"pulumi", "pulumi-language-nodejs"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("#!/bin/sh\necho "+version+"\n"), 0755); err != nil {
			t.Fatal(err)
		}
	}
	return filepath.Join(dir, "pulumi")
}

func TestPulumiSelectionAndRepair(t *testing.T) {
	oldConfig, oldOverride := configDir, flag.SST_PULUMI_PATH
	t.Cleanup(func() { configDir, flag.SST_PULUMI_PATH = oldConfig, oldOverride })
	configDir, flag.SST_PULUMI_PATH = t.TempDir(), ""
	legacy := fakePulumi(t, BinPath(), "v3.215.0")
	if PulumiPath() == legacy || !NeedsPulumi() {
		t.Fatal("legacy shared install must not satisfy versioned install")
	}
	engine := fakePulumi(t, filepath.Dir(PulumiPath()), PULUMI_VERSION)
	if NeedsPulumi() {
		t.Fatal("matching pair should be reused")
	}
	host := filepath.Join(filepath.Dir(engine), "pulumi-language-nodejs")
	if err := os.Remove(host); err != nil {
		t.Fatal(err)
	}
	if !NeedsPulumi() {
		t.Fatal("missing host must trigger repair")
	}
	fakePulumi(t, filepath.Dir(engine), "v3.215.0")
	if !NeedsPulumi() {
		t.Fatal("old engine/host must trigger repair")
	}
	fakePulumi(t, filepath.Dir(engine), PULUMI_VERSION)
	if err := os.WriteFile(host, []byte("#!/bin/sh\necho v3.999.0\n"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := ValidatePulumi(engine); err == nil {
		t.Fatal("mixed engine/host accepted")
	}
	override := fakePulumi(t, filepath.Join(t.TempDir(), "custom"), PULUMI_VERSION)
	flag.SST_PULUMI_PATH = override
	if PulumiPath() != override || NeedsPulumi() {
		t.Fatal("override must be used, never overwritten")
	}
	if err := InstallPulumi(context.Background()); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(t.TempDir(), "pulumi")
	if err := os.Symlink(override, link); err != nil {
		t.Fatal(err)
	}
	if err := ValidatePulumi(link); err != nil {
		t.Fatal(err)
	}
	fakePulumi(t, filepath.Dir(override), "v3.215.0")
	if err := InstallPulumi(context.Background()); err == nil {
		t.Fatal("mismatched override must fail without downloading")
	}
}

func TestPulumiEnvOverridesConflicts(t *testing.T) {
	env := PulumiEnv([]string{"PATH=/conflicting/bin", "PULUMI_IGNORE_AMBIENT_PLUGINS=false", "PULUMI_HOME=/other", "PULUMI_SKIP_UPDATE_CHECK=false", "SST_SECRET_TEST=retained"})
	joined := strings.Join(env, "\n")
	if strings.Contains(joined, "=false") || strings.Contains(joined, "=/other") || !strings.Contains(joined, "PULUMI_IGNORE_AMBIENT_PLUGINS=true") || !strings.Contains(joined, "SST_SECRET_TEST=retained") {
		t.Fatal(env)
	}
	if strings.Count(joined, "PULUMI_HOME=") != 1 {
		t.Fatal(env)
	}
}

type pulumiTransport func(*http.Request) (*http.Response, error)

func (f pulumiTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestInstallPulumiVerifiedBundle(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("tar/shell fixture")
	}
	oldConfig, oldOverride, oldChecksums, oldClient := configDir, flag.SST_PULUMI_PATH, pulumiChecksums, http.DefaultClient
	t.Cleanup(func() {
		configDir, flag.SST_PULUMI_PATH, pulumiChecksums, http.DefaultClient = oldConfig, oldOverride, oldChecksums, oldClient
	})
	configDir, flag.SST_PULUMI_PATH = t.TempDir(), ""
	var archive bytes.Buffer
	gz := gzip.NewWriter(&archive)
	tw := tar.NewWriter(gz)
	for _, name := range []string{"pulumi", "pulumi-language-nodejs"} {
		body := []byte("#!/bin/sh\necho " + PULUMI_VERSION + "\n")
		if err := tw.WriteHeader(&tar.Header{Name: "pulumi/" + name, Mode: 0755, Size: int64(len(body))}); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write(body); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	arch := "arm64"
	if runtime.GOARCH == "amd64" {
		arch = "x64"
	}
	filename := fmt.Sprintf("pulumi-%s-%s-%s.tar.gz", PULUMI_VERSION, runtime.GOOS, arch)
	pulumiChecksums = fmt.Sprintf("%x  %s\n", sha256.Sum256(archive.Bytes()), filename)
	http.DefaultClient = &http.Client{Transport: pulumiTransport(func(r *http.Request) (*http.Response, error) {
		if !strings.HasSuffix(r.URL.Path, filename) {
			t.Fatalf("unexpected URL %s", r.URL)
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(bytes.NewReader(archive.Bytes())), Header: make(http.Header)}, nil
	})}
	if err := InstallPulumi(context.Background()); err != nil {
		t.Fatal(err)
	}
	if NeedsPulumi() {
		t.Fatal("installed pair is invalid")
	}
	// Damaged same-version installs get the full pair, not a partial overwrite.
	if err := os.Remove(filepath.Join(filepath.Dir(PulumiPath()), "pulumi-language-nodejs")); err != nil {
		t.Fatal(err)
	}
	if err := InstallPulumi(context.Background()); err != nil {
		t.Fatal(err)
	}
	if NeedsPulumi() {
		t.Fatal("repair failed")
	}
	// A valid installation is reused even if another installer changed the
	// available checksum; corruption of a repair download must fail closed.
	pulumiChecksums = strings.Repeat("0", 64) + "  " + filename
	if err := InstallPulumi(context.Background()); err != nil {
		t.Fatal("valid distribution should be reused", err)
	}
	hostPath := filepath.Join(filepath.Dir(PulumiPath()), "pulumi-language-nodejs")
	if err := os.Remove(hostPath); err != nil {
		t.Fatal(err)
	}
	if err := InstallPulumi(context.Background()); err == nil {
		t.Fatal("bad checksum accepted")
	}
	if _, err := os.Stat(PulumiPath()); err != nil {
		t.Fatal("failed repair removed existing engine", err)
	}
}

func TestPulumiRelativeOverride(t *testing.T) {
	old := flag.SST_PULUMI_PATH
	t.Cleanup(func() { flag.SST_PULUMI_PATH = old })
	path := fakePulumi(t, t.TempDir(), PULUMI_VERSION)
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	relative, err := filepath.Rel(cwd, path)
	if err != nil {
		t.Fatal(err)
	}
	flag.SST_PULUMI_PATH = relative
	if PulumiPath() != path {
		t.Fatalf("override not absolute: %s", PulumiPath())
	}
	cmd := process.Command(PulumiPath(), "version")
	cmd.Dir = t.TempDir()
	if out, err := cmd.Output(); err != nil || strings.TrimSpace(string(out)) != PULUMI_VERSION {
		t.Fatalf("override failed after changing workdir: %s %v", out, err)
	}
}

func TestPromotePulumiInstallFailureRecovery(t *testing.T) {
	for _, failure := range []string{"backup", "promotion", "rollback", "none"} {
		t.Run(failure, func(t *testing.T) {
			root := t.TempDir()
			staged, target := filepath.Join(root, "staged"), filepath.Join(root, "target")
			for dir, content := range map[string]string{staged: "new", target: "previous"} {
				if err := os.Mkdir(dir, 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(dir, "engine"), []byte(content), 0644); err != nil {
					t.Fatal(err)
				}
			}
			rename := func(from, to string) error {
				if (failure == "backup" && from == target) ||
					((failure == "promotion" || failure == "rollback") && from == staged) ||
					(failure == "rollback" && from == staged+".previous") {
					return fmt.Errorf("injected rename failure")
				}
				return os.Rename(from, to)
			}
			err := promotePulumiInstall(staged, target, rename)
			read := func(dir, want string) {
				data, err := os.ReadFile(filepath.Join(dir, "engine"))
				if err != nil || string(data) != want {
					t.Fatalf("%s: got %q, %v; want %q", dir, data, err, want)
				}
			}
			if failure == "none" {
				if err != nil {
					t.Fatal(err)
				}
				read(target, "new")
				if _, err := os.Stat(staged + ".previous"); !os.IsNotExist(err) {
					t.Fatal("successful promotion retained obsolete backup", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), staged) {
				t.Fatal("failure must report retained replacement path", err)
			}
			read(staged, "new")
			if failure == "rollback" {
				read(staged+".previous", "previous")
				if !strings.Contains(err.Error(), staged+".previous") {
					t.Fatal("rollback error must report recovery path", err)
				}
			} else {
				read(target, "previous")
				// A retry after a transient failure can promote the retained copy.
				if err := promotePulumiInstall(staged, target, os.Rename); err != nil {
					t.Fatal(err)
				}
				read(target, "new")
			}
		})
	}
}

func TestPulumiInstallLockCancellation(t *testing.T) {
	oldConfig, oldOverride := configDir, flag.SST_PULUMI_PATH
	t.Cleanup(func() { configDir, flag.SST_PULUMI_PATH = oldConfig, oldOverride })
	configDir, flag.SST_PULUMI_PATH = t.TempDir(), ""
	lockPath := filepath.Dir(PulumiPath()) + ".lock"
	if err := os.MkdirAll(lockPath, 0755); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := InstallPulumi(ctx); err == nil || !strings.Contains(err.Error(), "install lock") {
		t.Fatalf("expected lock cancellation, got %v", err)
	}
	if _, err := os.Stat(lockPath); err != nil {
		t.Fatal("removed another installer's lock")
	}
}
