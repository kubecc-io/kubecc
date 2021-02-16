package sysfs

import (
	"os"
	"path/filepath"
	"strconv"
)

const (
	cgroupDir = "/sys/fs/cgroup"
)

func readInt64(path string) int64 {
	data, err := os.ReadFile(filepath.Join(cgroupDir, path))
	if err != nil {
		panic("Could not read CFS quota from sysfs")
	}
	value, err := strconv.ParseInt(string(data), 10, 64)
	if err != nil {
		panic(err)
	}
	return value
}

func CfsQuota() int64 {
	return readInt64("cpu/cpu.cfs_quota_us")
}

func CfsPeriod() int64 {
	return readInt64("cpu/cpu.cfs_period_us")
}
