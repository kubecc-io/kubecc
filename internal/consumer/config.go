package consumer

import (
	"github.com/mitchellh/go-homedir"
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

// InitConfig initializes the Viper config
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
	viper.SetDefault("loglevel", zap.DebugLevel)
	viper.SetDefault("schedulerAddress", "")

	// If a config file is found, read it in.
	if err := viper.ReadInConfig(); err != nil {
		log.With(zap.Error(err)).Debug("Error reading config file")
	}
}
