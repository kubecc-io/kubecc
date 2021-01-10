package main

import (
	"context"
	"fmt"

	"github.com/cobalt77/kubecc/pkg/cluster"
	"github.com/cobalt77/kubecc/pkg/types"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type AgentWatcher struct {
	agents map[*types.AgentInfo]*Agent
}

func NewAgentWatcher() *AgentWatcher {
	return &AgentWatcher{
		agents: make(map[*types.AgentInfo]*Agent),
	}
}

func (w *AgentWatcher) AgentIsConnected(info *types.AgentInfo) bool {
	_, ok := w.agents[info]
	return ok
}

func (w *AgentWatcher) WatchAgent(a *Agent) {
	w.agents[a.Info] = a
	go func() {
		<-a.Context.Done()
		delete(w.agents, a.Info)
	}()
}

func (w *AgentWatcher) GetAgentInfo(ctx context.Context) (*types.AgentInfo, error) {
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

func (w *AgentWatcher) Wait(
	info *types.AgentInfo,
	task *CompileTask,
) (*types.CompileResponse, error) {
	agent, ok := w.agents[info]
	if !ok {
		return nil, fmt.Errorf("no such agent: %s", info)
	}
	for {
		select {
		case s := <-task.Status:
			switch s.CompileStatus {
			case types.CompileStatus_Starting:
				agent.ActiveTasks[task.Context] = task.Cancel
			case types.CompileStatus_Done:
				return &types.CompileResponse{
					CompiledSource: s.CompiledSource,
				}, nil
			}
		case err := <-task.Error:
			return nil, err
		case <-agent.Context.Done():
			task.Cancel()
			return nil, status.Error(codes.Canceled, "Agent canceled the request")
		}
	}
}
