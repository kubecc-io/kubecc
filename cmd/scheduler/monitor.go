package main

import (
	"context"

	"github.com/cobalt77/kubecc/pkg/cluster"
	"github.com/cobalt77/kubecc/pkg/types"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Monitor struct {
	agents map[*types.AgentInfo]*Agent
}

func NewMonitor() *Monitor {
	return &Monitor{
		agents: make(map[*types.AgentInfo]*Agent),
	}
}

func (w *Monitor) AgentIsConnected(info *types.AgentInfo) bool {
	_, ok := w.agents[info]
	return ok
}

func (w *Monitor) AgentConnected(a *Agent) {
	w.agents[a.Info] = a
	go func() {
		<-a.Context.Done()
		delete(w.agents, a.Info)
	}()
}

func (w *Monitor) GetAgentInfo(ctx context.Context) (*types.AgentInfo, error) {
	info, err := cluster.AgentInfoFromContext(ctx)
	if err != nil {
		return nil, err
	}
	if !w.AgentIsConnected(info) {
		return nil, status.Error(codes.FailedPrecondition,
			"Not connected, ensure a connection stream is active with Connect()")
	}
	return info, nil
}

func (mon *Monitor) Wait(task *CompileTask) (*types.CompileResponse, error) {
	for {
		select {
		case s := <-task.Status:
			switch s.CompileStatus {
			case types.CompileStatus_Accept:
				info := s.GetInfo()
				mon.agents[info].ActiveTasks[task.Context] = task.Cancel
			case types.CompileStatus_Reject:
				return nil, status.Error(codes.Aborted, "Agent rejected task")
			case types.CompileStatus_Success:
				return &types.CompileResponse{
					CompileResult: types.CompileResponse_Success,
					Data: &types.CompileResponse_CompiledSource{
						CompiledSource: s.GetCompiledSource(),
					},
				}, nil
			case types.CompileStatus_Fail:
				return &types.CompileResponse{
					CompileResult: types.CompileResponse_Fail,
					Data: &types.CompileResponse_Error{
						Error: s.GetError(),
					},
				}, nil
			}
		case err := <-task.Error:
			return nil, err
		case <-task.Context.Done():
			task.Cancel()
			return nil, status.Error(codes.Canceled, "Consumer canceled the request")
		}
	}
}
