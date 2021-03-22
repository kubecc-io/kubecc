package host

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
)

const (
	cgroupDir = "/sys/fs/cgroup"
)

func readInt64(path string) (int64, error) {
	data, err := os.ReadFile(filepath.Join(cgroupDir, path))
	if err != nil {
		return 0, err
	}
	value, err := strconv.ParseInt(
		strings.TrimSpace(string(data)), 10, 64)
	if err != nil {
		panic(err)
	}
	return value, nil
}

func CfsQuota() int64 {
	value, err := readInt64("cpu/cpu.cfs_quota_us")
	if err != nil {
		fmt.Printf("Warning: could not read CFS quota from %s. Your kernel may not be compiled with CFS Bandwidth support.\n", cgroupDir)
		// Assuming CfsPeriod() will fail and return 1
		return int64(runtime.NumCPU())
	}
	return value
}

func CfsPeriod() int64 {
	value, err := readInt64("cpu/cpu.cfs_period_us")
	if err != nil {
		fmt.Printf("Warning: could not read CFS period from %s. Your kernel may not be compiled with CFS Bandwidth support.\n", cgroupDir)
		return 1
	}
	return value
}
