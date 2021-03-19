package consumerd_test

import (
	"time"

	"github.com/cobalt77/kubecc/pkg/apps/consumerd"
	"github.com/cobalt77/kubecc/pkg/test"
	"github.com/cobalt77/kubecc/pkg/types"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Split Queue", func() {
	numTasks := 100
	When("when no scheduler is available", func() {
		Specify("startup", func() {
			testEnv = test.NewDefaultEnvironment()
			testEnv.SpawnMonitor()
			go testEnv.Serve()
			testEnv.WaitForServices([]string{
				types.Monitor_ServiceDesc.ServiceName,
			})
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
			go testEnv.Serve()
			testEnv.WaitForServices([]string{
				types.Monitor_ServiceDesc.ServiceName,
				types.Scheduler_ServiceDesc.ServiceName,
			})
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
