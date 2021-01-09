package main

import (
	"github.com/mitchellh/go-homedir"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

const DefaultLogOption string = "INFO"

var LogOptions = map[string]log.Level{
	"ERROR": log.ErrorLevel,
	"WARN":  log.WarnLevel,
	"INFO":  log.InfoLevel,
	"DEBUG": log.DebugLevel,
	"TRACE": log.TraceLevel,
}

// InitConfig initializes the Viper config
func InitConfig() {
	viper.AddConfigPath("/etc")
	// Find home directory.
	home, err := homedir.Dir()
	if err == nil {
		viper.AddConfigPath(home)
	}
	viper.SetConfigName(".kdc-agent")

	viper.AutomaticEnv() // read in environment variables that match

	viper.SetDefault("agentPort", 23632)
	viper.SetDefault("loglevel", DefaultLogOption)
	viper.SetDefault("remoteAddress", "localhost")
	viper.SafeWriteConfig()

	// If a config file is found, read it in.
	if err := viper.ReadInConfig(); err != nil {
		log.WithError(err).Debug("Error reading config file")
	}
}
