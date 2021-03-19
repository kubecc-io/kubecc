package consumerd_test

import (
	"time"

	"github.com/cobalt77/kubecc/pkg/apps/consumerd"
	"github.com/cobalt77/kubecc/pkg/test"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Split Queue", func() {
	Describe("Basic Tests", func() {
		numTasks := 100
		When("when no scheduler is available", func() {
			Specify("startup", func() {
				testEnv = test.NewDefaultEnvironment()
				testEnv.SpawnMonitor()
			})
			Specify("the queue should run all tasks locally", func() {
				taskPool := makeTaskPool(numTasks)
				sq := consumerd.NewSplitQueue(testCtx, testEnv.NewMonitorClient(testCtx))
				go func() {
					defer GinkgoRecover()
					for {
						select {
						case task := <-taskPool:
							sq.In() <- task
							go func() {
								_, err := task.Wait()
								Expect(err).NotTo(HaveOccurred())
							}()
						default:
							return
						}
					}
				}()
				Eventually(func() int32 {
					return localExec.completed.Load()
				}, 10*time.Second, 100*time.Millisecond).Should(Equal(int32(numTasks)))
			})
			Specify("shutdown", func() {
				testEnv.Shutdown()
			})
		})
		When("a scheduler is available", func() {
			Specify("startup", func() {
				testEnv = test.NewDefaultEnvironment()
				testEnv.SpawnMonitor()
				testEnv.SpawnScheduler()
			})
			Specify("the queue should split tasks between local and remote", func() {
				initial := int(localExec.completed.Load() + remoteExec.completed.Load())
				taskPool := makeTaskPool(numTasks)
				sq := consumerd.NewSplitQueue(testCtx, testEnv.NewMonitorClient(testCtx))
				time.Sleep(100 * time.Millisecond)
				go func() {
					defer GinkgoRecover()
					for {
						select {
						case task := <-taskPool:
							sq.In() <- task
							go func() {
								_, err := task.Wait()
								Expect(err).NotTo(HaveOccurred())

							}()
						default:
							return
						}
					}
				}()
				Eventually(func() int32 {
					return localExec.completed.Load() + remoteExec.completed.Load()
				}, 10*time.Second, 100*time.Millisecond).Should(Equal(int32(initial + numTasks)))
				Expect(localExec.completed.Load()).To(BeNumerically(">", 0))
				Expect(remoteExec.completed.Load()).To(BeNumerically(">", 0))
			})
			Specify("shutdown", func() {
				testEnv.Shutdown()
			})
		})
	})

	Describe("Redirecting tasks when state changes", func() {
		Specify("startup", func() {
			testEnv = test.NewDefaultEnvironment()
			testEnv.SpawnMonitor()
		})
		When("no remote is available yet", func() {

		})
		When("the remote becomes available", func() {

		})
		When("the remote becomes unavailable", func() {

		})
		Specify("shutdown", func() {
			testEnv.Shutdown()
		})
	})
})
