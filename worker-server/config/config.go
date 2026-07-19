// Package config provides configuration loading for the worker server.
package config

import (
	"strings"

	"github.com/spf13/viper"
)

// LoadConfig loads environment configuration from .env file and system environment.
func LoadConfig() {
	// Set up for Environment Variables
	viper.SetConfigFile(".env")
	viper.SetConfigType("env")

	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	// Check if the env file can be read or not
	if err := viper.ReadInConfig(); err != nil {
		panic("Error reading config file: " + err.Error())
	}

	viper.AutomaticEnv()

	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
}
