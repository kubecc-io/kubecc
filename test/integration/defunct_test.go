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
	"time"

	"github.com/kubecc-io/kubecc/pkg/apps/agent"
	"github.com/kubecc-io/kubecc/pkg/apps/consumerd"
	"github.com/kubecc-io/kubecc/pkg/metrics"
	"github.com/kubecc-io/kubecc/pkg/test"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"go.uber.org/zap/zapcore"
)

var _ = Describe("Defunct Tasks", func() {
	var testEnv *test.Environment
	localJobs := 10
	numTasks := 20

	var cdCtx context.Context
	var agentCancel context.CancelFunc
	Specify("setup", func() {
		test.SkipInGithubWorkflow()
		testEnv = test.NewEnvironmentWithLogLevel(zapcore.ErrorLevel)

		testEnv.SpawnMonitor(test.WaitForReady())
		testEnv.SpawnScheduler(test.WaitForReady())
		cdCtx, _ = testEnv.SpawnConsumerd(test.WaitForReady(), test.WithConsumerdOptions(
			consumerd.WithQueueOptions(
				consumerd.WithLocalUsageManager(consumerd.FixedUsageLimits(5)),
				consumerd.WithRemoteUsageManager(consumerd.FixedUsageLimits(5)),
			),
		))
		_, agentCancel = testEnv.SpawnAgent(test.WaitForReady(), test.WithAgentOptions(
			agent.WithUsageLimits(&metrics.UsageLimits{
				ConcurrentProcessLimit: 5,
			}),
		))
	})

	When("An agent becomes unavailable while processing tasks", func() {
		It("should handle retrying tasks or running locally", func() {
			test.SkipInGithubWorkflow()
			go func() {
				time.Sleep(500 * time.Millisecond)
				agentCancel()
			}()
			test.ProcessTaskPool(testEnv, localJobs, test.MakeSleepTaskPool(numTasks,
				func() string {
					return "1s"
				}), 5*time.Second)

			// Both should be equal to numTasks
			Eventually(testEnv.MetricF(cdCtx, &metrics.LocalTasksCompleted{}),
				8*time.Second, 100*time.Millisecond,
			).Should(
				WithTransform(func(m *metrics.LocalTasksCompleted) int64 {
					return m.Total
				}, BeEquivalentTo(numTasks)),
			)
			Eventually(testEnv.MetricF(cdCtx, &metrics.DelegatedTasksCompleted{}),
				8*time.Second, 100*time.Millisecond,
			).Should(
				WithTransform(func(m *metrics.DelegatedTasksCompleted) int64 {
					return m.Total
				}, BeEquivalentTo(numTasks)),
			)
		})
	})
})
