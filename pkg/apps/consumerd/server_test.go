package consumerd_test

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"go.uber.org/zap/zapcore"

	"github.com/kubecc-io/kubecc/pkg/apps/agent"
	"github.com/kubecc-io/kubecc/pkg/apps/consumerd"
	"github.com/kubecc-io/kubecc/pkg/metrics"
	"github.com/kubecc-io/kubecc/pkg/test"
	"github.com/kubecc-io/kubecc/pkg/types"
)

var _ = Describe("Consumerd Server", func() {
	Specify("setup", func() {
		testEnv = test.NewEnvironmentWithLogLevel(zapcore.ErrorLevel)
		testEnv.SpawnMonitor()
		testEnv.SpawnScheduler(test.WaitForReady())
		testEnv.SpawnAgent(test.WaitForReady(), test.WithAgentOptions(
			agent.WithUsageLimits(&metrics.UsageLimits{
				ConcurrentProcessLimit: 5,
			}),
		))
	})

	var cdCtx context.Context
	It("should eventually become ready", func() {
		cdCtx, _ = testEnv.SpawnConsumerd(test.WaitForReady(), test.WithConsumerdOptions(
			consumerd.WithQueueOptions(
				consumerd.WithLocalUsageManager(consumerd.FixedUsageLimits(5)),
				consumerd.WithRemoteUsageManager(consumerd.FixedUsageLimits(5)),
			),
		))
		test.EventuallyHealthStatusShouldBeReady(cdCtx, testEnv)
	})

	It("should post metrics", func() {
		Eventually(testEnv.MetricF(cdCtx, &metrics.UsageLimits{})).Should(Not(BeNil()))
		Eventually(testEnv.MetricF(cdCtx, &metrics.TaskStatus{})).Should(test.EqualProto(
			&metrics.TaskStatus{
				NumRunning:   0,
				NumQueued:    0,
				NumDelegated: 0,
			},
		))
		Eventually(testEnv.MetricF(cdCtx, &metrics.LocalTasksCompleted{})).Should(test.EqualProto(
			&metrics.LocalTasksCompleted{
				Total: 0,
			},
		))
		Eventually(testEnv.MetricF(cdCtx, &metrics.Toolchains{})).Should(test.EqualProto(
			&metrics.Toolchains{
				Items: []*types.Toolchain{
					test.DefaultTestToolchain,
				},
			},
		))
	})

	numTasks := 11
	It("should run tasks", func() {
		By("Running a task pool")
		go func() {
			defer GinkgoRecover()
			// timeout of 4000ms to ensure the tasks complete before the consumerd
			// can send out its first XTasksCompleted metrics
			test.ProcessTaskPool(testEnv, numTasks, test.MakeSleepTaskPool(numTasks, func() string {
				return "1s"
			}), 4000*time.Millisecond)
		}()
		getLocalTasks := testEnv.MetricF(cdCtx, &metrics.LocalTasksCompleted{})
		getRemoteTasks := testEnv.MetricF(cdCtx, &metrics.DelegatedTasksCompleted{})
		getTaskStatus := testEnv.MetricF(cdCtx, &metrics.TaskStatus{})

		By("ensuring metrics reflect correct task and queue status")
		// while running
		Eventually(
			func() bool {
				s, err := getTaskStatus()
				if err != nil {
					return false
				}
				stats := s.(*metrics.TaskStatus)
				return stats.NumRunning == 5 && stats.NumDelegated == 5 && stats.NumQueued == 1
			},
			4*time.Second, 60*time.Millisecond, // posted every 160-500ms
		).Should(BeTrue())

		By("ensuring metrics reflect all tasks completing")
		// after running
		Eventually(
			testEnv.MetricF(cdCtx, &metrics.TaskStatus{}),
			4*time.Second, 60*time.Millisecond, // posted every 160-500ms
		).Should(test.EqualProto(
			&metrics.TaskStatus{
				NumRunning:   0,
				NumQueued:    0,
				NumDelegated: 0,
			},
		))

		By("ensuring metrics reflect the correct number of completed tasks")
		Eventually(
			func() bool {
				local, err := getLocalTasks()
				if err != nil {
					return false
				}
				remote, err := getRemoteTasks()
				if err != nil {
					return false
				}
				totalL := local.(*metrics.LocalTasksCompleted).Total
				totalR := remote.(*metrics.DelegatedTasksCompleted).Total
				return totalL > 0 && totalR > 0 && totalL+totalR == int64(numTasks)
			},
			7*time.Second, 50*time.Millisecond, // posted every 5-6.25s
		).Should(BeTrue())
	})

	Specify("shutdown", func() {
		testEnv.Shutdown()
	})
})
