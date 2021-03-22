package controller

import (
	"context"

	"github.com/cobalt77/kubecc/internal/testutil"
	"github.com/cobalt77/kubecc/pkg/clients"
	"github.com/cobalt77/kubecc/pkg/run"
	"github.com/cobalt77/kubecc/pkg/types"
)

type TestToolchainCtrl struct{}

func (r *TestToolchainCtrl) RunLocal(run.ArgParser) run.RequestManager {
	return &localRunnerManager{}
}

func (r *TestToolchainCtrl) SendRemote(_ run.ArgParser, client *clients.CompileRequestClient) run.RequestManager {
	return &sendRemoteRunnerManager{
		client: client,
	}
}

func (r *TestToolchainCtrl) RecvRemote() run.RequestManager {
	return &recvRemoteRunnerManager{}
}

func (r *TestToolchainCtrl) NewArgParser(_ context.Context, args []string) run.ArgParser {
	return &testutil.SleepArgParser{
		Args: args,
	}
}

func AddToStore(store *run.ToolchainRunnerStore) {
	store.Add(types.TestToolchain, &TestToolchainCtrl{})
}

// TestToolchainCtrlLocal is similar to TestToolchainRunner, except that it
// will simulate a remote connection. All tasks are run locally but separate
// executors can be used to measure local and remote usage.
type TestToolchainCtrlLocal struct{}

func (r *TestToolchainCtrlLocal) RunLocal(run.ArgParser) run.RequestManager {
	return &localRunnerManager{}
}

func (r *TestToolchainCtrlLocal) SendRemote(run.ArgParser, *clients.CompileRequestClient) run.RequestManager {
	return &sendRemoteRunnerManagerSim{}
}

func (r *TestToolchainCtrlLocal) RecvRemote() run.RequestManager {
	return &recvRemoteRunnerManagerSim{}
}

func (r *TestToolchainCtrlLocal) NewArgParser(_ context.Context, args []string) run.ArgParser {
	return &testutil.SleepArgParser{
		Args: args,
	}
}

func AddToStoreSim(store *run.ToolchainRunnerStore) {
	store.Add(types.TestToolchain, &TestToolchainCtrlLocal{})
}
