package deployments

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNPMLockfilePolicy(t *testing.T) {
	for _, lock := range []string{"", "package-lock.json", "npm-shrinkwrap.json"} {
		for _, optIn := range []string{"", "true"} {
			t.Run(lock+"/optIn="+optIn, func(t *testing.T) {
				dir := t.TempDir()
				t.Setenv("ALLOW_UNLOCKED_NPM_INSTALL", optIn)
				if lock != "" {
					writeFile(t, dir, lock, "{}")
				}
				got, err := renderInstallPolicy(dir, nodeTemplateFixture, pkgManagerNPM)
				if lock == "" && optIn == "" {
					if err == nil || !strings.Contains(err.Error(), "ALLOW_UNLOCKED_NPM_INSTALL=true") || !strings.Contains(err.Error(), "non-reproducible") {
						t.Fatalf("missing actionable refusal: %v", err)
					}
					return
				}
				if err != nil {
					t.Fatal(err)
				}
				if lock != "" {
					if !strings.Contains(got, "npm ci --ignore-scripts --omit=dev") || strings.Contains(got, "npm install") {
						t.Fatalf("frozen policy lost: %s", got)
					}
					if lock == "npm-shrinkwrap.json" && !strings.Contains(got, "COPY package.json npm-shrinkwrap.json ./") {
						t.Fatal("shrinkwrap not copied")
					}
				} else {
					if strings.Contains(got, "npm ci") || !strings.Contains(got, "npm install --ignore-scripts --package-lock=false --omit=dev") {
						t.Fatalf("wrong unlocked recipe: %s", got)
					}
					if _, err := os.Stat(filepath.Join(dir, "package-lock.json")); !os.IsNotExist(err) {
						t.Fatal("checkout lockfile was created")
					}
				}
			})
		}
	}
}

func TestSupportedLockfilesKeepFrozenInstalls(t *testing.T) {
	for _, tc := range []struct{ lock, pm, command string }{
		{"pnpm-lock.yaml", pkgManagerPNPM, "pnpm install --frozen-lockfile --ignore-scripts"},
		{"yarn.lock", pkgManagerYarn, "yarn install --frozen-lockfile --ignore-scripts"},
		{"bun.lock", pkgManagerBun, "bun install --frozen-lockfile --ignore-scripts"},
		{"bun.lockb", pkgManagerBun, "bun install --frozen-lockfile --ignore-scripts"},
		{"npm-shrinkwrap.json", pkgManagerNPM, "npm ci --ignore-scripts"},
	} {
		t.Run(tc.lock, func(t *testing.T) {
			dir := t.TempDir()
			writeFile(t, dir, tc.lock, "{}")
			if got := detectPackageManager(dir); got != tc.pm {
				t.Fatalf("manager=%s", got)
			}
			got, err := renderInstallPolicy(dir, renderNodePackageManagerTemplate(nodeTemplateFixture, tc.pm), tc.pm)
			if err != nil || !strings.Contains(got, tc.command) {
				t.Fatalf("%v: %s", err, got)
			}
		})
	}
}

func TestGoVersionAndTargetSelection(t *testing.T) {
	for _, tc := range []struct {
		mod, version string
		fail         bool
	}{
		{"module example.com/a\ngo 1.26.0\n", "1.26.0", false},
		{"module example.com/a\ngo 1.25\ntoolchain go1.26.1\n", "1.26.1", false},
		{"module example.com/a\ngo 1.26.1\ntoolchain go1.25.2\n", "1.26.1", false},
		{"module example.com/a\ngo 1.25\ntoolchain default\n", "1.25", false},
		{"module example.com/a\ngo latest\n", "", true},
		{"module example.com/a\ngo 1.25\ntoolchain go1.26rc1\n", "", true},
		{"module example.com/a\ngo 1.25\ntoolchain\tgo1.26rc1\n", "", true},
		{"module example.com/a\ngo 1.25\ngo 1.26\n", "", true},
	} {
		t.Run(tc.mod, func(t *testing.T) {
			dir := t.TempDir()
			writeFile(t, dir, "go.mod", tc.mod)
			version, err := goImageVersion(dir)
			if tc.fail {
				if err == nil {
					t.Fatal("expected invalid version refusal")
				}
				return
			}
			if err != nil || version != tc.version {
				t.Fatalf("version=%q err=%v", version, err)
			}
		})
	}
	dir := t.TempDir()
	writeFile(t, dir, "go.mod", "module example.com/a\ngo 1.26.0\n")
	writeFile(t, dir, "cmd/server/main.go", "package main\nfunc main() {}")
	writeFile(t, dir, "zero-devops.build.json", `{"go_package":"./cmd/server"}`)
	if err := writeDockerfile(dir, &Builder{Name: langGo, Template: "dynamic/Dockerfile.go.tmpl"}, pkgManagerNPM); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "Dockerfile"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"docker.io/library/golang:1.26.0-alpine", `"./cmd/server"`, "GOTOOLCHAIN=local"} {
		if !strings.Contains(string(data), want) {
			t.Fatalf("missing %s: %s", want, data)
		}
	}
}

func TestGoSeparateRepositoryContext(t *testing.T) {
	root := t.TempDir()
	app := filepath.Join(root, "server")
	writeFile(t, app, "go.mod", "module example.com/a\ngo 1.26.0\n")
	writeFile(t, app, "cmd/server/main.go", "package main\nfunc main() {}")
	writeFile(t, app, "zero-devops.build.json", `{"build_context":".","go_package":"./cmd/server"}`)
	builder := &Builder{Name: langGo, Template: "dynamic/Dockerfile.go.tmpl"}
	if err := writeDockerfile(app, builder, pkgManagerNPM); err != nil {
		t.Fatal(err)
	}
	contextDir, err := prepareBuildContext(root, app, builder)
	if err != nil || contextDir != root {
		t.Fatalf("%s: %v", contextDir, err)
	}
	data, _ := os.ReadFile(filepath.Join(app, "Dockerfile"))
	for _, want := range []string{"COPY . .\nWORKDIR /app/server\nRUN go mod download", `"-o","/app/app","./cmd/server"`} {
		if !strings.Contains(string(data), want) {
			t.Fatalf("missing %s: %s", want, data)
		}
	}
}

func TestGoTargetRejectsEscapesAndInjection(t *testing.T) {
	for _, target := range []string{"../outside", "/tmp", "./...", "-x", ".;touch /tmp/no", "cmd\\server"} {
		t.Run(target, func(t *testing.T) {
			dir := t.TempDir()
			writeFile(t, dir, "go.mod", "module example.com/a\ngo 1.26\n")
			// Quotes here are test data encoded through JSON, not shell input.
			data := "{\"go_package\":" + strconvQuote(target) + "}"
			writeFile(t, dir, "zero-devops.build.json", data)
			if _, err := renderGoBuild(dir, "__GO_BUILD__"); err == nil {
				t.Fatal("unsafe target accepted")
			}
		})
	}
}

func TestRustWorkspaceAndBinarySelection(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "Cargo.toml", "[workspace]\nmembers = [\"apps/web\", \"shared\"]\n[workspace.package]\nedition = \"2021\"\n")
	writeFile(t, root, "Cargo.lock", "version = 4\n")
	writeFile(t, root, "shared/Cargo.toml", "[package]\nname=\"shared\"\nversion=\"0.1.0\"\n")
	writeFile(t, root, "shared/src/lib.rs", "")
	app := filepath.Join(root, "apps", "web")
	writeFile(t, app, "Cargo.toml", "[package]\nname=\"web\"\nversion=\"0.1.0\"\nedition.workspace=true\n[dependencies]\nshared={path=\"../../shared\"}\n")
	writeFile(t, app, "src/main.rs", "fn main() {}")
	builder := &Builder{Name: langRust, Template: "dynamic/Dockerfile.rust.tmpl"}
	if err := writeDockerfile(app, builder, pkgManagerNPM); err != nil {
		t.Fatal(err)
	}
	contextDir, err := prepareBuildContext(root, app, builder)
	if err != nil || contextDir != root {
		t.Fatalf("context=%s err=%v", contextDir, err)
	}
	data, _ := os.ReadFile(filepath.Join(app, "Dockerfile"))
	for _, want := range []string{`"--manifest-path","apps/web/Cargo.toml"`, `"--package","web","--bin","web"`, `"--locked"`, "/app/.zerodevops-target/release/web"} {
		if !strings.Contains(string(data), want) {
			t.Fatalf("missing %s: %s", want, data)
		}
	}
	if strings.Contains(string(data), "find target") || strings.Contains(string(data), "__CARGO_") {
		t.Fatal("binary selection not deterministic")
	}
}

func TestRustStandaloneAmbiguousAndExplicitTargets(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "Cargo.toml", "[package]\nname=\"server\"\nversion=\"0.1.0\"\n")
	writeFile(t, root, "src/main.rs", "fn main() {}")
	m, err := readCargo(root, root)
	if err != nil {
		t.Fatal(err)
	}
	_, bin, err := cargoBinary(root, m, buildOptions{})
	if err != nil || bin != "server" {
		t.Fatalf("%s %v", bin, err)
	}
	writeFile(t, root, "src/bin/admin.rs", "fn main() {}")
	if _, _, err := cargoBinary(root, m, buildOptions{}); err == nil {
		t.Fatal("ambiguous binaries accepted")
	}
	_, bin, err = cargoBinary(root, m, buildOptions{CargoBinary: "admin"})
	if err != nil || bin != "admin" {
		t.Fatalf("%s %v", bin, err)
	}
	if _, _, err := cargoBinary(root, cargoManifest{}, buildOptions{}); err == nil {
		t.Fatal("virtual workspace accepted without target")
	}
	if _, _, err := cargoBinary(root, m, buildOptions{CargoBinary: "../escape"}); err == nil {
		t.Fatal("unsafe binary accepted")
	}
}

func TestContextRejectsTraversalAndSymlinks(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	writeFile(t, root, "app/go.mod", "module example.com/a\ngo 1.26\n")
	app := filepath.Join(root, "app")
	if err := os.Symlink(outside, filepath.Join(root, "escape")); err != nil {
		t.Fatal(err)
	}
	if err := requireContainedPath(root, filepath.Join(root, "escape")); err == nil {
		t.Fatal("symlink escape accepted")
	}
	if err := requireContainedPath(root, outside); err == nil {
		t.Fatal("outside path accepted")
	}
	writeFile(t, app, "zero-devops.build.json", `{"build_context":"../outside"}`)
	if _, err := prepareBuildContext(root, app, &Builder{Name: langGo}); err == nil {
		t.Fatal("context escape accepted")
	}
	writeFile(t, root, "Cargo.toml", "[package]\nname=\"a\"\nversion=\"0.1.0\"\n[dependencies]\nb={path=\"../outside\"}\n")
	if err := validateCargoPaths(root); err == nil {
		t.Fatal("Cargo dependency escape accepted")
	}
}

func TestBuildahReceivesSeparateContextAndDockerfile(t *testing.T) {
	dir := t.TempDir()
	app := filepath.Join(dir, "apps", "web")
	writeFile(t, dir, "fakebin/buildah", "#!/bin/sh\nprintf '%s\\n' \"$@\"\n")
	if err := os.Chmod(filepath.Join(dir, "fakebin", "buildah"), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", filepath.Join(dir, "fakebin")+string(os.PathListSeparator)+os.Getenv("PATH"))
	out, err := buildImageWithContext(context.Background(), app, dir, "test")
	if err != nil || !strings.Contains(out, "-f\n"+filepath.Join(app, "Dockerfile")+"\n"+dir+"\n") {
		t.Fatalf("%v: %s", err, out)
	}
}

func TestAllTemplateBaseImagesAreQualified(t *testing.T) {
	err := filepath.WalkDir("../../templates", func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for _, line := range strings.Split(string(data), "\n") {
			if strings.HasPrefix(line, "FROM ") {
				ref := strings.Fields(line)[1]
				registry := strings.Split(ref, "/")[0]
				if !strings.Contains(registry, ".") {
					t.Errorf("unqualified image in %s: %s", path, ref)
				}
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func strconvQuote(s string) string { data, _ := json.Marshal(s); return string(data) }
