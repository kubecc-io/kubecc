package consumerd_test

import (
	"time"

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

func ewma(kind consumerd.EntryKind) consumerd.Entries {
	e := queue.Telemetry().Entries()
	if len(e) < 2 {
		return consumerd.Entries{}
	}
	return e.Filter(func(e consumerd.Entry) bool {
		return e.Kind == kind
	}).Deltas().EWMA(10 * collectionPeriod)
}

func linReg(e consumerd.Entries, start, end time.Time) float64 {
	_, beta := e.TimeRange(start, end).LinearRegression()
	return beta
}

var _ = Describe("Split Queue", func() {
	Specify("setup", func() {
		testEnv = test.NewDefaultEnvironment()
		queue = consumerd.NewSplitQueue(testCtx,
			testEnv.NewMonitorClient(testCtx),
			consumerd.WithTelemetryConfig(consumerd.TelemetryConfig{
				Enabled:        true,
				RecordInterval: collectionPeriod,
				HistoryLen:     1e4,
			}),
		)
		testEnv.SpawnMonitor()
	})
	Describe("Tasks complete immediately, executors are not queued", func() {
		numTasks := 300
		iterations := 3
		// timestamps
		// 0 = start
		// 1 = after local tasks run
		// 2 = after brief inactivity
		// 3 = after remote+local runs
		// (repeat 2, 3 for num iterations)
		eventTimestamps := []time.Time{time.Now()}
		When("when no scheduler is available", func() {
			Specify("the queue should run all tasks locally", func() {
				taskPool := makeTaskPool(numTasks)
				local, remote := runAllTasks(taskPool)

				Expect(local).To(BeEquivalentTo(numTasks))
				Expect(remote).To(BeEquivalentTo(0))
				eventTimestamps = append(eventTimestamps, time.Now())
			})
		})
		When("a scheduler is available", func() {
			Specify("startup", func() {
				testEnv.SpawnScheduler()
			})
			Measure("the queue should split tasks between local and remote evenly", func(b Benchmarker) {
				time.Sleep(100 * time.Millisecond)
				eventTimestamps = append(eventTimestamps, time.Now())
				numTasks = 1000
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
				eventTimestamps = append(eventTimestamps, time.Now())
			}, iterations)
		})
		Specify("Analyzing telemetry", func() {
			// Analyze portions of the complete EWMA graph to check that the slope
			// is positive or negative in certain places. At times when there should
			// be activity on the queue, the slope should be positive. When the queue
			// is not in use, the slope should turn negative.
			ewmaLocal := ewma(consumerd.CompletedTasksLocal)
			ewmaRemote := ewma(consumerd.CompletedTasksRemote)
			Expect(linReg(ewmaLocal, eventTimestamps[0], eventTimestamps[1])).To(BeNumerically(">", 0))
			Expect(linReg(ewmaLocal, eventTimestamps[1], eventTimestamps[2])).To(BeNumerically("<", 0))
			for i := 1; i <= iterations; i++ {
				Expect(linReg(ewmaLocal, eventTimestamps[i*2], eventTimestamps[i*2+1])).To(BeNumerically(">", 0))
				Expect(linReg(ewmaRemote, eventTimestamps[i*2], eventTimestamps[i*2+1])).To(BeNumerically(">", 0))
				if i < iterations {
					Expect(linReg(ewmaLocal, eventTimestamps[i*2+1], eventTimestamps[i*2+2])).To(BeNumerically("<", 0))
					Expect(linReg(ewmaRemote, eventTimestamps[i*2+1], eventTimestamps[i*2+2])).To(BeNumerically("<", 0))
				}
			}
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
