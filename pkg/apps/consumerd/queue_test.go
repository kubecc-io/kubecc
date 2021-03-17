package consumerd_test

import (
	"context"

	"github.com/cobalt77/kubecc/internal/logkc"
	"github.com/cobalt77/kubecc/internal/testutil"
	testtoolchain "github.com/cobalt77/kubecc/internal/testutil/toolchain"
	"github.com/cobalt77/kubecc/pkg/apps/consumerd"
	"github.com/cobalt77/kubecc/pkg/identity"
	"github.com/cobalt77/kubecc/pkg/meta"
	"github.com/cobalt77/kubecc/pkg/metrics"
	"github.com/cobalt77/kubecc/pkg/run"
	"github.com/cobalt77/kubecc/pkg/tracing"
	"github.com/cobalt77/kubecc/pkg/types"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"go.uber.org/atomic"
)

type testExecutor struct {
	numTasks  *atomic.Int32
	completed *atomic.Int32
}

func (x *testExecutor) Exec(task *run.Task) error {
	x.numTasks.Inc()
	defer x.numTasks.Dec()

	go func() {
		defer GinkgoRecover()
		task.Run()
	}()
	select {
	case <-task.Done():
	case <-task.Context().Done():
	}
	x.completed.Inc()
	return task.Error()
}

func newTestExecutor() *testExecutor {
	return &testExecutor{
		numTasks:  atomic.NewInt32(0),
		completed: atomic.NewInt32(0),
	}
}

func (x *testExecutor) CompleteUsageLimits(*metrics.UsageLimits) {

}

func (x *testExecutor) CompleteTaskStatus(s *metrics.TaskStatus) {
	s.NumDelegated = x.numTasks.Load()
}

func (x *testExecutor) ExecAsync(task *run.Task) <-chan error {
	panic("not implemented")
}

var _ = Describe("Split Queue", func() {
	testCtx := meta.NewContext(
		meta.WithProvider(identity.Component, meta.WithValue(types.TestComponent)),
		meta.WithProvider(identity.UUID),
		meta.WithProvider(logkc.Logger),
		meta.WithProvider(tracing.Tracer),
	)

	numTasks := 100
	taskPool := make(chan *consumerd.SplitTask, numTasks)
	cleanup := make(chan context.CancelFunc, 100)
	localExec := newTestExecutor()
	remoteExec := newTestExecutor()
	tc := &types.Toolchain{
		Kind:       types.Gnu,
		Lang:       types.CXX,
		Executable: testutil.TestToolchainExecutable,
		TargetArch: "testarch",
		Version:    "0",
		PicDefault: true,
	}
	taskArgs := []string{"-duration", "0"}
	rm := &testtoolchain.TestToolchainRunner{}
	request := &types.RunRequest{
		Compiler: &types.RunRequest_Toolchain{
			Toolchain: tc,
		},
		Args: taskArgs,
		UID:  1000,
		GID:  1000,
	}

	BeforeEach(func() {
		schedulerClient := testEnv.NewSchedulerClient()

		Expect(len(taskPool)).To(Equal(0))
		Expect(cap(taskPool)).To(Equal(numTasks))

		for i := 0; i < numTasks; i++ {
			contexts := run.Contexts{
				ServerContext: testCtx,
				ClientContext: testCtx,
			}
			taskPool <- &consumerd.SplitTask{
				Local: run.Package(
					rm.RunLocal(rm.NewArgParser(testCtx, taskArgs)),
					contexts,
					localExec,
					request,
				),
				Remote: run.Package(
					rm.SendRemote(
						rm.NewArgParser(testCtx, taskArgs),
						schedulerClient,
					),
					contexts,
					remoteExec,
					request,
				),
			}
		}

		_, cf := testEnv.SpawnMonitor()
		cleanup <- cf
	})

	AfterEach(func() {
		for c := range cleanup {
			c()
		}
	})

	PSpecify("when no scheduler is available, the queue should run all tasks locally", func() {
		sq := consumerd.NewSplitQueue(testCtx, testEnv.NewMonitorClient())
		for task := range taskPool {
			sq.In() <- task
		}
		Eventually(func() int32 {
			return localExec.numTasks.Load()
		}).Should(Equal(int32(numTasks)))
	})
})
