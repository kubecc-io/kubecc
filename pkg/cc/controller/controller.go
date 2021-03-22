package toolchain

import (
	"context"

	"github.com/cobalt77/kubecc/pkg/cc"
	"github.com/cobalt77/kubecc/pkg/clients"
	"github.com/cobalt77/kubecc/pkg/run"
	"github.com/cobalt77/kubecc/pkg/types"
)

type CCToolchainCtrl struct {
	tc *types.Toolchain
}

func (r *CCToolchainCtrl) With(tc *types.Toolchain) {
	r.tc = tc
}

func (r *CCToolchainCtrl) RunLocal(ap run.ArgParser) run.RequestManager {
	return &localRunnerManager{
		tc: r.tc,
		ap: ap.(*cc.ArgParser),
	}
}

func (r *CCToolchainCtrl) SendRemote(ap run.ArgParser, client *clients.CompileRequestClient) run.RequestManager {
	return &sendRemoteRunnerManager{
		tc:        r.tc,
		ap:        ap.(*cc.ArgParser),
		reqClient: client,
	}
}

func (r *CCToolchainCtrl) RecvRemote() run.RequestManager {
	return &recvRemoteRunnerManager{
		tc: r.tc,
	}
}

func (r *CCToolchainCtrl) NewArgParser(ctx context.Context, args []string) run.ArgParser {
	return cc.NewArgParser(ctx, args)
}

func AddToStore(store *run.ToolchainRunnerStore) {
	runner := &CCToolchainCtrl{}
	store.Add(types.Gnu, runner)
	store.Add(types.Clang, runner)
}
