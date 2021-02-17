package testutil

import (
	"context"
	"time"

	"github.com/cobalt77/kubecc/pkg/run"
	"github.com/cobalt77/kubecc/pkg/toolchains"
	"github.com/cobalt77/kubecc/pkg/types"
)

var TestToolchainExecutable = "/path/to/test-toolchain"

type TestQuerier struct{}

func (q TestQuerier) Version(compiler string) (string, error) {
	return "0", nil
}

func (q TestQuerier) TargetArch(compiler string) (string, error) {
	return "testarch", nil
}

func (q TestQuerier) IsPicDefault(compiler string) (bool, error) {
	return true, nil
}

func (q TestQuerier) Kind(compiler string) (types.ToolchainKind, error) {
	return types.TestToolchain, nil
}

func (q TestQuerier) Lang(compiler string) (types.ToolchainLang, error) {
	return types.CXX, nil
}

func (q TestQuerier) ModTime(compiler string) (time.Time, error) {
	return time.Unix(0, 0), nil
}

type TestToolchainFinder struct{}

func (f TestToolchainFinder) FindToolchains(
	ctx context.Context,
	opts ...toolchains.FindOption,
) *toolchains.Store {
	store := toolchains.NewStore()
	_, _ = store.Add(TestToolchainExecutable, TestQuerier{})
	return store
}

type SleepRunner struct {
	Duration time.Duration
}

func (r *SleepRunner) Run(_ context.Context, _ *types.Toolchain) error {
	time.Sleep(r.Duration)
	return nil
}

type TestRunnerManager struct {
	Executor run.Executor
}
type TestRunRequest struct {
	Toolchain *types.Toolchain
	Duration  time.Duration
}
type TestRunResponse struct{}

func (r *TestRunnerManager) Run(
	ctx run.Contexts,
	x run.Executor,
	request interface{},
) (response interface{}, err error) {
	req := request.(*TestRunRequest)
	task := run.NewTask(ctx.ClientContext, &SleepRunner{
		Duration: req.Duration,
	}, req.Toolchain)
	err = x.Exec(task)
	return &TestRunResponse{}, err
}
