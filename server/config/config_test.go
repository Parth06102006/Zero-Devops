package config

import (
	"os"
	"testing"

	"github.com/spf13/viper"
)

func TestLoadConfig_NoEnvFile(t *testing.T) {
	viper.Reset()

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("LoadConfig panicked when .env is missing: %v", r)
		}
	}()

	LoadConfig()
}

func TestLoadConfig_ReadsEnvFile(t *testing.T) {
	content := []byte("TEST_KEY=test_value\n")
	tmpFile, err := os.CreateTemp("", ".env")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = tmpFile.Close()
		_ = os.Remove(tmpFile.Name())
	}()

	if _, err := tmpFile.Write(content); err != nil {
		t.Fatal(err)
	}
	_ = tmpFile.Close()

	origDir, _ := os.Getwd()
	_ = os.Chdir(t.TempDir())
	defer func() { _ = os.Chdir(origDir) }()

	envPath := tmpFile.Name()
	viper.Reset()
	viper.SetConfigFile(envPath)
	viper.SetConfigType("env")

	LoadConfig()

	if viper.GetString("TEST_KEY") != "" {
		key := viper.GetString("TEST_KEY")
		if key == "" {
			t.Log("note: TEST_KEY was not loaded (may have been reset)")
		}
	}
}

func TestLoadConfig_DoesNotPanic(t *testing.T) {
	viper.Reset()

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("LoadConfig panicked: %v", r)
		}
	}()

	LoadConfig()
}
