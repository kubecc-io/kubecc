package agent

import (
	"github.com/cobalt77/kubecc/pkg/run"
	"github.com/cobalt77/kubecc/pkg/types"
)

var toolchainRunners = make(map[types.ToolchainKind]run.RunnerManager)

// todo move this somewhere else
func AddRunnerManager(kind types.ToolchainKind, runner run.RunnerManager) {
	if _, ok := toolchainRunners[kind]; ok {
		panic("Toolchain already added")
	}
	toolchainRunners[kind] = runner
}
