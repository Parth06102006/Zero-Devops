package config

import (
	"os"
	"strings"

	"github.com/spf13/viper"
)

func LoadConfig(){

	// Set up for Enviroment Variables
	viper.SetConfigFile(".env")
	viper.SetConfigType("env")

	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	// Check if the env file can be read or not
	if _, err := os.Stat(".env"); err == nil {
		if err := viper.ReadInConfig(); err != nil {
			panic("Error reading config file: " + err.Error())
		}
	} else {
		panic("Error finding .env file (check working directory): " + err.Error())
	}

	viper.AutomaticEnv()

	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
}

