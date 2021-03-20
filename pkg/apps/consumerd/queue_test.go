package consumerd_test

import (
	"context"
	"time"

	"github.com/cobalt77/kubecc/pkg/apps/consumerd"
	"github.com/cobalt77/kubecc/pkg/clients"
	"github.com/cobalt77/kubecc/pkg/test"
	"github.com/cobalt77/kubecc/pkg/types"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func processTasks(taskPool chan *consumerd.SplitTask, sq *consumerd.SplitQueue) {
	go func() {
		defer GinkgoRecover()
		for {
			select {
			case task := <-taskPool:
				sq.Put(task)
				go func() {
					_, err := task.Wait()
					Expect(err).NotTo(HaveOccurred())
				}()
			default:
				return
			}
		}
	}()
}

func resetCounts() {
	localExec.completed.Store(0)
	remoteExec.completed.Store(0)
}

var _ = Describe("Split Queue", func() {
	Describe("Tasks complete immediately, executors are not queued", func() {
		numTasks := 100
		When("when no scheduler is available", func() {
			Specify("startup", func() {
				testEnv = test.NewDefaultEnvironment()
				testEnv.SpawnMonitor()
			})
			Specify("the queue should run all tasks locally", func() {
				taskPool := makeTaskPool(numTasks)
				sq := consumerd.NewSplitQueue(testCtx, testEnv.NewMonitorClient(testCtx))
				processTasks(taskPool, sq)
				Eventually(func() int32 {
					return localExec.completed.Load()
				}, 5*time.Second, 10*time.Millisecond).Should(Equal(int32(numTasks)))
			})
			Specify("shutdown", func() {
				testEnv.Shutdown()
			})
		})
		When("a scheduler is available", func() {
			Specify("startup", func() {
				resetCounts()
				testEnv = test.NewDefaultEnvironment()
				testEnv.SpawnMonitor()
				testEnv.SpawnScheduler()
			})
			Measure("the queue should split tasks between local and remote evenly", func(b Benchmarker) {
				resetCounts()
				taskPool := makeTaskPool(numTasks)
				monClient := testEnv.NewMonitorClient(testCtx)
				sq := consumerd.NewSplitQueue(testCtx, monClient)
				avc := clients.NewAvailabilityChecker(
					clients.ComponentFilter(types.Scheduler),
				)
				clients.WatchAvailability(testCtx, types.Scheduler, monClient, avc)
				avc.EnsureAvailable()

				processTasks(taskPool, sq)
				Eventually(func() int32 {
					return localExec.completed.Load() + remoteExec.completed.Load()
				}, 5*time.Second, 10*time.Millisecond).Should(Equal(int32(numTasks)))
				Expect(localExec.completed.Load()).To(BeNumerically(">", 0))
				Expect(remoteExec.completed.Load()).To(BeNumerically(">", 0))
				b.RecordValue("local", float64(localExec.completed.Load()))
				b.RecordValue("remote", float64(remoteExec.completed.Load()))
			}, 3)
			Specify("shutdown", func() {
				testEnv.Shutdown()
			})
		})
	})

	PDescribe("Redirecting tasks when state changes", func() {
		var cancelPool context.CancelFunc
		Specify("startup", func() {
			testEnv = test.NewDefaultEnvironment()
			testEnv.SpawnMonitor()
			taskPool, cancel := makeInfiniteTaskPool()
			cancelPool = cancel
			sq := consumerd.NewSplitQueue(testCtx, testEnv.NewMonitorClient(testCtx))
			processTasks(taskPool, sq)
		})
		When("no remote is available yet", func() {
			Specify("all tasks should be executed locally", func() {
				for i := 0; i < 100; i++ {
					outLog.Info(localExec.Slope(), remoteExec.Slope())
				}
			})
		})
		When("the remote becomes available", func() {

		})
		When("the remote becomes unavailable", func() {

		})
		Specify("shutdown", func() {
			cancelPool()
			testEnv.Shutdown()
		})
	})
})
