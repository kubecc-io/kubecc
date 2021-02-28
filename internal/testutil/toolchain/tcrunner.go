package toolchain

import (
	"github.com/cobalt77/kubecc/internal/testutil"
	"github.com/cobalt77/kubecc/pkg/meta"
	"github.com/cobalt77/kubecc/pkg/run"
	"github.com/cobalt77/kubecc/pkg/types"
)

type TestToolchainRunner struct{}

func (r *TestToolchainRunner) RunLocal(run.ArgParser) run.RunnerManager {
	return &localRunnerManager{}
}

func (r *TestToolchainRunner) SendRemote(_ run.ArgParser, client types.SchedulerClient) run.RunnerManager {
	return &sendRemoteRunnerManager{
		client: client,
	}
}

func (r *TestToolchainRunner) RecvRemote() run.RunnerManager {
	return &recvRemoteRunnerManager{}
}

func (r *TestToolchainRunner) NewArgParser(_ meta.Context, args []string) run.ArgParser {
	return &testutil.TestArgParser{
		Args: args,
	}
}

func AddToStore(store *run.ToolchainRunnerStore) {
	store.Add(types.TestToolchain, &TestToolchainRunner{})
}
