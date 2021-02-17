package toolchain

import (
	"github.com/cobalt77/kubecc/pkg/run"
	"github.com/cobalt77/kubecc/pkg/types"
)

type TestToolchainRunner struct{}

func (r *TestToolchainRunner) RunLocal(ap run.ArgParserTodo) run.RunnerManager {
	return &localRunnerManager{}
}

func (r *TestToolchainRunner) SendRemote(ap run.ArgParserTodo, client types.SchedulerClient) run.RunnerManager {
	return &sendRemoteRunnerManager{
		client: client,
	}
}

func (r *TestToolchainRunner) RecvRemote() run.RunnerManager {
	return &recvRemoteRunnerManager{}
}

func AddToStore(store *run.ToolchainRunnerStore) {
	store.Add(types.TestToolchain, &TestToolchainRunner{})
}
