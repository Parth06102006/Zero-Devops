package deployments

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeFile(t *testing.T, root, name, content string) {
	t.Helper()

	path := filepath.Join(root, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestDetectFrameworkByConfigFile(t *testing.T) {
	tests := []struct {
		name       string
		configFile string
		want       string
	}{
		{name: "vite", configFile: "vite.config.js", want: "vite"},
		{name: "nextjs", configFile: "next.config.js", want: "nextjs"},
		{name: "astro", configFile: "astro.config.mjs", want: "astro"},
		{name: "go", configFile: "go.mod", want: "go"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repoPath := t.TempDir()
			writeFile(t, repoPath, tt.configFile, "")

			builder, err := detectFramework(repoPath)
			if err != nil {
				t.Fatalf("detectFramework returned error: %v", err)
			}
			if builder.Name != tt.want {
				t.Fatalf("builder.Name = %q, want %q", builder.Name, tt.want)
			}
		})
	}
}

func TestDetectFrameworkRejectsViteSSRMetaFramework(t *testing.T) {
	tests := []struct {
		name        string
		packageJSON string
	}{
		{
			name:        "vike",
			packageJSON: `{"dependencies":{"vike":"^0.4.0"},"devDependencies":{"vite":"^5.0.0"}}`,
		},
		{
			name:        "qwik city",
			packageJSON: `{"dependencies":{"@builder.io/qwik-city":"^1.0.0"},"devDependencies":{"vite":"^5.0.0"}}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repoPath := t.TempDir()
			writeFile(t, repoPath, "vite.config.ts", "export default {}")
			writeFile(t, repoPath, "package.json", tt.packageJSON)

			builder, err := detectFramework(repoPath)
			if err == nil {
				t.Fatalf("detectFramework() = %+v, nil; want an error refusing the SSR meta-framework", builder)
			}
			if builder != nil {
				t.Fatalf("detectFramework() builder = %+v, want nil on error", builder)
			}
		})
	}
}

func TestDetectFrameworkRoutesTanStackStartToItsOwnRecipe(t *testing.T) {
	repoPath := t.TempDir()
	writeFile(t, repoPath, "vite.config.ts", "export default {}")
	writeFile(t, repoPath, "package.json", `{"dependencies":{"@tanstack/react-start":"^1.0.0","vite":"^5.0.0"}}`)

	builder, err := detectFramework(repoPath)
	if err != nil {
		t.Fatalf("detectFramework returned error for TanStack Start: %v", err)
	}
	if builder.Name != frameworkTanStack {
		t.Fatalf("builder.Name = %q, want %q", builder.Name, frameworkTanStack)
	}
	if builder.Template != "dynamic/Dockerfile.tanstack-start.tmpl" {
		t.Fatalf("builder.Template = %q, want the tanstack-start template", builder.Template)
	}
}

func TestDetectFrameworkAcceptsPlainVite(t *testing.T) {
	repoPath := t.TempDir()
	writeFile(t, repoPath, "vite.config.ts", "export default {}")
	writeFile(t, repoPath, "package.json", `{"dependencies":{"react":"^18.0.0","react-dom":"^18.0.0"},"devDependencies":{"vite":"^5.0.0"}}`)

	builder, err := detectFramework(repoPath)
	if err != nil {
		t.Fatalf("detectFramework returned error for plain Vite app: %v", err)
	}
	if builder.Name != frameworkVite {
		t.Fatalf("builder.Name = %q, want %q", builder.Name, frameworkVite)
	}
}

func TestDetectFrameworkRejectsAstroSSRAdapter(t *testing.T) {
	repoPath := t.TempDir()
	writeFile(t, repoPath, "astro.config.mjs", "export default {}")
	writeFile(t, repoPath, "package.json", `{"dependencies":{"astro":"^4.0.0","@astrojs/node":"^8.0.0"}}`)

	builder, err := detectFramework(repoPath)
	if err == nil {
		t.Fatalf("detectFramework() = %+v, nil; want an error refusing Astro SSR mode", builder)
	}
}

func TestDetectFrameworkAcceptsStaticAstro(t *testing.T) {
	repoPath := t.TempDir()
	writeFile(t, repoPath, "astro.config.mjs", "export default {}")
	writeFile(t, repoPath, "package.json", `{"dependencies":{"astro":"^4.0.0"}}`)

	builder, err := detectFramework(repoPath)
	if err != nil {
		t.Fatalf("detectFramework returned error for static Astro app: %v", err)
	}
	if builder.Name != frameworkAstro {
		t.Fatalf("builder.Name = %q, want %q", builder.Name, frameworkAstro)
	}
}

func TestHasStartScript(t *testing.T) {
	tests := []struct {
		name string
		pkg  *packageJSON
		want bool
	}{
		{name: "present", pkg: &packageJSON{Scripts: map[string]string{"start": "node server.js"}}, want: true},
		{name: "empty string", pkg: &packageJSON{Scripts: map[string]string{"start": "   "}}, want: false},
		{name: "missing", pkg: &packageJSON{Scripts: map[string]string{"build": "vite build"}}, want: false},
		{name: "nil scripts", pkg: &packageJSON{}, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := hasStartScript(tt.pkg); got != tt.want {
				t.Errorf("hasStartScript() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDetectFrameworkByPackageDependencies(t *testing.T) {
	tests := []struct {
		name        string
		packageJSON string
		want        string
	}{
		{
			name:        "react",
			packageJSON: `{"dependencies":{"react":"latest","react-dom":"latest"}}`,
			want:        "react",
		},
		{
			name:        "node fallback",
			packageJSON: `{"dependencies":{"express":"latest"}}`,
			want:        "node",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repoPath := t.TempDir()
			writeFile(t, repoPath, "package.json", tt.packageJSON)

			builder, err := detectFramework(repoPath)
			if err != nil {
				t.Fatalf("detectFramework returned error: %v", err)
			}
			if builder.Name != tt.want {
				t.Fatalf("builder.Name = %q, want %q", builder.Name, tt.want)
			}
		})
	}
}

func TestDetectFrameworkByNestedPythonFile(t *testing.T) {
	repoPath := t.TempDir()
	writeFile(t, repoPath, filepath.Join("service", "requirements.txt"), "flask\n")

	builder, err := detectFramework(repoPath)
	if err != nil {
		t.Fatalf("detectFramework returned error: %v", err)
	}
	if builder.Name != "python" {
		t.Fatalf("builder.Name = %q, want %q", builder.Name, "python")
	}
}

func TestDetectFrameworkByDockerfile(t *testing.T) {
	repoPath := t.TempDir()
	writeFile(t, repoPath, "Dockerfile", "FROM alpine\n")

	builder, err := detectFramework(repoPath)
	if err != nil {
		t.Fatalf("detectFramework returned error: %v", err)
	}
	if builder.Name != "docker" {
		t.Fatalf("builder.Name = %q, want %q", builder.Name, "docker")
	}
}

func TestDetectFrameworkReturnsErrorForUnknownProject(t *testing.T) {
	_, err := detectFramework(t.TempDir())
	if err == nil {
		t.Fatal("detectFramework returned nil error for unknown project")
	}
}

func TestDetectPackageManager(t *testing.T) {
	tests := []struct {
		name     string
		lockFile string
		want     string
	}{
		{name: "pnpm", lockFile: "pnpm-lock.yaml", want: "pnpm"},
		{name: "yarn", lockFile: "yarn.lock", want: "yarn"},
		{name: "bun", lockFile: "bun.lockb", want: "bun"},
		{name: "npm", lockFile: "package-lock.json", want: "npm"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repoPath := t.TempDir()
			writeFile(t, repoPath, tt.lockFile, "")

			if got := detectPackageManager(repoPath); got != tt.want {
				t.Fatalf("detectPackageManager = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestDetectPackageManagerDefaultsToNPM(t *testing.T) {
	if got := detectPackageManager(t.TempDir()); got != "npm" {
		t.Fatalf("detectPackageManager = %q, want %q", got, "npm")
	}
}

func TestResolveAppDir_ExplicitWorkingDir(t *testing.T) {
	repoPath := t.TempDir()
	writeFile(t, repoPath, filepath.Join("client", "package.json"), `{"dependencies":{"react":"latest"}}`)
	writeFile(t, repoPath, filepath.Join("api", "requirements.txt"), "flask\n")

	// Explicit "client"
	appDir, err := ResolveAppDir(repoPath, "client")
	if err != nil {
		t.Fatalf("ResolveAppDir(client) error: %v", err)
	}
	wantClient := filepath.Join(repoPath, "client")
	if appDir != wantClient {
		t.Fatalf("ResolveAppDir(client) = %q, want %q", appDir, wantClient)
	}

	builder, err := detectFramework(appDir)
	if err != nil {
		t.Fatalf("detectFramework(client) error: %v", err)
	}
	if builder.Name != "react" {
		t.Fatalf("builder.Name = %q, want react", builder.Name)
	}

	// Explicit "./api"
	appDir, err = ResolveAppDir(repoPath, "./api")
	if err != nil {
		t.Fatalf("ResolveAppDir(./api) error: %v", err)
	}
	wantAPI := filepath.Join(repoPath, "api")
	if appDir != wantAPI {
		t.Fatalf("ResolveAppDir(./api) = %q, want %q", appDir, wantAPI)
	}

	builder, err = detectFramework(appDir)
	if err != nil {
		t.Fatalf("detectFramework(api) error: %v", err)
	}
	if builder.Name != "python" {
		t.Fatalf("builder.Name = %q, want python", builder.Name)
	}
}

func TestResolveAppDir_PathTraversalRejected(t *testing.T) {
	repoPath := t.TempDir()
	writeFile(t, repoPath, "package.json", `{"dependencies":{"express":"latest"}}`)

	badPaths := []string{
		"../outside",
		"../../etc",
		"sub/../../outside",
		"/etc/passwd",
		"/app",
		"../",
		"..",
	}

	for _, p := range badPaths {
		t.Run(p, func(t *testing.T) {
			_, err := ResolveAppDir(repoPath, p)
			if err == nil {
				t.Fatalf("ResolveAppDir(%q) should have failed with path traversal error", p)
			}
		})
	}
}

func TestResolveAppDir_PreservesRootProject(t *testing.T) {
	repoPath := t.TempDir()
	// Node at root, Python utility script nested
	writeFile(t, repoPath, "package.json", `{"dependencies":{"express":"latest"}}`)
	writeFile(t, repoPath, filepath.Join("scripts", "requirements.txt"), "requests\n")

	for _, wd := range []string{"", ".", "  "} {
		appDir, err := ResolveAppDir(repoPath, wd)
		if err != nil {
			t.Fatalf("ResolveAppDir(%q) error: %v", wd, err)
		}
		if appDir != repoPath {
			t.Fatalf("ResolveAppDir(%q) = %q, want repoPath %q", wd, appDir, repoPath)
		}

		builder, err := detectFramework(appDir)
		if err != nil {
			t.Fatalf("detectFramework error: %v", err)
		}
		if builder.Name != "node" {
			t.Fatalf("builder.Name = %q, want 'node' (must not be misclassified as python!)", builder.Name)
		}
	}
}

func TestResolveAppDir_AmbiguityFailsClosed(t *testing.T) {
	repoPath := t.TempDir()
	// Nested Node app and nested Python manifest, no root manifest
	writeFile(t, repoPath, filepath.Join("frontend", "package.json"), `{"dependencies":{"react":"latest"}}`)
	writeFile(t, repoPath, filepath.Join("backend", "requirements.txt"), "django\n")

	// With empty or "." working directory, must fail closed
	for _, wd := range []string{"", "."} {
		_, err := ResolveAppDir(repoPath, wd)
		if err == nil {
			t.Fatalf("ResolveAppDir(%q) expected ambiguity error, got nil", wd)
		}
		if !strings.Contains(err.Error(), "ambiguous application root: multiple candidates found") {
			t.Fatalf("unexpected error message: %v", err)
		}
	}

	// detectFramework on repoPath must also fail closed with ambiguity error
	_, err := detectFramework(repoPath)
	if err == nil {
		t.Fatal("detectFramework expected ambiguity error, got nil")
	}
	if !strings.Contains(err.Error(), "ambiguous application root: multiple candidates found") {
		t.Fatalf("unexpected error message from detectFramework: %v", err)
	}
}

func TestResolveAppDir_SingleNestedCandidate(t *testing.T) {
	repoPath := t.TempDir()
	writeFile(t, repoPath, filepath.Join("apps", "web", "package.json"), `{"dependencies":{"next":"latest"}}`)

	appDir, err := ResolveAppDir(repoPath, "")
	if err != nil {
		t.Fatalf("ResolveAppDir error: %v", err)
	}
	want := filepath.Join(repoPath, "apps", "web")
	if appDir != want {
		t.Fatalf("ResolveAppDir = %q, want %q", appDir, want)
	}

	builder, err := detectFramework(appDir)
	if err != nil {
		t.Fatalf("detectFramework error: %v", err)
	}
	if builder.Name != "nextjs" {
		t.Fatalf("builder.Name = %q, want nextjs", builder.Name)
	}
}

func TestResolveAppDir_NonexistentOrFile(t *testing.T) {
	repoPath := t.TempDir()
	writeFile(t, repoPath, "hello.txt", "hello")

	// Non-existent directory
	_, err := ResolveAppDir(repoPath, "missing-dir")
	if err == nil {
		t.Fatal("expected error for non-existent working dir")
	}
	if !strings.Contains(err.Error(), "does not exist") {
		t.Fatalf("unexpected error: %v", err)
	}

	// Regular file instead of directory
	_, err = ResolveAppDir(repoPath, "hello.txt")
	if err == nil {
		t.Fatal("expected error for file passed as working dir")
	}
	if !strings.Contains(err.Error(), "is not a directory") {
		t.Fatalf("unexpected error: %v", err)
	}
}
