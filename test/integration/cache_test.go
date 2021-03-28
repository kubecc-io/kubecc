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

	"github.com/cobalt77/kubecc/pkg/apps/agent"
	"github.com/cobalt77/kubecc/pkg/apps/consumerd"
	"github.com/cobalt77/kubecc/pkg/clients"
	"github.com/cobalt77/kubecc/pkg/metrics"
	"github.com/cobalt77/kubecc/pkg/test"
	. "github.com/onsi/ginkgo"
	// . "github.com/onsi/gomega"
)

var _ = FDescribe("Cache test", func() {
	var testEnv *test.Environment
	localJobs := 20

	Specify("setup", func() {
		cfg := test.DefaultConfig()
		cfg.Global.LogLevel = "info"
		testEnv = test.NewEnvironment(cfg)

		testEnv.SpawnMonitor()
		testEnv.SpawnScheduler()
		testEnv.SpawnCache()
		testEnv.SpawnAgent(test.WithAgentOptions(
			agent.WithUsageLimits(&metrics.UsageLimits{
				ConcurrentProcessLimit: 50,
			}),
		))
		testEnv.SpawnConsumerd(test.WithConsumerdOptions(
			consumerd.WithQueueOptions(
				consumerd.WithLocalUsageManager(
					consumerd.FixedUsageLimits(0),
				),
				consumerd.WithRemoteUsageManager(
					clients.NewRemoteUsageManager(testCtx,
						testEnv.NewMonitorClient(testCtx))),
			),
		))
		time.Sleep(50 * time.Millisecond)
	})

	Measure("1 agent, cache online", func(b Benchmarker) {
		start := time.Now()
		processTaskPool(testEnv, localJobs, makeSleepTaskPool(50, func() string {
			return "100ms"
		}), 5*time.Second)
		duration1 := time.Since(start)
		b.RecordValueWithPrecision("No cached results", float64(duration1.Milliseconds()), "ms", 2)
		start2 := time.Now()
		processTaskPool(testEnv, localJobs, makeSleepTaskPool(50, func() string {
			return "100ms"
		}), 5*time.Second)
		duration2 := time.Since(start2)
		b.RecordValueWithPrecision("With cached results", float64(duration2.Milliseconds()), "ms", 2)
		// Expect(duration2.Milliseconds()).To(BeNumerically("<", 150))
	}, 1)

})
