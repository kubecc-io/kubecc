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

package scheduler_test

import (
	"context"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"go.uber.org/zap/zapcore"

	"github.com/kubecc-io/kubecc/pkg/agent"
	"github.com/kubecc-io/kubecc/pkg/consumerd"
	"github.com/kubecc-io/kubecc/pkg/meta"
	"github.com/kubecc-io/kubecc/pkg/metrics"
	"github.com/kubecc-io/kubecc/pkg/test"
)

var _ = Describe("Scheduler Server", func() {
	var schedCtx context.Context
	var testEnv test.Environment
	var agentID string
	var consumerdID string
	Specify("setup", func() {
		testEnv = test.NewBufconnEnvironmentWithLogLevel(zapcore.ErrorLevel)
		test.SpawnMonitor(testEnv)
		schedCtx, _ = test.SpawnScheduler(testEnv, test.WaitForReady())
		ctx, _ := test.SpawnAgent(testEnv, test.WaitForReady(), test.WithAgentOptions(
			agent.WithUsageLimits(&metrics.UsageLimits{
				ConcurrentProcessLimit: 20,
			}),
		))
		agentID = meta.UUID(ctx)
		ctx, _ = test.SpawnConsumerd(testEnv, test.WaitForReady(), test.WithConsumerdOptions(
			consumerd.WithQueueOptions(
				consumerd.WithLocalUsageManager(consumerd.FixedUsageLimits(20)),
				consumerd.WithRemoteUsageManager(consumerd.FixedUsageLimits(20)),
			),
		))
		consumerdID = meta.UUID(ctx)
	})

	It("should eventually become ready", func() {
		test.EventuallyHealthStatusShouldBeReady(schedCtx, testEnv)
	})

	It("should post metrics", func() {
		numTasks := 100
		go test.ProcessTaskPool(testEnv, "default", 100, test.MakeHashTaskPool(numTasks), 1*time.Second)
		Eventually(testEnv.MetricF(schedCtx, &metrics.PreferredUsageLimits{})).
			Should(Not(BeNil()))

		Eventually(testEnv.MetricF(schedCtx, &metrics.AgentCount{}),
			9*time.Second, 50*time.Millisecond, // posted every 5-7.5s
		).Should(
			WithTransform(func(c *metrics.AgentCount) int64 {
				return c.Count
			}, BeEquivalentTo(1)),
		)
		Eventually(testEnv.MetricF(schedCtx, &metrics.ConsumerdCount{}),
			9*time.Second, 50*time.Millisecond, // posted every 5-7.5s
		).Should(
			WithTransform(func(c *metrics.ConsumerdCount) int64 {
				return c.Count
			}, BeEquivalentTo(1)),
		)

		agentTasks := testEnv.MetricF(schedCtx, &metrics.AgentTasksTotal{})
		cdTasks := testEnv.MetricF(schedCtx, &metrics.ConsumerdTasksTotal{})
		Eventually(
			func() string {
				a, err := agentTasks()
				if err != nil {
					return err.Error()
				}
				c, err := cdTasks()
				if err != nil {
					return err.Error()
				}
				agent := a.(*metrics.AgentTasksTotal)
				cd := c.(*metrics.ConsumerdTasksTotal)
				if agent.UUID != agentID {
					return "Agent UUIDs do not match"
				}
				if cd.UUID != consumerdID {
					return "Consumerd UUIDs do not match"
				}
				if agent.Total != cd.Total {
					return fmt.Sprintf("Task counts do not match (Agent (%d) != Consumerd: (%d)",
						agent.Total, cd.Total)
				}
				return ""
			},
			9*time.Second, 50*time.Millisecond, // posted every 5-7.5s
		).Should(BeEmpty())
	})

	Specify("shutdown", func() {
		testEnv.Shutdown()
	})
})
