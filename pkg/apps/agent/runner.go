package agent

import (
	"context"

	"github.com/cobalt77/kubecc/pkg/run"
	"github.com/cobalt77/kubecc/pkg/types"
)

type Contexts struct {
	ServerContext context.Context
	ClientContext context.Context
}

type ToolchainRunner interface {
	Run(Contexts, *run.Executor, *types.CompileRequest) (*types.CompileResponse, error)
}

var toolchainRunners map[types.ToolchainKind]ToolchainRunner

func AddToolchainRunner(kind types.ToolchainKind, run ToolchainRunner) {
	if _, ok := toolchainRunners[kind]; ok {
		panic("Toolchain already added")
	}
	toolchainRunners[kind] = run
}
