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
	"context"
	"runtime"
	"time"

	"github.com/kubecc-io/kubecc/pkg/apps/agent"
	"github.com/kubecc-io/kubecc/pkg/apps/consumerd"
	"github.com/kubecc-io/kubecc/pkg/clients"
	"github.com/kubecc-io/kubecc/pkg/metrics"
	"github.com/kubecc-io/kubecc/pkg/test"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"go.uber.org/zap/zapcore"
	"google.golang.org/protobuf/proto"
)

var _ = Describe("Cache test", func() {
	var testEnv test.Environment
	localJobs := runtime.NumCPU()
	var cdCtx context.Context
	Specify("setup", func() {
		testEnv = test.NewLocalhostEnvironmentWithLogLevel(zapcore.DebugLevel)

		test.SpawnMonitor(testEnv, test.WaitForReady())
		test.SpawnScheduler(testEnv, test.WaitForReady())
		test.SpawnCache(testEnv, test.WaitForReady())
		test.SpawnAgent(testEnv, test.WithAgentOptions(
			agent.WithUsageLimits(&metrics.UsageLimits{
				ConcurrentProcessLimit: int32(localJobs),
			}),
		), test.WaitForReady())
		cdCtx, _ = test.SpawnConsumerd(testEnv, test.WithConsumerdOptions(
			consumerd.WithQueueOptions(
				consumerd.WithLocalUsageManager(
					consumerd.FixedUsageLimits(0), // disable local
				),
				consumerd.WithRemoteUsageManager(
					clients.NewRemoteUsageManager(testCtx,
						test.NewMonitorClient(testEnv, testCtx))),
			),
		), test.WaitForReady())
	})

	Specify("consumerd queue should enable remote tasks", func() {
		Eventually(testEnv.MetricF(cdCtx, &metrics.UsageLimits{}), 10*time.Second, 100*time.Millisecond).Should(
			WithTransform(func(m proto.Message) int32 {
				return m.(*metrics.UsageLimits).DelegatedTaskLimit
			}, BeEquivalentTo(localJobs)),
		)
	})

	Measure("1 agent, cache online", func(b Benchmarker) {
		start := time.Now()
		test.ProcessTaskPool(testEnv, "default", localJobs, test.MakeSleepTaskPool(localJobs, func() string {
			return "100ms"
		}), 5*time.Second)
		duration1 := time.Since(start)
		b.RecordValueWithPrecision("No cached results", float64(duration1.Milliseconds()), "ms", 2)
		start2 := time.Now()
		test.ProcessTaskPool(testEnv, "default", localJobs, test.MakeSleepTaskPool(localJobs, func() string {
			return "100ms"
		}), 5*time.Second)
		duration2 := time.Since(start2)
		b.RecordValueWithPrecision("With cached results", float64(duration2.Milliseconds()), "ms", 2)
		Expect(duration2.Milliseconds()).To(BeNumerically("<", duration1.Milliseconds()))
	}, 1)

})
