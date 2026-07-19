package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/viper"
)

func TestLoadConfigReadsDotEnvFile(t *testing.T) {
	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := os.Chdir(originalDir); err != nil {
			t.Fatal(err)
		}
	}()

	viper.Reset()
	defer viper.Reset()

	tempDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(tempDir, ".env"), []byte("DATABASE_HOST=localhost\nCLOUDFLARE_BUCKET_NAME=test-bucket\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(tempDir); err != nil {
		t.Fatal(err)
	}

	LoadConfig()

	if got := viper.GetString("DATABASE_HOST"); got != "localhost" {
		t.Fatalf("DATABASE_HOST = %q, want %q", got, "localhost")
	}
	if got := viper.GetString("CLOUDFLARE_BUCKET_NAME"); got != "test-bucket" {
		t.Fatalf("CLOUDFLARE_BUCKET_NAME = %q, want %q", got, "test-bucket")
	}
}

func TestLoadConfigPanicsWhenDotEnvMissing(t *testing.T) {
	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := os.Chdir(originalDir); err != nil {
			t.Fatal(err)
		}
	}()

	viper.Reset()
	defer viper.Reset()

	if err := os.Chdir(t.TempDir()); err != nil {
		t.Fatal(err)
	}

	defer func() {
		if recover() == nil {
			t.Fatal("LoadConfig did not panic for missing .env")
		}
	}()

	LoadConfig()
}
