package global

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// Exercises the real engine's plugin resolution with a deliberately incompatible
// language host FIRST on PATH. Only a temporary local backend and preview are
// used; no resources or cloud providers are declared or installed.
func TestPulumiConflictingLanguageHost(t *testing.T) {
	engine := os.Getenv("SST_TEST_PULUMI")
	sdk := os.Getenv("SST_TEST_PLATFORM")
	if engine == "" || sdk == "" {
		t.Skip("set SST_TEST_PULUMI and SST_TEST_PLATFORM for real engine regression")
	}
	if runtime.GOOS == "windows" {
		t.Skip("shell conflict fixture")
	}
	engine, err := filepath.Abs(engine)
	if err != nil {
		t.Fatal(err)
	}
	if err := ValidatePulumi(engine); err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	oldConfig := configDir
	t.Cleanup(func() { configDir = oldConfig })
	configDir = filepath.Join(root, "home")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatal(err)
	}
	conflict := filepath.Join(root, "conflict")
	if err := os.MkdirAll(conflict, 0755); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(root, "ambient-host-invoked")
	if err := os.WriteFile(filepath.Join(conflict, "pulumi-language-nodejs"), []byte("#!/bin/sh\necho invoked > \"$SST_TEST_HOST_MARKER\"\necho 'flag provided but not defined: -root (conflicting host)' >&2\nexit 42\n"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "Pulumi.yaml"), []byte("name: forum-host-selection\nruntime: nodejs\nmain: index.js\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "package.json"), []byte(`{"name":"forum-host-selection","private":true}`), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "index.js"), []byte(`const pulumi = require("@pulumi/pulumi"); exports.control = pulumi.output("no-cloud-resources"); exports.secret = pulumi.secret("fixture-secret");`), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(sdk, "node_modules"), filepath.Join(root, "node_modules")); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", conflict+string(os.PathListSeparator)+os.Getenv("PATH"))
	env := PulumiEnv(append(os.Environ(), "PULUMI_BACKEND_URL=file://"+filepath.ToSlash(root), "PULUMI_CONFIG_PASSPHRASE=fixture-only", "PULUMI_DISABLE_AUTOMATIC_PLUGIN_ACQUISITION=true", "SST_TEST_HOST_MARKER="+marker))
	run := func(environment []string, args ...string) ([]byte, error) {
		cmd := exec.Command(engine, args...)
		cmd.Dir = root
		cmd.Env = environment
		return cmd.CombinedOutput()
	}
	if out, err := run(env, "stack", "init", "test", "--non-interactive"); err != nil {
		t.Fatalf("local stack init: %s: %v", out, err)
	}
	// Positive: actual bundled host runs the program despite the conflicting PATH.
	if out, err := run(env, "preview", "--stack", "test", "--non-interactive"); err != nil {
		t.Fatalf("bundled host preview: %s: %v", out, err)
	} else {
		t.Log(string(out))
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatal("ambient host ran with protected environment")
	}
	// Negative control proves the bad host is executable and would be selected.
	control := append([]string{}, env...)
	for i, entry := range control {
		if strings.HasPrefix(entry, "PULUMI_IGNORE_AMBIENT_PLUGINS=") {
			control[i] = "PULUMI_IGNORE_AMBIENT_PLUGINS=false"
		}
	}
	if out, err := run(control, "preview", "--stack", "test", "--non-interactive"); err == nil {
		t.Fatalf("control unexpectedly succeeded: %s", out)
	}
	if _, err := os.Stat(marker); err != nil {
		t.Fatalf("negative control did not invoke conflicting host: %v", err)
	}
}
