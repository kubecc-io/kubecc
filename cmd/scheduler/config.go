package main

import (
	"github.com/cobalt77/kubecc/internal/lll"
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

func initConfig() {
	viper.AddConfigPath("/config")
	viper.SetConfigName("scheduler")
	viper.AutomaticEnv()
	viper.SetDefault("scheduler", "roundRobinDns")
	if err := viper.ReadInConfig(); err != nil {
		lll.With(zap.Error(err)).Warn("Error reading config file")
	}
}
