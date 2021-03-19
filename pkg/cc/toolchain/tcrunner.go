package toolchain

import (
	"context"

	"github.com/cobalt77/kubecc/pkg/cc"
	"github.com/cobalt77/kubecc/pkg/clients"
	"github.com/cobalt77/kubecc/pkg/run"
	"github.com/cobalt77/kubecc/pkg/types"
)

type CCToolchainRunner struct{}

func (r *CCToolchainRunner) RunLocal(ap run.ArgParser) run.RunnerManager {
	return &localRunnerManager{
		ArgParser: ap.(*cc.ArgParser),
	}
}

func (r *CCToolchainRunner) SendRemote(ap run.ArgParser, client *clients.CompileRequestClient) run.RunnerManager {
	return &sendRemoteRunnerManager{
		ArgParser:       ap.(*cc.ArgParser),
		schedulerClient: client,
	}
}

func (r *CCToolchainRunner) RecvRemote() run.RunnerManager {
	return &recvRemoteRunnerManager{}
}

func (r *CCToolchainRunner) NewArgParser(ctx context.Context, args []string) run.ArgParser {
	return cc.NewArgParser(ctx, args)
}

func AddToStore(store *run.ToolchainRunnerStore) {
	runner := &CCToolchainRunner{}
	store.Add(types.Gnu, runner)
	store.Add(types.Clang, runner)
}
