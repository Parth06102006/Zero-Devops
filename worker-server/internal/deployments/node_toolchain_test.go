package deployments

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNodeVersionSelection(t *testing.T) {
	for _, tc := range []struct {
		engine, pin, want string
		fail              bool
	}{
		{"", "", "24", false},
		{">=22.23.4", "", "24", false},
		{"^22.23.4", "", "22", false},
		{"^20.19.0 || >=22.12.0", "", "24", false},
		{">=20 <22", "", "20", false},
		{">=22.23.4", "22.23.4", "22.23.4", false},
		{">=22.23.4", "20.20.2", "", true},
		{">=26", "", "", true},
		{"shell; command", "", "", true},
		{"", "lts/*", "", true},
	} {
		t.Run(tc.engine+"/"+tc.pin, func(t *testing.T) {
			dir := t.TempDir()
			if tc.pin != "" {
				writeFile(t, dir, ".node-version", tc.pin)
			}
			got, err := selectNodeVersion(dir, tc.engine)
			if tc.fail {
				if err == nil {
					t.Fatal("invalid version accepted")
				}
				return
			}
			if err != nil || got != tc.want {
				t.Fatalf("version=%q want=%q err=%v", got, tc.want, err)
			}
		})
	}
}

func TestNodeToolchainChecksBothInstallStages(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "package.json", `{"engines":{"node":">=22.23.4"}}`)
	got, err := renderNodeToolchain(dir, nodeTemplateFixture)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(got, "FROM docker.io/library/node:24-alpine") != 2 {
		t.Fatalf("wrong images: %s", got)
	}
	if strings.Count(got, "does not satisfy engines.node") != 2 {
		t.Fatalf("missing actual-version validation: %s", got)
	}
	if strings.Contains(got, "node:20") {
		t.Fatal("old builder remains")
	}
}

func TestNodeToolchainPreservesBunBuilder(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "package.json", `{"engines":{"node":">=22.23.4"}}`)
	got, err := renderNodeToolchain(dir, renderNodePackageManagerTemplate(nodeTemplateFixture, pkgManagerBun))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "FROM docker.io/oven/bun:1-alpine AS builder") || strings.Contains(got, "npm/node_modules/semver") {
		t.Fatalf("Bun compatibility regressed: %s", got)
	}
}

func TestNodeVersionPinConflictsAndEscapes(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, ".node-version", "24.0.0")
	writeFile(t, dir, ".nvmrc", "22.23.4")
	if _, err := selectNodeVersion(dir, ""); err == nil {
		t.Fatal("conflicting pins accepted")
	}
	outside := t.TempDir()
	writeFile(t, outside, ".node-version", "24.0.0")
	inside := t.TempDir()
	if err := os.Symlink(filepath.Join(outside, ".node-version"), filepath.Join(inside, ".node-version")); err != nil {
		t.Fatal(err)
	}
	if _, err := selectNodeVersion(inside, ""); err == nil {
		t.Fatal("Node pin symlink escape accepted")
	}
}
