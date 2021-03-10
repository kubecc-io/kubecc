package host

import (
	"math"
	"runtime"
)

func AutoConcurrentProcessLimit() (maxRunning int32) {
	period := CfsPeriod()
	if quota := CfsQuota(); quota < 0 {
		// No limit, default to the number of cpu threads
		maxRunning = int32(runtime.NumCPU())
	} else {
		// We need a whole number, and it needs to be at least 1.
		maxRunning = int32(math.Max(1, math.Ceil(float64(quota)/float64(period))))
	}
	return
}
