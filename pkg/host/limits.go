package cpuconfig

import (
	"math"
	"runtime"

	"github.com/cobalt77/kubecc/pkg/config"
	"github.com/cobalt77/kubecc/pkg/host"
	"github.com/cobalt77/kubecc/pkg/types"
	"github.com/spf13/viper"
)

func DefaultUsageLimits() *types.UsageLimits {
	var maxRunning int32
	if value := viper.GetInt(config.ConcurrentProcessLimit); value >= 0 {
		maxRunning = int32(value)
	} else {
		period := host.CfsPeriod()
		if quota := host.CfsQuota(); quota < 0 {
			// No limit, default to the number of cpu threads
			maxRunning = int32(runtime.NumCPU())
		} else {
			// We need a whole number, and it needs to be at least 1.
			maxRunning = int32(math.Max(1, math.Ceil(float64(quota)/float64(period))))
		}
	}

	return &types.UsageLimits{
		ConcurrentProcessLimit:  maxRunning,
		QueuePressureMultiplier: viper.GetFloat64(config.QueuePressureMultiplier),
		QueueRejectMultiplier:   viper.GetFloat64(config.QueueRejectMultiplier),
	}
}
