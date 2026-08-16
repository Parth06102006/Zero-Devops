package main

import (
	"Zero_Devops/server/internal/config"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/viper"
)

func withCleanEnv(t *testing.T, fn func()) {
	t.Helper()
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
	fn()
}

func withTempDir(t *testing.T, fn func(dir string)) {
	t.Helper()
	originalWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	defer func() {
		_ = os.Chdir(originalWD)
	}()
	tmpDir := t.TempDir()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	fn(tmpDir)
}

func TestLoadConfig_FromEnvFile(t *testing.T) {
	withCleanEnv(t, func() {
		withTempDir(t, func(dir string) {
			envPath := filepath.Join(dir, ".env")
			if err := os.WriteFile(envPath, []byte("SERVER_ADDRESS=:8080\nDATABASE_HOST=localhost\n"), 0o600); err != nil {
				t.Fatalf("write env: %v", err)
			}

			viper.Reset()
			config.LoadConfig()

			if got := viper.GetString("SERVER_ADDRESS"); got != ":8080" {
				t.Fatalf("expected SERVER_ADDRESS to be loaded, got %q", got)
			}
			if got := viper.GetString("DATABASE_HOST"); got != "localhost" {
				t.Fatalf("expected DATABASE_HOST to be loaded, got %q", got)
			}
		})
	})
}

func TestLoadConfig_FromEnvVars(t *testing.T) {
	withCleanEnv(t, func() {
		withTempDir(t, func(_ string) {
			_ = os.Setenv("SERVER_ADDRESS", ":9090")
			_ = os.Setenv("DATABASE_HOST", "db.example.com")

			viper.Reset()
			config.LoadConfig()

			if got := viper.GetString("SERVER_ADDRESS"); got != ":9090" {
				t.Fatalf("expected SERVER_ADDRESS :9090, got %q", got)
			}
			if got := viper.GetString("DATABASE_HOST"); got != "db.example.com" {
				t.Fatalf("expected DATABASE_HOST db.example.com, got %q", got)
			}
		})
	})
}

func TestLoadConfig_EnvFileOverriddenByEnvVars(t *testing.T) {
	withCleanEnv(t, func() {
		withTempDir(t, func(dir string) {
			envPath := filepath.Join(dir, ".env")
			if err := os.WriteFile(envPath, []byte("SERVER_ADDRESS=:8080\n"), 0o600); err != nil {
				t.Fatalf("write env: %v", err)
			}
			_ = os.Setenv("SERVER_ADDRESS", ":3000")

			viper.Reset()
			config.LoadConfig()

			if got := viper.GetString("SERVER_ADDRESS"); got != ":3000" {
				t.Fatalf("expected SERVER_ADDRESS :3000 (env var override), got %q", got)
			}
		})
	})
}

func TestLoadConfig_MissingEnvFile(t *testing.T) {
	withCleanEnv(t, func() {
		withTempDir(t, func(_ string) {
			viper.Reset()

			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("LoadConfig panicked when .env is missing: %v", r)
				}
			}()

			config.LoadConfig()
		})
	})
}
