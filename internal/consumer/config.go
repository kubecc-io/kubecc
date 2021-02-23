package consumer

import (
	"fmt"

	"github.com/mitchellh/go-homedir"
	"github.com/spf13/viper"
)

// todo: delete this

// InitConfig initializes the Viper config.
func InitConfig() {
	viper.AddConfigPath("/etc")
	// Find home directory.
	home, err := homedir.Dir()
	if err == nil {
		viper.AddConfigPath(home)
	}
	viper.SetConfigName(".kccagent")

	viper.AutomaticEnv() // read in environment variables that match

	viper.SetDefault("port", 23632)
	viper.SetDefault("tls", true)
	viper.SetDefault("remoteOnly", false)

	// If a config file is found, read it in.
	if err := viper.ReadInConfig(); err != nil {
		fmt.Println("Error reading config file")
	}
}
