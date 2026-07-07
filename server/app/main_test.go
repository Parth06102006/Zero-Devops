package main

import (
	"Zero_Devops/server/config"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/viper"
)

func TestLoadConfig(t *testing.T) {
	originalEnv := os.Environ()
	defer func() {
		os.Clearenv()
		for _, kv := range originalEnv {
			key, value, ok := strings.Cut(kv, "=")
			if ok {
				_ = os.Setenv(key, value)
			}
		}
	}()

	originalWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	defer func() {
		_ = os.Chdir(originalWD)
	}()

	tmpDir := t.TempDir()
	envPath := filepath.Join(tmpDir, ".env")
	if err := os.WriteFile(envPath, []byte("SERVER_ADDRESS=:8080\nDATABASE_HOST=localhost\n"), 0o600); err != nil {
		t.Fatalf("write env: %v", err)
	}

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	viper.Reset()
	config.LoadConfig()

	if got := viper.GetString("SERVER_ADDRESS"); got != ":8080" {
		t.Fatalf("expected SERVER_ADDRESS to be loaded, got %q", got)
	}
	if got := viper.GetString("DATABASE_HOST"); got != "localhost" {
		t.Fatalf("expected DATABASE_HOST to be loaded, got %q", got)
	}
}
