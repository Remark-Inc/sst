package global

import (
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	_ "embed"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/sst/sst/v3/pkg/flag"
	"github.com/sst/sst/v3/pkg/process"
	"github.com/sst/sst/v3/pkg/task"
)

// Pulumi checksums are reviewed with the coordinated SDK/engine update.
//
//go:embed pulumi-checksums.txt
var pulumiChecksums string

// PulumiEnv applies to every engine invocation, including version and stack init.
// The engine resolves bundled language plugins next to its real executable when
// ambient plugins are disabled (Pulumi workspace.getPluginInfoAndPath).
func PulumiEnv(env []string) []string {
	result := make([]string, 0, len(env)+3)
	for _, entry := range env {
		key, _, _ := strings.Cut(entry, "=")
		if key == "PULUMI_HOME" || key == "PULUMI_IGNORE_AMBIENT_PLUGINS" || key == "PULUMI_SKIP_UPDATE_CHECK" {
			continue
		}
		result = append(result, entry)
	}
	return append(result, "PULUMI_HOME="+ConfigDir(), "PULUMI_IGNORE_AMBIENT_PLUGINS=true", "PULUMI_SKIP_UPDATE_CHECK=true")
}

// ValidatePulumi rejects incomplete or mixed installations, including overrides.
// Resolve symlinks as the engine does, so its sibling host is the one validated.
func ValidatePulumi(path string) error {
	realPath, err := filepath.EvalSymlinks(path)
	if err != nil {
		return fmt.Errorf("resolve Pulumi installation: %w", err)
	}
	for _, check := range []struct {
		path string
		arg  string
	}{
		{realPath, "version"},
		{filepath.Join(filepath.Dir(realPath), executableName("pulumi-language-nodejs")), "--version"},
	} {
		cmd := process.Command(check.path, check.arg)
		cmd.Env = PulumiEnv(os.Environ())
		output, err := cmd.Output()
		if err != nil {
			return fmt.Errorf("check %s: %w; install the complete Pulumi %s distribution", check.path, err, PULUMI_VERSION)
		}
		if strings.TrimPrefix(strings.TrimSpace(string(output)), "v") != strings.TrimPrefix(PULUMI_VERSION, "v") {
			return fmt.Errorf("%s must be Pulumi %s (got %q)", check.path, PULUMI_VERSION, strings.TrimSpace(string(output)))
		}
	}
	return nil
}

func NeedsPulumi() bool {
	// Never replace an explicitly supplied distribution. Run validates it even if
	// SST_SKIP_DEPENDENCY_CHECK is set.
	return flag.SST_PULUMI_PATH == "" && ValidatePulumi(PulumiPath()) != nil
}

func InstallPulumi(ctx context.Context) error {
	if flag.SST_PULUMI_PATH != "" {
		return ValidatePulumi(PulumiPath())
	}
	slog.Info("pulumi install")
	// Serialize upgrades/repairs across CLI processes. Do not remove a version
	// another process has just installed and may already be executing.
	target := filepath.Dir(PulumiPath())
	if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
		return err
	}
	lockPath := target + ".lock"
	waitCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	for {
		err := os.Mkdir(lockPath, 0755)
		if err == nil {
			break
		}
		if !os.IsExist(err) {
			return err
		}
		select {
		case <-waitCtx.Done():
			return fmt.Errorf("waiting for Pulumi install lock %s: %w (if no installer is running, remove the stale lock directory)", lockPath, waitCtx.Err())
		case <-time.After(100 * time.Millisecond):
		}
	}
	defer os.Remove(lockPath)
	if ValidatePulumi(PulumiPath()) == nil {
		return nil
	}
	var osArch string

	switch runtime.GOOS {
	case "darwin":
		osArch = "darwin"
	case "linux":
		osArch = "linux"
	case "windows":
		osArch = "windows"
	default:
		return fmt.Errorf("unsupported operating system")
	}

	switch runtime.GOARCH {
	case "amd64":
		osArch += "-x64"
	case "arm64":
		osArch += "-arm64"
	default:
		return fmt.Errorf("unsupported architecture: " + runtime.GOARCH)
	}

	fileExtension := ".tar.gz"
	if runtime.GOOS == "windows" {
		fileExtension = ".zip"
	}

	_, err := task.Run(ctx, func() (any, error) {
		filename := fmt.Sprintf("pulumi-%s-%s%s", PULUMI_VERSION, osArch, fileExtension)
		checksum := ""
		for _, line := range strings.Split(pulumiChecksums, "\n") {
			fields := strings.Fields(line)
			if len(fields) == 2 && fields[1] == filename {
				checksum = fields[0]
			}
		}
		if checksum == "" {
			return nil, fmt.Errorf("missing reviewed checksum for %s", filename)
		}
		url := fmt.Sprintf("https://github.com/pulumi/pulumi/releases/download/%s/%s", PULUMI_VERSION, filename)
		slog.Info("pulumi downloading", "url", url)

		request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return nil, err
		}
		resp, err := http.DefaultClient.Do(request)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("failed to download pulumi: HTTP status %d", resp.StatusCode)
		}

		target := filepath.Dir(PulumiPath())
		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return nil, err
		}
		tmp, err := os.MkdirTemp(filepath.Dir(target), ".install-")
		if err != nil {
			return nil, err
		}
		defer os.RemoveAll(tmp)
		archive, err := os.CreateTemp(filepath.Dir(target), ".archive-")
		if err != nil {
			return nil, err
		}
		defer os.Remove(archive.Name())
		defer archive.Close()
		hash := sha256.New()
		if _, err := io.Copy(io.MultiWriter(archive, hash), resp.Body); err != nil {
			return nil, err
		}
		if fmt.Sprintf("%x", hash.Sum(nil)) != checksum {
			return nil, fmt.Errorf("checksum mismatch for %s", filename)
		}
		if _, err := archive.Seek(0, 0); err != nil {
			return nil, err
		}
		switch fileExtension {
		case ".tar.gz":
			gzr, err := gzip.NewReader(archive)
			if err != nil {
				return nil, err
			}
			defer gzr.Close()
			err = untar(gzr, tmp)
			if err != nil {
				return nil, err
			}

		case ".zip":
			// Read zip file
			zipBytes, err := io.ReadAll(archive)
			if err != nil {
				return nil, err
			}

			// Open zip archive
			zipReader, err := zip.NewReader(bytes.NewReader(zipBytes), int64(len(zipBytes)))
			if err != nil {
				return nil, err
			}

			// Extract all files to tmp root
			for _, file := range zipReader.File {
				if file.FileInfo().IsDir() {
					continue
				}

				// Strip pulumi/bin/ prefix from filename
				filename := filepath.Base(file.Name)
				destPath := filepath.Join(tmp, filename)

				rc, err := file.Open()
				if err != nil {
					return nil, err
				}

				outFile, err := os.Create(destPath)
				if err != nil {
					rc.Close()
					return nil, err
				}

				_, err = io.Copy(outFile, rc)
				rc.Close()
				outFile.Close()
				if err != nil {
					return nil, err
				}
			}

		default:
			return nil, fmt.Errorf("unsupported file extension: %s", fileExtension)
		}

		if err := ValidatePulumi(filepath.Join(tmp, executableName("pulumi"))); err != nil {
			return nil, err
		}
		// A previous SDK uses a different directory, so upgrades cannot leave a new
		// engine beside an old language host. Only replace an invalid same-version install.
		if err := os.RemoveAll(target); err != nil {
			return nil, err
		}
		if err := os.Rename(tmp, target); err != nil {
			return nil, err
		}
		return nil, nil
	})
	return err
}

func executableName(name string) string {
	if runtime.GOOS == "windows" {
		return name + ".exe"
	}
	return name
}

func PulumiPath() string {
	if flag.SST_PULUMI_PATH != "" {
		// Commands run from Pulumi's temporary working directory, not the caller's.
		absolute, err := filepath.Abs(flag.SST_PULUMI_PATH)
		if err == nil {
			return absolute
		}
		return flag.SST_PULUMI_PATH
	}
	return filepath.Join(BinPath(), "pulumi-releases", PULUMI_VERSION, executableName("pulumi"))
}
