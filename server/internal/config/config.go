// Package config provides application configuration loading using viper
package config

import (
	"os"
	"strings"

	"github.com/spf13/viper"
)

// LoadConfig configures application settings from an optional .env file and environment variables.
// It panics if an existing .env file cannot be read.
func LoadConfig() {
	viper.SetConfigFile(".env")
	viper.SetConfigType("env")

	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	if _, err := os.Stat(".env"); err == nil {
		if err := viper.ReadInConfig(); err != nil {
			panic("Error reading config file: " + err.Error())
		}
	}

	viper.AutomaticEnv()

	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
}
