package main

import (
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

func initConfig() {
	viper.AddConfigPath("/config")
	viper.SetConfigName("scheduler")
	viper.AutomaticEnv()
	viper.SetDefault("scheduler", "roundRobinDns")
	if err := viper.ReadInConfig(); err != nil {
		log.With(zap.Error(err)).Warn("Error reading config file")
	}
}
