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

package consumerd_test

import (
	"context"
	"time"

	"github.com/kubecc-io/kubecc/pkg/apps/consumerd"
	"github.com/kubecc-io/kubecc/pkg/clients"
	"github.com/kubecc-io/kubecc/pkg/test"
	"github.com/kubecc-io/kubecc/pkg/types"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"go.uber.org/atomic"
)

func runTasksInf(
	taskPool chan *consumerd.SplitTask,
	queue *consumerd.SplitQueue,
	requestCounts chan struct{},
) chan counts {
	local := atomic.NewInt32(0)
	remote := atomic.NewInt32(0)
	c := make(chan counts)
	go func() {
		for task := range taskPool {
			if err := queue.Exec(task); err != nil {
				panic(err)
			}
			go func(task *consumerd.SplitTask) {
				_, err := task.Wait()
				if err != nil {
					panic(err)
				}
				switch task.Which() {
				case consumerd.Local:
					local.Inc()
				case consumerd.Remote:
					remote.Inc()
				case consumerd.Unknown:
					panic("consumerd.Unknown")
				}
			}(task)
		}
	}()
	go func() {
		for {
			<-requestCounts
			c <- counts{
				local:  int64(local.Swap(0)),
				remote: int64(remote.Swap(0)),
			}
		}
	}()
	return c
}

func runAllTasks(
	taskPool chan *consumerd.SplitTask,
	queue *consumerd.SplitQueue,
) (local, remote int64) {
	pending := []*consumerd.SplitTask{}
	// Block until the first task has been received
	task := <-taskPool
	if err := queue.Exec(task); err != nil {
		panic(err)
	}
	pending = append(pending, task)

	// Read tasks from the channel until there are none left
	for {
		select {
		case task := <-taskPool:
			if err := queue.Exec(task); err != nil {
				panic(err)
			}
			pending = append(pending, task)
		default:
			goto wait
		}
	}
	// This is to avoid spawning thousands of goroutines which will break
	// the race detector.
wait:
	for _, task := range pending {
		_, err := task.Wait()
		if err != nil {
			panic(err)
		}
		switch task.Which() {
		case consumerd.Local:
			local++
		case consumerd.Remote:
			remote++
		case consumerd.Unknown:
			panic("consumerd.Unknown")
		}
	}
	return
}

func ewma(queue *consumerd.SplitQueue, kind consumerd.EntryKind) consumerd.Entries {
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

var _ = Describe("Basic Functionality", func() {
	var queue *consumerd.SplitQueue
	Specify("setup", func() {
		testEnv = test.NewDefaultEnvironment()
		queue = consumerd.NewSplitQueue(testCtx,
			testEnv.NewMonitorClient(testCtx),
			consumerd.WithTelemetryConfig(consumerd.TelemetryConfig{
				Enabled:        true,
				RecordInterval: collectionPeriod,
				HistoryLen:     1e4,
			}),
			consumerd.WithLocalUsageManager(consumerd.FixedUsageLimits(35)),
			consumerd.WithRemoteUsageManager(consumerd.FixedUsageLimits(50)),
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
				local, remote := runAllTasks(taskPool, queue)

				Expect(local).To(BeEquivalentTo(numTasks))
				Expect(remote).To(BeEquivalentTo(0))
				eventTimestamps = append(eventTimestamps, time.Now())
			})
		})
		When("a scheduler is available", func() {
			Specify("startup", func() {
				testEnv.SpawnScheduler(test.WaitForReady())
			})
			Measure("the queue should split tasks between local and remote evenly", func(b Benchmarker) {
				time.Sleep(100 * time.Millisecond) // important
				eventTimestamps = append(eventTimestamps, time.Now())
				numTasks = 1000
				taskPool := makeTaskPool(numTasks)
				monClient := testEnv.NewMonitorClient(testCtx)
				avc := clients.NewAvailabilityChecker(
					clients.ComponentFilter(types.Scheduler),
				)
				clients.WatchAvailability(testCtx, monClient, avc)
				By("Waiting for scheduler")
				avc.EnsureAvailable()

				By("Running tasks")
				local, remote := runAllTasks(taskPool, queue)
				Expect(local + remote).To(BeEquivalentTo(numTasks))
				Expect(local).To(BeNumerically(">", 0))
				Expect(remote).To(BeNumerically(">", 0))
				b.RecordValue("local", float64(local))
				b.RecordValue("remote", float64(remote))
				eventTimestamps = append(eventTimestamps, time.Now())
			}, iterations)
		})
		Specify("Analyzing telemetry", func() {
			Expect(len(eventTimestamps)).To(Equal(iterations*2 + 2))
			// Analyze portions of the complete EWMA graph to check that the slope
			// is positive or negative in certain places. At times when there should
			// be activity on the queue, the slope should be positive. When the queue
			// is not in use, the slope should turn negative.
			ewmaLocal := ewma(queue, consumerd.CompletedTasksLocal)
			ewmaRemote := ewma(queue, consumerd.CompletedTasksRemote)
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
		Specify("Saving telemetry graph", func() {
			test.SkipInGithubWorkflow()
			plotStatsLog(queue)
		})
	})
	Specify("Shutdown", func() {
		testEnv.Shutdown()
	})
})

type counts struct {
	local, remote int64
}

var _ = Describe("Task Redirection", func() {
	// These tasks will keep the queue at max capacity, since they will be
	// added much faster than they can be completed
	taskPool := makeInfiniteTaskPool([]string{"-sleep", "10ms"})
	var queue *consumerd.SplitQueue
	var countsCh chan counts
	requestCounts := make(chan struct{})
	expectedTasks := 150
	if test.InGithubWorkflow() {
		expectedTasks = 75
	}
	Specify("setup", func() {
		testEnv = test.NewDefaultEnvironment()
		queue = consumerd.NewSplitQueue(testCtx,
			testEnv.NewMonitorClient(testCtx),
			consumerd.WithTelemetryConfig(consumerd.TelemetryConfig{
				Enabled: false,
			}),
			consumerd.WithLocalUsageManager(consumerd.FixedUsageLimits(10)),
			consumerd.WithRemoteUsageManager(consumerd.FixedUsageLimits(10)),
		)
		testEnv.SpawnMonitor()
		countsCh = runTasksInf(taskPool.C, queue, requestCounts)
	})
	When("no remote is available yet", func() {
		It("should run all tasks locally", func() {
			By("Waiting 100ms")
			time.Sleep(200 * time.Millisecond)
			By("Checking counts")
			requestCounts <- struct{}{}
			counts := <-countsCh
			// At this point in time the queue should be near-full and about 200 tasks
			// should have been completed locally
			Expect(counts.local).To(BeNumerically(">", expectedTasks))
			Expect(counts.remote).To(BeEquivalentTo(0))
		})
	})
	When("the remote becomes available", func() {
		var cancel context.CancelFunc
		Specify("starting scheduler", func() {
			_, cancel = testEnv.SpawnScheduler()
		})
		It("should split tasks between local and remote", func() {
			By("Waiting 100ms")
			requestCounts <- struct{}{} // set the counts back to 0
			<-countsCh
			time.Sleep(200 * time.Millisecond)
			By("Checking counts")
			requestCounts <- struct{}{}
			counts := <-countsCh
			// At this point in time the queue should be near-full and about 200 tasks
			// should have been completed on both local and remote
			Expect(counts.local).To(BeNumerically(">", expectedTasks))
			Expect(counts.remote).To(BeNumerically(">", expectedTasks))
		})
		Specify("stopping scheduler", func() {
			cancel()
			// wait for any active remote tasks to be completed/canceled
			time.Sleep(50 * time.Millisecond)
		})
	})
	When("the remote becomes unavailable", func() {
		It("should run all tasks locally", func() {
			By("Waiting 100ms")
			requestCounts <- struct{}{} // set the counts back to 0
			<-countsCh
			time.Sleep(200 * time.Millisecond)
			By("Checking counts")
			requestCounts <- struct{}{}
			counts := <-countsCh
			// At this point in time the queue should be near-full and about 200 tasks
			// should have been completed locally
			Expect(counts.local).To(BeNumerically(">", expectedTasks))
			Expect(counts.remote).To(BeEquivalentTo(0))
		})
	})
	Specify("shutdown", func() {
		taskPool.Cancel()
		testEnv.Shutdown()
	})
})
