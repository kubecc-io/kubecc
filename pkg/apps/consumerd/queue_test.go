package consumerd_test

import (
	"github.com/cobalt77/kubecc/pkg/apps/consumerd"
	"github.com/cobalt77/kubecc/pkg/clients"
	"github.com/cobalt77/kubecc/pkg/test"
	"github.com/cobalt77/kubecc/pkg/types"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func runAllTasks(taskPool chan *consumerd.SplitTask) (local, remote int64) {
	pending := make(chan *consumerd.SplitTask, cap(taskPool))
	for {
		select {
		case task := <-taskPool:
			err := queue.Exec(task)
			Expect(err).NotTo(HaveOccurred())
			pending <- task
		default:
			goto wait
		}
	}
	// This is to avoid spawning thousands of goroutines which will break
	// the race detector.
wait:
	for {
		select {
		case task := <-pending:
			_, err := task.Wait()
			Expect(err).NotTo(HaveOccurred())
			switch task.Which() {
			case consumerd.Local:
				local++
			case consumerd.Remote:
				remote++
			case consumerd.Unknown:
				panic("consumerd.Unknown")
			}
		default:
			return
		}
	}
}

var _ = Describe("Split Queue", func() {
	Specify("setup", func() {
		testEnv = test.NewDefaultEnvironment()
		queue = consumerd.NewSplitQueue(testCtx,
			testEnv.NewMonitorClient(testCtx),
			consumerd.WithTelemetryConfig(consumerd.TelemetryConfig{
				Enabled:    true,
				SampleRate: 0.1,
				HistoryLen: 1e4,
			}),
		)
		testEnv.SpawnMonitor()
	})
	Describe("Tasks complete immediately, executors are not queued", func() {
		numTasks := 3000
		When("when no scheduler is available", func() {
			Specify("the queue should run all tasks locally", func() {
				taskPool := makeTaskPool(numTasks)
				local, remote := runAllTasks(taskPool)
				Expect(local + remote).To(BeEquivalentTo(numTasks))
			})
		})
		When("a scheduler is available", func() {
			Specify("startup", func() {
				testEnv.SpawnScheduler()
			})
			Measure("the queue should split tasks between local and remote evenly", func(b Benchmarker) {
				numTasks = 10000
				taskPool := makeTaskPool(numTasks)
				monClient := testEnv.NewMonitorClient(testCtx)
				avc := clients.NewAvailabilityChecker(
					clients.ComponentFilter(types.Scheduler),
				)
				clients.WatchAvailability(testCtx, types.Scheduler, monClient, avc)
				avc.EnsureAvailable()

				local, remote := runAllTasks(taskPool)
				Expect(local + remote).To(BeEquivalentTo(numTasks))
				Expect(local).To(BeNumerically(">", 0))
				Expect(remote).To(BeNumerically(">", 0))
				b.RecordValue("local", float64(local))
				b.RecordValue("remote", float64(remote))
			}, 3)
		})
	})

	// PDescribe("Redirecting tasks when state changes", func() {
	// 	var cancelPool context.CancelFunc
	// 	Specify("startup", func() {
	// 		testEnv = test.NewDefaultEnvironment()
	// 		testEnv.SpawnMonitor()
	// 		taskPool, cancel := makeInfiniteTaskPool()
	// 		cancelPool = cancel
	// 		sq := consumerd.NewSplitQueue(testCtx, testEnv.NewMonitorClient(testCtx))
	// 		processTasks(taskPool, sq)
	// 	})
	// 	When("no remote is available yet", func() {
	// 		Specify("all tasks should be executed locally", func() {
	// 			for i := 0; i < 100; i++ {
	// 				outLog.Info(localExec.Slope(), remoteExec.Slope())
	// 			}
	// 		})
	// 	})
	// 	When("the remote becomes available", func() {

	// 	})
	// 	When("the remote becomes unavailable", func() {

	// 	})
	// 	Specify("shutdown", func() {
	// 		cancelPool()
	// 		testEnv.Shutdown()
	// 	})
	// })
})
