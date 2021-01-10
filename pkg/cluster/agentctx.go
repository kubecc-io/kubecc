package cluster

import (
	"context"
	"runtime"

	"github.com/cobalt77/kubecc/pkg/types"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type agentInfoKey struct{}

var (
	// AgentInfoKey is a context value key containing AgentInfo data
	AgentInfoKey agentInfoKey
)

// NewAgentContext creates a new cancellable context with
// embedded system info and values from the downward API
func NewAgentContext() (context.Context, context.CancelFunc) {
	return context.WithCancel(
		context.WithValue(context.Background(),
			AgentInfoKey,
			&types.AgentInfo{
				Arch:      runtime.GOARCH,
				NumCpus:   int32(runtime.NumCPU()),
				Node:      GetNode(),
				Pod:       GetPodName(),
				Namespace: GetNamespace(),
			}))
}

func AgentInfoFromContext(ctx context.Context) (*types.AgentInfo, error) {
	agentCtx := ctx.Value(AgentInfoKey)
	if agentCtx == nil {
		return nil, status.Error(codes.InvalidArgument,
			"Context does not contain AgentInfo")
	}
	agentInfo, ok := agentCtx.(*types.AgentInfo)
	if !ok {
		return nil, status.Error(codes.InvalidArgument,
			"Context contains invalid AgentInfo data")
	}
	return agentInfo, nil
}
