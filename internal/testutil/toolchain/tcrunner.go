package toolchain

import (
	"context"

	"github.com/cobalt77/kubecc/internal/testutil"
	"github.com/cobalt77/kubecc/pkg/clients"
	"github.com/cobalt77/kubecc/pkg/run"
	"github.com/cobalt77/kubecc/pkg/types"
)

type TestToolchainRunner struct{}

func (r *TestToolchainRunner) RunLocal(run.ArgParser) run.RunnerManager {
	return &localRunnerManager{}
}

func (r *TestToolchainRunner) SendRemote(_ run.ArgParser, client *clients.CompileRequestClient) run.RunnerManager {
	return &sendRemoteRunnerManager{
		client: client,
	}
}

func (r *TestToolchainRunner) RecvRemote() run.RunnerManager {
	return &recvRemoteRunnerManager{}
}

func (r *TestToolchainRunner) NewArgParser(_ context.Context, args []string) run.ArgParser {
	return &testutil.TestArgParser{
		Args: args,
	}
}

func AddToStore(store *run.ToolchainRunnerStore) {
	store.Add(types.TestToolchain, &TestToolchainRunner{})
}

type NoopToolchainRunner struct{}

func (r *NoopToolchainRunner) RunLocal(run.ArgParser) run.RunnerManager {
	return &localRunnerManagerNoop{}
}

func (r *NoopToolchainRunner) SendRemote(run.ArgParser, *clients.CompileRequestClient) run.RunnerManager {
	return &sendRemoteRunnerManagerNoop{}
}

func (r *NoopToolchainRunner) RecvRemote() run.RunnerManager {
	return &recvRemoteRunnerManagerNoop{}
}

func (r *NoopToolchainRunner) NewArgParser(context.Context, []string) run.ArgParser {
	return &testutil.NoopArgParser{}
}

func AddToStoreNoop(store *run.ToolchainRunnerStore) {
	store.Add(types.TestToolchain, &NoopToolchainRunner{})
}
