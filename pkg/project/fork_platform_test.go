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
	if pkg.Dependencies["vite"] != "7.3.7" {
		t.Fatal("generated platform must retain Vite for Remix config loading")
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
	p := &Project{root: dir, config: filepath.Join(dir, "sst.config.ts"), app: &App{Providers: map[string]interface{}{}}}
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
	if err := p.Install(); err != nil {
		t.Fatal(err)
	}
	if p.NeedsInstall() {
		t.Fatal("clean installed platform still needs installation")
	}
	// The generated manifest must not be allowed to resolve a dependency outside
	// the reviewed runtime lock, even if a valid install receipt already exists.
	manifestPath := filepath.Join(p.PathPlatformDir(), "package.json")
	original, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatal(err)
	}
	var manifest map[string]interface{}
	if err := json.Unmarshal(original, &manifest); err != nil {
		t.Fatal(err)
	}
	manifest["dependencies"].(map[string]interface{})["typescript"] = "5.7.3"
	changed, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(manifestPath, changed, 0644); err != nil {
		t.Fatal(err)
	}
	if err := p.fetchDeps(); err == nil || !strings.Contains(err.Error(), "lockfile") {
		t.Fatalf("unreviewed dependency resolution was not rejected: %v", err)
	}
	if !p.NeedsInstall() {
		t.Fatal("rejected install retained a completion receipt")
	}
	if err := os.WriteFile(manifestPath, original, 0644); err != nil {
		t.Fatal(err)
	}
	if err := p.Install(); err != nil {
		t.Fatal("reviewed manifest must install again", err)
	}
	t.Log("generated platform:", p.PathPlatformDir())
}

func TestUnreviewedProviderRejected(t *testing.T) {
	p := &Project{root: t.TempDir(), app: &App{Providers: map[string]interface{}{}}}
	if err := p.CopyPlatform("4.17.2-forum.test"); err != nil {
		t.Fatal(err)
	}
	p.lock = ProviderLock{{Name: "aws", Package: "@pulumi/aws", Version: "7.21.0", Alias: "aws"}}
	if err := p.writePackageJson(); err == nil || !strings.Contains(err.Error(), "reviewed runtime lock") {
		t.Fatal("unreviewed provider version must fail closed", err)
	}
}

func TestInstallReceiptRequiresGeneratedMetadata(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell package manager fixture")
	}
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("node required for patch fixture")
	}
	for _, failure := range []string{"config.d.ts", "../provider-lock.json"} {
		t.Run(failure, func(t *testing.T) {
			root := t.TempDir()
			p := &Project{root: root, config: filepath.Join(root, "sst.config.ts"), app: &App{Providers: map[string]interface{}{}}}
			if err := p.CopyPlatform("4.17.2-forum.test"); err != nil {
				t.Fatal(err)
			}
			sdk := filepath.Join(p.PathPlatformDir(), "node_modules", "@pulumi", "pulumi")
			if err := os.MkdirAll(sdk, 0755); err != nil {
				t.Fatal(err)
			}
			for name, body := range map[string]string{
				filepath.Join(sdk, "package.json"):                  `{"version":"` + strings.TrimPrefix(global.PULUMI_VERSION, "v") + `"}`,
				filepath.Join(p.PathPlatformDir(), ".fork-install"): global.PULUMI_VERSION,
				// This test isolates metadata failures after successful dependency/patch work.
				filepath.Join(p.PathPlatformDir(), "scripts", "patch-pulumi-tracing.cjs"): "process.exit(0);",
			} {
				if err := os.WriteFile(name, []byte(body), 0644); err != nil {
					t.Fatal(err)
				}
			}
			bun := filepath.Join(root, "bun")
			if err := os.WriteFile(bun, []byte("#!/bin/sh\nif [ \"$1\" = --version ]; then echo "+global.BUN_VERSION+"; fi\nexit 0\n"), 0755); err != nil {
				t.Fatal(err)
			}
			oldBun := flag.SST_BUN_PATH
			t.Cleanup(func() { flag.SST_BUN_PATH = oldBun })
			flag.SST_BUN_PATH = bun
			blockedPath := filepath.Join(p.PathPlatformDir(), failure)
			if err := os.Mkdir(blockedPath, 0755); err != nil {
				t.Fatal(err)
			}
			if err := p.Install(); err == nil {
				t.Fatal("metadata write failure was ignored")
			}
			if !p.NeedsInstall() {
				t.Fatal("incomplete metadata retained an installation receipt")
			}
			if err := os.Remove(blockedPath); err != nil {
				t.Fatal(err)
			}
			if err := p.Install(); err != nil {
				t.Fatal("retry after metadata failure", err)
			}
			if p.NeedsInstall() {
				t.Fatal("complete installation did not write its receipt")
			}
		})
	}
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
