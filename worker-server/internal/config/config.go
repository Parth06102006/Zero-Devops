// Package config provides configuration loading for the worker server.
package config

import (
	"os"
	"strings"

	"github.com/spf13/viper"
)

// LoadConfig loads configuration from an optional .env file and system environment
// variables. Docker containers receive configuration through their environment.
func LoadConfig() {
	// Set up for Environment Variables
	viper.SetConfigFile(".env")
	viper.SetConfigType("env")

	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	if err := viper.ReadInConfig(); err != nil && !os.IsNotExist(err) {
		panic("Error reading config file: " + err.Error())
	}

	viper.AutomaticEnv()

	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
}
