package toolchain

import (
	"context"

	"github.com/cobalt77/kubecc/pkg/clients"
	"github.com/cobalt77/kubecc/pkg/run"
	"github.com/cobalt77/kubecc/pkg/types"
)

type SleepToolchainRunner struct{}

func (r *SleepToolchainRunner) RunLocal(run.ArgParser) run.RunnerManager {
	return &localRunnerManager{}
}

func (r *SleepToolchainRunner) SendRemote(_ run.ArgParser, client *clients.CompileRequestClient) run.RunnerManager {
	return &sendRemoteRunnerManager{
		client: client,
	}
}

func (r *SleepToolchainRunner) RecvRemote() run.RunnerManager {
	return &recvRemoteRunnerManager{}
}

func (r *SleepToolchainRunner) NewArgParser(context.Context, []string) run.ArgParser {
	return &NoopArgParser{}
}

func AddToStore(store *run.ToolchainRunnerStore) {
	store.Add(types.Sleep, &SleepToolchainRunner{})
}

type NoopArgParser struct{}

func (ap *NoopArgParser) Parse() {}

func (NoopArgParser) CanRunRemote() bool {
	return true
}
