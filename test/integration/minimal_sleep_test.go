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

package integration

import (
	"time"

	"github.com/cobalt77/kubecc/pkg/test"
	. "github.com/onsi/ginkgo"
	// . "github.com/onsi/gomega"
)

var _ = Describe("Sleep test", func() {
	var testEnv *test.Environment
	localJobs := 20

	Specify("setup", func() {
		cfg := test.DefaultConfig()
		cfg.Global.LogLevel = "warn"
		testEnv = test.NewEnvironment(cfg)

		testEnv.SpawnMonitor(test.WaitForReady())
		testEnv.SpawnScheduler(test.WaitForReady())
		testEnv.SpawnConsumerd(test.WaitForReady())
	})

	Specify("Starting consumerd", func() {
		testEnv.SpawnConsumerd(test.WaitForReady())
	})
	Specify("1 agent, no cache", func() {
		testEnv.SpawnAgent(test.WaitForReady())
		processTaskPool(testEnv, localJobs, makeSleepTaskPool(100), 5*time.Second)
	})

	Specify("2 agents, no cache", func() {
		testEnv.SpawnAgent(test.WaitForReady())
		processTaskPool(testEnv, localJobs, makeSleepTaskPool(200), 5*time.Second)
	})

	Specify("4 agents, no cache", func() {
		testEnv.SpawnAgent(test.WaitForReady())
		testEnv.SpawnAgent(test.WaitForReady())
		processTaskPool(testEnv, localJobs, makeSleepTaskPool(400), 5*time.Second)
	})

	Specify("shutdown", func() {
		testEnv.Shutdown()
	})
})
