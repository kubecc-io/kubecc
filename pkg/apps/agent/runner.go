package agent

import (
	"github.com/cobalt77/kubecc/pkg/run"
	"github.com/cobalt77/kubecc/pkg/types"
)

var toolchainRunners map[types.ToolchainKind]run.ToolchainRunner

func AddToolchainRunner(kind types.ToolchainKind, runner run.ToolchainRunner) {
	if _, ok := toolchainRunners[kind]; ok {
		panic("Toolchain already added")
	}
	toolchainRunners[kind] = runner
}
