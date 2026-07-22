package deployments

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestPackBuild_InvalidPath(t *testing.T) {
	ctx := context.Background()
	err := packBuild(ctx, "/nonexistent/path", "test-image:latest")
	if err == nil {
		t.Fatal("expected error for invalid path")
	}
}

func TestPackBuild_WithGoApp(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping pack build integration test in short mode")
	}
	if _, err := exec.LookPath("pack"); err != nil {
		t.Skip("pack CLI not installed, skipping")
	}
	if _, err := exec.LookPath("docker"); err != nil {
		t.Skip("docker not available, skipping")
	}

	tmpDir := t.TempDir()

	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte("module test\n\ngo 1.25\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte("package main\nimport \"fmt\"\nfunc main() { fmt.Println(\"hello\") }\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	imageTag := "pack-test-go-app:latest"
	err := packBuild(context.Background(), tmpDir, imageTag)
	if err != nil {
		t.Fatalf("pack build failed on Go project: %v", err)
	}

	t.Cleanup(func() {
		_ = exec.CommandContext(context.Background(), "docker", "rmi", "-f", imageTag).Run()
	})
}

func TestValidateCloneURL_ValidHTTPSGithub(t *testing.T) {
	urls := []string{
		"https://github.com/user/repo.git",
		"https://github.com/org/project",
		"https://github.com/org/project.git",
	}
	for _, u := range urls {
		if err := validateCloneURL(u); err != nil {
			t.Errorf("validateCloneURL(%q) = %v, want nil", u, err)
		}
	}
}

func TestValidateCloneURL_RejectsNonHTTPS(t *testing.T) {
	urls := []string{
		"http://github.com/user/repo.git",
		"git@github.com:user/repo.git",
		"ftp://github.com/user/repo",
		"file:///path/to/repo",
	}
	for _, u := range urls {
		if err := validateCloneURL(u); err == nil {
			t.Errorf("validateCloneURL(%q) = nil, want error for non-https scheme", u)
		}
	}
}

func TestValidateCloneURL_RejectsDashPrefix(t *testing.T) {
	urls := []string{
		"--depth=1",
		"-oUserKnownHostsFile=/dev/null",
		"-",
	}
	for _, u := range urls {
		if err := validateCloneURL(u); err == nil {
			t.Errorf("validateCloneURL(%q) = nil, want error for dash prefix", u)
		}
	}
}

func TestValidateCloneURL_RejectsNonGithubHost(t *testing.T) {
	urls := []string{
		"https://gitlab.com/user/repo.git",
		"https://bitbucket.org/user/repo",
		"https://example.com/repo",
	}
	for _, u := range urls {
		if err := validateCloneURL(u); err == nil {
			t.Errorf("validateCloneURL(%q) = nil, want error for non-github host", u)
		}
	}
}

func TestValidateCloneURL_RejectsMalformedURL(t *testing.T) {
	urls := []string{
		":invalid",
		"%",
		"https://",
	}
	for _, u := range urls {
		if err := validateCloneURL(u); err == nil {
			t.Errorf("validateCloneURL(%q) = nil, want error for malformed URL", u)
		}
	}
}
