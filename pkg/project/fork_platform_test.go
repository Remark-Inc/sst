package project

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/sst/sst/v3/pkg/flag"
	"github.com/sst/sst/v3/pkg/global"
)

func TestForkPlatformUpgradeAndReinstall(t *testing.T) {
	p := &Project{root: t.TempDir(), app: &App{Providers: map[string]interface{}{}}}
	if err := p.CopyPlatform("4.17.2-forum.old"); err != nil {
		t.Fatal(err)
	}
	sdkDir := filepath.Join(p.PathPlatformDir(), "node_modules", "@pulumi", "pulumi")
	if err := os.MkdirAll(sdkDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sdkDir, "package.json"), []byte(`{"version":"3.215.0"}`), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(p.PathPlatformDir(), ".fork-install"), []byte(global.PULUMI_VERSION), 0644); err != nil {
		t.Fatal(err)
	}
	if !p.NeedsInstall() {
		t.Fatal("stale SDK did not trigger installation")
	}
	if p.CheckPlatform("4.17.2-forum.new") {
		t.Fatal("new artifact reused old embedded platform")
	}
	if err := p.CopyPlatform("4.17.2-forum.new"); err != nil {
		t.Fatal(err)
	}
	if !p.CheckPlatform("4.17.2-forum.new") || !p.NeedsInstall() {
		t.Fatal("upgrade must replace platform and require dependency installation")
	}
	p.lock = ProviderLock{{Name: "aws", Package: "@pulumi/aws", Version: "7.20.0", Alias: "aws"}}
	if err := p.writePackageJson(); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(p.PathPlatformDir(), "package.json"))
	if err != nil {
		t.Fatal(err)
	}
	var pkg struct {
		Dependencies    map[string]string      `json:"dependencies"`
		Overrides       map[string]interface{} `json:"overrides"`
		DevDependencies map[string]string      `json:"devDependencies"`
	}
	if err := json.Unmarshal(data, &pkg); err != nil {
		t.Fatal(err)
	}
	if len(pkg.DevDependencies) != 0 || pkg.Dependencies["@types/archiver"] == "" {
		t.Fatal("generated platform must retain declarations but omit repository test tools")
	}
	if pkg.Dependencies["@pulumi/pulumi"] != strings.TrimPrefix(global.PULUMI_VERSION, "v") || pkg.Dependencies["@pulumi/aws"] != "7.20.0" || len(pkg.Overrides) == 0 {
		t.Fatalf("generated manifest lost SDK/provider policy: %s", data)
	}
	if err := os.MkdirAll(sdkDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sdkDir, "package.json"), []byte(`{"version":"`+strings.TrimPrefix(global.PULUMI_VERSION, "v")+`"}`), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(p.PathPlatformDir(), ".fork-install"), []byte(global.PULUMI_VERSION), 0644); err != nil {
		t.Fatal(err)
	}
	p.lock = ProviderLock{}
	if p.NeedsInstall() {
		t.Fatal("matching installed SDK should be reused")
	}
	if err := os.Remove(filepath.Join(p.PathPlatformDir(), ".fork-install")); err != nil {
		t.Fatal(err)
	}
	if !p.NeedsInstall() {
		t.Fatal("interrupted patch must trigger reinstall")
	}
	if err := os.WriteFile(filepath.Join(p.PathPlatformDir(), ".fork-install"), []byte(global.PULUMI_VERSION), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(sdkDir, "package.json")); err != nil {
		t.Fatal(err)
	}
	if !p.NeedsInstall() {
		t.Fatal("incomplete installation should be repaired")
	}
}

// Opt-in evidence fixture: copies the embedded platform and runs the production
// manifest writer/installer. No project config evaluation, credentials or cloud.
func TestGeneratedPlatformInstall(t *testing.T) {
	dir := os.Getenv("SST_TEST_GENERATED_PLATFORM")
	if dir == "" {
		t.Skip("set SST_TEST_GENERATED_PLATFORM to a fresh absolute directory")
	}
	if !filepath.IsAbs(dir) {
		t.Fatal("fixture directory must be absolute")
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatal("fixture directory must not exist")
	}
	p := &Project{root: dir, app: &App{Providers: map[string]interface{}{}}}
	if err := p.CopyPlatform("4.17.2-forum.validation"); err != nil {
		t.Fatal(err)
	}
	if err := p.writePackageJson(); err != nil {
		t.Fatal(err)
	}
	oldBun := flag.SST_BUN_PATH
	t.Cleanup(func() { flag.SST_BUN_PATH = oldBun })
	if bun := os.Getenv("SST_TEST_BUN"); bun != "" {
		flag.SST_BUN_PATH = bun
	}
	if err := p.fetchDeps(); err != nil {
		t.Fatal(err)
	}
	if p.NeedsInstall() {
		t.Fatal("clean installed platform still needs installation")
	}
	t.Log("generated platform:", p.PathPlatformDir())
}

func TestFailedForkReinstallInvalidatesReceipt(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell installer fixture")
	}
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("node required for patch failure fixture")
	}
	for _, failure := range []string{"package manager", "tracing patch"} {
		t.Run(failure, func(t *testing.T) {
			p := &Project{root: t.TempDir(), app: &App{Providers: map[string]interface{}{}}}
			if err := p.CopyPlatform("4.17.2-forum.test"); err != nil {
				t.Fatal(err)
			}
			sdk := filepath.Join(p.PathPlatformDir(), "node_modules", "@pulumi", "pulumi")
			if err := os.MkdirAll(sdk, 0755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(sdk, "package.json"), []byte(`{"version":"`+strings.TrimPrefix(global.PULUMI_VERSION, "v")+`"}`), 0644); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(p.PathPlatformDir(), ".fork-install"), []byte(global.PULUMI_VERSION), 0644); err != nil {
				t.Fatal(err)
			}
			if p.NeedsInstall() {
				t.Fatal("fixture should initially be reusable")
			}
			installer := filepath.Join(t.TempDir(), "bun")
			versionLine := "#!/bin/sh\nif [ \"$1\" = --version ]; then echo " + global.BUN_VERSION + "; exit 0; fi\n"
			body := versionLine + "exit 0\n"
			if failure == "package manager" {
				body = versionLine + "exit 1\n"
			}
			if err := os.WriteFile(installer, []byte(body), 0755); err != nil {
				t.Fatal(err)
			}
			oldBun, oldNoBun := flag.SST_BUN_PATH, flag.SST_NO_BUN
			t.Cleanup(func() { flag.SST_BUN_PATH, flag.SST_NO_BUN = oldBun, oldNoBun })
			flag.SST_BUN_PATH, flag.SST_NO_BUN = installer, false
			if err := os.WriteFile(filepath.Join(p.PathPlatformDir(), "scripts", "patch-pulumi-tracing.cjs"), []byte(`throw new Error("simulated patch failure");`), 0644); err != nil {
				t.Fatal(err)
			}
			if err := p.fetchDeps(); err == nil {
				t.Fatal("reinstall unexpectedly succeeded")
			}
			if !p.NeedsInstall() {
				t.Fatal("failed reinstall reused the stale completion receipt")
			}
		})
	}
}

func TestForkRejectsNpmFallback(t *testing.T) {
	old := flag.SST_NO_BUN
	t.Cleanup(func() { flag.SST_NO_BUN = old })
	flag.SST_NO_BUN = true
	p := &Project{root: t.TempDir()}
	if err := p.fetchDeps(); err == nil || !strings.Contains(err.Error(), "embedded lockfile") {
		t.Fatalf("expected reproducibility error, got %v", err)
	}
}
