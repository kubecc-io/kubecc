package config

import (
	"path"
	"strings"

	"github.com/cobalt77/kubecc/pkg/meta"
	"github.com/cobalt77/kubecc/pkg/types"
	"github.com/spf13/viper"
	"go.uber.org/zap"
	"k8s.io/client-go/util/homedir"
)

const (
	QueuePressureMultiplier = "queuePressureMultiplier"
	QueueRejectMultiplier   = "queueRejectMultiplier"
	ConcurrentProcessLimit  = "concurrentProcessLimit"
	LogLevel                = "loglevel"
	SchedulerAddress        = "schedulerAddress"
	MonitorAddress          = "monitorAddress"
)

type ConfigProvider interface {
	Setup(types.Component)
}

type ConfigMapProvider struct{}

func (cmp *ConfigMapProvider) Setup(ctx meta.Context, c types.Component) {
	lg := ctx.Log()
	switch c {
	case types.Agent, types.Scheduler, types.Dashboard, types.Monitor:
		viper.AddConfigPath("/etc/kubecc")
		viper.SetConfigName(strings.ToLower(c.Name()))
	case types.Controller:
	case types.Consumer, types.Consumerd, types.Make, types.CLI:
		viper.AddConfigPath("/etc/kubecc")
		viper.AddConfigPath(path.Join(homedir.HomeDir(), ".kubecc"))
		viper.SetConfigName("config")
	case types.TestComponent:
	}

	if c == types.Agent || c == types.Consumerd {
		viper.SetDefault(QueuePressureMultiplier, 1.5)
		viper.SetDefault(QueueRejectMultiplier, 2.0)
		viper.SetDefault(ConcurrentProcessLimit, -1) // -1 = automatic
	}

	if err := viper.ReadInConfig(); err == nil {
		lg.With(zap.Error(err)).Debug("Could not read config file")
	}
}
