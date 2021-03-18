package consumerd_test

import (
	"testing"

	"github.com/cobalt77/kubecc/internal/logkc"
	"github.com/cobalt77/kubecc/internal/testutil"
	testtoolchain "github.com/cobalt77/kubecc/internal/testutil/toolchain"
	"github.com/cobalt77/kubecc/pkg/apps/consumerd"
	"github.com/cobalt77/kubecc/pkg/identity"
	"github.com/cobalt77/kubecc/pkg/meta"
	"github.com/cobalt77/kubecc/pkg/metrics"
	"github.com/cobalt77/kubecc/pkg/run"
	"github.com/cobalt77/kubecc/pkg/test"
	"github.com/cobalt77/kubecc/pkg/tracing"
	"github.com/cobalt77/kubecc/pkg/types"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"go.uber.org/atomic"
)

var (
	testEnv *test.Environment
	testCtx = meta.NewContext(
		meta.WithProvider(identity.Component, meta.WithValue(types.TestComponent)),
		meta.WithProvider(identity.UUID),
		meta.WithProvider(logkc.Logger),
		meta.WithProvider(tracing.Tracer),
	)
	testToolchainRunner = &testtoolchain.TestToolchainRunner{}
	taskArgs            = []string{"-duration", "0"}
	localExec           = newTestExecutor()
	remoteExec          = newTestExecutor()
)

func makeTaskPool(numTasks int) chan *consumerd.SplitTask {
	taskPool := make(chan *consumerd.SplitTask, numTasks)
	tc := &types.Toolchain{
		Kind:       types.Gnu,
		Lang:       types.CXX,
		Executable: testutil.TestToolchainExecutable,
		TargetArch: "testarch",
		Version:    "0",
		PicDefault: true,
	}

	for i := 0; i < numTasks; i++ {
		contexts := run.Contexts{
			ServerContext: testCtx,
			ClientContext: testCtx,
		}
		request := &types.RunRequest{
			Compiler: &types.RunRequest_Toolchain{
				Toolchain: tc,
			},
			Args: taskArgs,
			UID:  1000,
			GID:  1000,
		}

		taskPool <- &consumerd.SplitTask{
			Local: run.Package(
				testToolchainRunner.RunLocal(
					testToolchainRunner.NewArgParser(testCtx, taskArgs)),
				contexts,
				localExec,
				request,
			),
			Remote: run.Package(
				testToolchainRunner.SendRemote(
					testToolchainRunner.NewArgParser(testCtx, taskArgs),
					nil,
				),
				contexts,
				remoteExec,
				request,
			),
		}
	}
	return taskPool
}

func TestConsumerd(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Consumerd Suite")
}

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
