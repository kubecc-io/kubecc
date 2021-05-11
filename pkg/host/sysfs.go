/*
Copyright 2021 The Kubecc Authors.

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU General Public License for more details.

You should have received a copy of the GNU General Public License
along with this program.  If not, see <http://www.gnu.org/licenses/>.
*/

package host

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/opencontainers/runc/libcontainer/cgroups"
	"github.com/opencontainers/runc/libcontainer/cgroups/fs"
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
		fmt.Fprintf(os.Stderr, "Warning: could not read CFS quota from %s. Your kernel may not be compiled with CFS Bandwidth support.\n", cgroupDir)
		// Assuming CfsPeriod() will fail and return 1
		return int64(runtime.NumCPU())
	}
	return value
}

func CfsPeriod() int64 {
	value, err := readInt64("cpu/cpu.cfs_period_us")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not read CFS period from %s. Your kernel may not be compiled with CFS Bandwidth support.\n", cgroupDir)
		return 1
	}
	return value
}

func CpuStats() (*cgroups.Stats, error) {
	cpuacct := &fs.CpuacctGroup{}
	cpu := &fs.CpuGroup{}
	stats := cgroups.NewStats()
	err := cpuacct.GetStats(filepath.Join(cgroupDir, "cpu"), stats)
	if err != nil {
		return nil, err
	}
	err = cpu.GetStats(filepath.Join(cgroupDir, "cpu"), stats)
	if err != nil {
		return nil, err
	}
	return stats, nil
}
