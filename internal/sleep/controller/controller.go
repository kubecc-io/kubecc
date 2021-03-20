package controller

import (
	"context"

	"github.com/cobalt77/kubecc/pkg/clients"
	"github.com/cobalt77/kubecc/pkg/run"
	"github.com/cobalt77/kubecc/pkg/types"
)

type SleepToolchainCtrl struct{}

func (r *SleepToolchainCtrl) RunLocal(run.ArgParser) run.RequestManager {
	return &localRunnerManager{}
}

func (r *SleepToolchainCtrl) SendRemote(_ run.ArgParser, client *clients.CompileRequestClient) run.RequestManager {
	return &sendRemoteRunnerManager{
		client: client,
	}
}

func (r *SleepToolchainCtrl) RecvRemote() run.RequestManager {
	return &recvRemoteRunnerManager{}
}

func (r *SleepToolchainCtrl) NewArgParser(context.Context, []string) run.ArgParser {
	return &NoopArgParser{}
}

func AddToStore(store *run.ToolchainRunnerStore) {
	store.Add(types.Sleep, &SleepToolchainCtrl{})
}

type NoopArgParser struct{}

func (ap *NoopArgParser) Parse() {}

func (NoopArgParser) CanRunRemote() bool {
	return true
}
