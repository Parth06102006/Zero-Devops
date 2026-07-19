package deployments

import (
	"os"
	"path/filepath"
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
