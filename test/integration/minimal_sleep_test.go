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

	"github.com/kubecc-io/kubecc/pkg/test"
	. "github.com/onsi/ginkgo"
	"go.uber.org/zap/zapcore"
	// . "github.com/onsi/gomega"
)

var _ = Describe("Sleep test", func() {
	var testEnv test.Environment
	localJobs := 20

	Specify("setup", func() {
		testEnv = test.NewLocalhostEnvironmentWithLogLevel(zapcore.WarnLevel)

		test.SpawnMonitor(testEnv, test.WaitForReady())
		test.SpawnScheduler(testEnv, test.WaitForReady())
	})

	Specify("Starting consumerd", func() {
		test.SpawnConsumerd(testEnv, test.WithName("c0"), test.WaitForReady())
	})
	Specify("1 agent, no cache", func() {
		test.SpawnAgent(testEnv, test.WithName("a0"), test.WaitForReady())
		test.ProcessTaskPool(testEnv, "c0", localJobs, test.MakeSleepTaskPool(100), 5*time.Second)
	})

	Specify("2 agents, no cache", func() {
		test.SpawnAgent(testEnv, test.WithName("a1"), test.WaitForReady())
		test.ProcessTaskPool(testEnv, "c0", localJobs, test.MakeSleepTaskPool(200), 5*time.Second)
	})

	Specify("4 agents, no cache", func() {
		test.SpawnAgent(testEnv, test.WithName("a2"), test.WaitForReady())
		test.SpawnAgent(testEnv, test.WithName("a3"), test.WaitForReady())
		test.ProcessTaskPool(testEnv, "c0", localJobs, test.MakeSleepTaskPool(400), 5*time.Second)
	})

	Specify("shutdown", func() {
		testEnv.Shutdown()
	})
})
