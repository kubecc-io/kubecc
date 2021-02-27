package host

import "runtime"

type SystemInfo struct {
	Arch         string
	CpuThreads   int32
	SystemMemory uint64
}

func GetSystemInfo() SystemInfo {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	return SystemInfo{
		Arch:         runtime.GOARCH,
		CpuThreads:   int32(runtime.NumCPU()),
		SystemMemory: memStats.Sys,
	}
}
