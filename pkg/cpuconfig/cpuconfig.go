package cpuconfig

import (
	"math"
	"runtime"

	"github.com/cobalt77/kubecc/pkg/sysfs"
	"github.com/cobalt77/kubecc/pkg/types"
)

func Default() *types.CpuConfig {
	period := sysfs.CfsPeriod()
	var maxRunning int32
	if quota := sysfs.CfsQuota(); quota < 0 {
		// No limit, default to the number of cpu threads
		maxRunning = int32(runtime.NumCPU())
	} else {
		// We need a whole number, and it needs to be at least 1.
		maxRunning = int32(math.Max(1, math.Ceil(float64(quota)/float64(period))))
	}

	return &types.CpuConfig{
		MaxRunningProcesses:    maxRunning * 3 / 2,
		QueuePressureThreshold: 1.0, // 100% of max
		QueueRejectThreshold:   2.0, // 200% of max
	}
}
