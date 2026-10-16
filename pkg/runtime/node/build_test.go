package node

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sst/sst/v3/pkg/js"
	"github.com/sst/sst/v3/pkg/runtime"
)

func TestResolveInstallVersion(t *testing.T) {
	tests := []struct {
		name    string
		pkg     string
		install map[string]string
		setup   func(t *testing.T) (string, js.PackageJson)
		want    string
		wantErr string
	}{
		{
			name:    "explicit version overrides package json",
			pkg:     "sharp",
			install: map[string]string{"sharp": "0.33.5"},
			setup: func(t *testing.T) (string, js.PackageJson) {
				return "", js.PackageJson{Dependencies: map[string]string{"sharp": "0.32.6"}}
			},
			want: "0.33.5",
		},
		{
			name:    "wildcard falls back to package json",
			pkg:     "sharp",
			install: map[string]string{"sharp": "*"},
			setup: func(t *testing.T) (string, js.PackageJson) {
				return "", js.PackageJson{Dependencies: map[string]string{"sharp": "0.32.6"}}
			},
			want: "0.32.6",
		},
		{
			name:    "missing package falls back to wildcard",
			pkg:     "sharp",
			install: map[string]string{"sharp": "*"},
			setup: func(t *testing.T) (string, js.PackageJson) {
				return "", js.PackageJson{}
			},
			want: "*",
		},
		{
			name:    "catalog spec resolves from pnpm workspace",
			pkg:     "sharp",
			install: map[string]string{"sharp": "*"},
			setup: func(t *testing.T) (string, js.PackageJson) {
				dir := t.TempDir()
				pkgDir := filepath.Join(dir, "packages", "functions")
				mustMkdirAll(t, pkgDir)
				mustWriteFile(t, filepath.Join(dir, "pnpm-workspace.yaml"), "catalogs:\n  shared:\n    sharp: 0.32.6\n")
				return pkgDir, js.PackageJson{Dependencies: map[string]string{"sharp": "catalog:shared"}}
			},
			want: "0.32.6",
		},
		{
			name:    "explicit catalog spec resolves from pnpm workspace",
			pkg:     "sharp",
			install: map[string]string{"sharp": "catalog:shared"},
			setup: func(t *testing.T) (string, js.PackageJson) {
				dir := t.TempDir()
				pkgDir := filepath.Join(dir, "packages", "functions")
				mustMkdirAll(t, pkgDir)
				mustWriteFile(t, filepath.Join(dir, "pnpm-workspace.yaml"), "catalogs:\n  shared:\n    sharp: 0.32.6\n")
				return pkgDir, js.PackageJson{}
			},
			want: "0.32.6",
		},
		{
			name:    "missing catalog entry returns error",
			pkg:     "sharp",
			install: map[string]string{"sharp": "*"},
			setup: func(t *testing.T) (string, js.PackageJson) {
				dir := t.TempDir()
				pkgDir := filepath.Join(dir, "packages", "functions")
				mustMkdirAll(t, pkgDir)
				mustWriteFile(t, filepath.Join(dir, "pnpm-workspace.yaml"), "catalogs:\n  shared:\n    other: 1.0.0\n")
				return pkgDir, js.PackageJson{Dependencies: map[string]string{"sharp": "catalog:shared"}}
			},
			wantErr: "no matching catalog entry exists",
		},
		{
			name:    "catalog spec resolves from bun top level catalog",
			pkg:     "sharp",
			install: map[string]string{"sharp": "*"},
			setup: func(t *testing.T) (string, js.PackageJson) {
				dir := t.TempDir()
				pkgDir := filepath.Join(dir, "packages", "functions")
				mustMkdirAll(t, pkgDir)
				mustWriteFile(t, filepath.Join(dir, "package.json"), `{
	"catalog": {
		"sharp": "0.32.6"
	}
}`)
				return pkgDir, js.PackageJson{Dependencies: map[string]string{"sharp": "catalog:"}}
			},
			want: "0.32.6",
		},
		{
			name:    "explicit catalog spec resolves from bun top level catalogs",
			pkg:     "sharp",
			install: map[string]string{"sharp": "catalog:shared"},
			setup: func(t *testing.T) (string, js.PackageJson) {
				dir := t.TempDir()
				pkgDir := filepath.Join(dir, "packages", "functions")
				mustMkdirAll(t, pkgDir)
				mustWriteFile(t, filepath.Join(dir, "package.json"), `{
	"catalogs": {
		"shared": {
			"sharp": "0.32.6"
		}
	}
}`)
				return pkgDir, js.PackageJson{}
			},
			want: "0.32.6",
		},
		{
			name:    "catalog spec resolves from bun workspaces catalog",
			pkg:     "sharp",
			install: map[string]string{"sharp": "*"},
			setup: func(t *testing.T) (string, js.PackageJson) {
				dir := t.TempDir()
				pkgDir := filepath.Join(dir, "packages", "functions")
				mustMkdirAll(t, pkgDir)
				mustWriteFile(t, filepath.Join(dir, "package.json"), `{
	"workspaces": {
		"packages": ["packages/*"],
		"catalog": {
			"sharp": "0.32.6"
		}
	}
}`)
				return pkgDir, js.PackageJson{Dependencies: map[string]string{"sharp": "catalog:"}}
			},
			want: "0.32.6",
		},
		{
			name:    "explicit catalog spec resolves from bun workspaces catalogs",
			pkg:     "sharp",
			install: map[string]string{"sharp": "catalog:shared"},
			setup: func(t *testing.T) (string, js.PackageJson) {
				dir := t.TempDir()
				pkgDir := filepath.Join(dir, "packages", "functions")
				mustMkdirAll(t, pkgDir)
				mustWriteFile(t, filepath.Join(dir, "package.json"), `{
	"workspaces": {
		"packages": ["packages/*"],
		"catalogs": {
			"shared": {
				"sharp": "0.32.6"
			}
		}
	}
}`)
				return pkgDir, js.PackageJson{}
			},
			want: "0.32.6",
		},
		{
			name:    "catalog spec resolves from bun top level catalog with workspaces array",
			pkg:     "sharp",
			install: map[string]string{"sharp": "*"},
			setup: func(t *testing.T) (string, js.PackageJson) {
				dir := t.TempDir()
				pkgDir := filepath.Join(dir, "packages", "functions")
				mustMkdirAll(t, pkgDir)
				mustWriteFile(t, filepath.Join(dir, "package.json"), `{
	"workspaces": ["packages/*"],
	"catalog": {
		"sharp": "0.32.6"
	}
}`)
				return pkgDir, js.PackageJson{Dependencies: map[string]string{"sharp": "catalog:"}}
			},
			want: "0.32.6",
		},
		{
			name:    "missing bun catalog entry returns error",
			pkg:     "sharp",
			install: map[string]string{"sharp": "*"},
			setup: func(t *testing.T) (string, js.PackageJson) {
				dir := t.TempDir()
				pkgDir := filepath.Join(dir, "packages", "functions")
				mustMkdirAll(t, pkgDir)
				mustWriteFile(t, filepath.Join(dir, "package.json"), `{
	"catalogs": {
		"shared": {
			"other": "1.0.0"
		}
	}
}`)
				return pkgDir, js.PackageJson{Dependencies: map[string]string{"sharp": "catalog:shared"}}
			},
			wantErr: "no matching catalog entry exists in",
		},
		{
			name:    "workspace spec returns error",
			pkg:     "sharp",
			install: map[string]string{"sharp": "*"},
			setup: func(t *testing.T) (string, js.PackageJson) {
				return "", js.PackageJson{Dependencies: map[string]string{"sharp": "workspace:^"}}
			},
			wantErr: "found \"workspace:^\" using \"workspace:\"",
		},
		{
			name:    "explicit workspace spec returns error",
			pkg:     "sharp",
			install: map[string]string{"sharp": "workspace:^"},
			setup: func(t *testing.T) (string, js.PackageJson) {
				return "", js.PackageJson{}
			},
			wantErr: "found \"workspace:^\" using \"workspace:\"",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir, packageJSON := tt.setup(t)
			got, err := resolveInstallVersion(tt.pkg, tt.install, dir, packageJSON)
			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("resolveInstallVersion() error = nil, want %q", tt.wantErr)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("resolveInstallVersion() error = %q, want substring %q", err.Error(), tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("resolveInstallVersion() error = %v", err)
			}
			if got != tt.want {
				t.Fatalf("resolveInstallVersion() = %q, want %q", got, tt.want)
			}
		})
	}
}

func mustMkdirAll(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
}

func mustWriteFile(t *testing.T, path string, contents string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
}

// Exercise the real bundler resolver, not just a version-comparison helper.
func TestBuildWithForkRuntimeSDK(t *testing.T) {
	forkVersion := "4.17.2+forum." + strings.Repeat("a", 40)
	if manifestPath := os.Getenv("SST_TEST_FORK_MANIFEST"); manifestPath != "" {
		data, err := os.ReadFile(manifestPath)
		if err != nil {
			t.Fatal(err)
		}
		var manifest struct{ Version string }
		if err := json.Unmarshal(data, &manifest); err != nil {
			t.Fatal(err)
		}
		forkVersion = manifest.Version
	}
	for _, tc := range []struct {
		name, cli, sdk string
		wantOK         bool
	}{
		{"fork base SDK", forkVersion, "4.17.2", true},
		{"fork old SDK", forkVersion, "4.17.0", false},
		{"fork newer SDK", forkVersion, "4.17.3", false},
		{"fork prerelease SDK", forkVersion, "4.17.2-beta.1", false},
		{"upstream exact", "4.17.2", "4.17.2", true},
		{"upstream mismatch", "4.17.2", "4.17.0", false},
		{"development", "dev", "4.17.0", true},
		{"malformed fork", "4.17.2+forum.short", "4.17.2", false},
		{"unrelated metadata", "4.17.2+other.test", "4.17.2", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			pkgDir := filepath.Join(dir, "node_modules", "sst")
			mustMkdirAll(t, pkgDir)
			mustWriteFile(t, filepath.Join(pkgDir, "package.json"), `{"name":"sst","version":"`+tc.sdk+`","type":"module","main":"index.js"}`)
			mustWriteFile(t, filepath.Join(pkgDir, "index.js"), `export const Resource = { Fixture: { name: "linked-resource" } };`)
			mustWriteFile(t, filepath.Join(dir, "handler.ts"), `import { Resource } from "sst"; export async function handler() { return Resource.Fixture.name; }`)
			result, err := New(tc.cli).Build(context.Background(), &runtime.BuildInput{
				CfgPath: filepath.Join(dir, "sst.config.ts"), FunctionID: "Fixture",
				Handler: "handler.handler", Runtime: "nodejs24.x",
			})
			if err != nil {
				t.Fatal(err)
			}
			if tc.wantOK {
				if len(result.Errors) != 0 {
					t.Fatalf("bundle failed: %v", result.Errors)
				}
				bundle, err := os.ReadFile(filepath.Join(dir, ".sst", "artifacts", "Fixture-src", "bundle.mjs"))
				if err != nil {
					t.Fatal(err)
				}
				if !strings.Contains(string(bundle), "linked-resource") {
					t.Fatal("SDK was not bundled")
				}
			} else if !strings.Contains(strings.Join(result.Errors, "\n"), "does not match the sst cli version") {
				t.Fatalf("expected version mismatch, got %v", result.Errors)
			}
		})
	}
}
