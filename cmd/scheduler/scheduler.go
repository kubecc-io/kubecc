package main

import (
	"github.com/cobalt77/kubecc/pkg/types"
)

type AgentScheduler interface {
	Schedule(*types.CompileRequest) (*CompileTask, error)
}
