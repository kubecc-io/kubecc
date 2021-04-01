package internal

import (
	"github.com/kubecc-io/kubecc/pkg/config"
)

type cliConfigProvider struct{}

var ConfigPath string
var CLIConfigProvider cliConfigProvider

func (cliConfigProvider) Load() *config.KubeccSpec {
	if ConfigPath == "" {
		return config.ConfigMapProvider.Load()
	}
	return config.LoadFile(ConfigPath)
}
