package deployments

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Run only in an isolated Buildah store, never in a developer's shared store.
// These fixtures prove compilation, exact binary selection, and CLI startup;
// they do not prove HTTP readiness or deployment of a real application.
func TestNativeBuildIntegration(t *testing.T) {
	if os.Getenv("RUN_NATIVE_BUILD_TESTS") != "true" {
		t.Skip("set RUN_NATIVE_BUILD_TESTS=true in an isolated Buildah environment")
	}
	for _, name := range []string{"go-explicit-target", "rust-standalone", "rust-workspace"} {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			app := root
			var builder *Builder
			switch name {
			case "go-explicit-target":
				app = filepath.Join(root, "server")
				writeFile(t, app, "go.mod", "module example.com/fixture\ngo 1.26.0\n")
				writeFile(t, app, "cmd/server/main.go", "package main\nimport \"fmt\"\nfunc main() {fmt.Println(\"chosen-go-server\")}")
				writeFile(t, app, "zero-devops.build.json", `{"build_context":".","go_package":"./cmd/server"}`)
				builder = &Builder{Name: langGo, Template: "dynamic/Dockerfile.go.tmpl"}
			case "rust-standalone":
				writeFile(t, app, "Cargo.toml", "[package]\nname=\"server\"\nversion=\"0.1.0\"\nedition=\"2021\"\n")
				writeFile(t, app, "src/main.rs", "fn main() {println!(\"wrong-binary\");}")
				writeFile(t, app, "src/bin/admin.rs", "fn main() {println!(\"chosen-rust-admin\");}")
				writeFile(t, app, "zero-devops.build.json", `{"cargo_binary":"admin"}`)
				builder = &Builder{Name: langRust, Template: "dynamic/Dockerfile.rust.tmpl"}
			case "rust-workspace":
				writeFile(t, root, "Cargo.toml", "[workspace]\nmembers=[\"apps/web\",\"shared\"]\nresolver=\"2\"\n[workspace.package]\nversion=\"0.1.0\"\nedition=\"2021\"\n")
				writeFile(t, root, "shared/Cargo.toml", "[package]\nname=\"shared\"\nversion.workspace=true\nedition.workspace=true\n")
				writeFile(t, root, "shared/src/lib.rs", "pub fn message() -> &'static str {\"chosen-workspace-server\"}")
				app = filepath.Join(root, "apps", "web")
				writeFile(t, app, "Cargo.toml", "[package]\nname=\"web\"\nversion.workspace=true\nedition.workspace=true\n[dependencies]\nshared={path=\"../../shared\"}\n")
				writeFile(t, app, "src/main.rs", "fn main() {println!(\"{}\",shared::message());}")
				builder = &Builder{Name: langRust, Template: "dynamic/Dockerfile.rust.tmpl"}
			}
			if err := writeDockerfile(app, builder, pkgManagerNPM); err != nil {
				t.Fatal(err)
			}
			contextDir, err := prepareBuildContext(root, app, builder)
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
			defer cancel()
			tag := "localhost/zdfix-native-" + name + ":test"
			if output, err := buildImageWithContext(ctx, app, contextDir, tag); err != nil {
				t.Fatalf("build: %v\n%s", err, output)
			}
			container := "zdfix-fixture-" + name
			if output, err := exec.CommandContext(ctx, "buildah", "from", "--name", container, tag).CombinedOutput(); err != nil {
				t.Fatalf("from: %v\n%s", err, output)
			}
			defer func() {
				_ = exec.Command("buildah", "rm", container).Run()
				_ = exec.Command("buildah", "rmi", tag).Run()
			}()
			binary := "/app/server"
			want := map[string]string{"go-explicit-target": "chosen-go-server", "rust-standalone": "chosen-rust-admin", "rust-workspace": "chosen-workspace-server"}[name]
			if name == "go-explicit-target" {
				binary = "/app/app"
			}
			output, err := exec.CommandContext(ctx, "buildah", "run", container, "--", binary).CombinedOutput()
			if err != nil || strings.TrimSpace(string(output)) != want {
				t.Fatalf("runtime: %v, got %q want %q", err, output, want)
			}
		})
	}
}
