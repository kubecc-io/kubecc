package cluster

import (
	"context"
	"encoding/json"
	"runtime"

	"github.com/cobalt77/kubecc/pkg/types"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

var (
	// AgentInfoKey is a context value key containing AgentInfo data
	AgentInfoKey string = "kubecc_agent_info"
)

func MakeAgentInfo() *types.AgentInfo {
	return &types.AgentInfo{
		Arch:      runtime.GOARCH,
		NumCpus:   int32(runtime.NumCPU()),
		Node:      GetNode(),
		Pod:       GetPodName(),
		Namespace: GetNamespace(),
	}
}

// NewAgentContext creates a new cancellable context with
// embedded system info and values from the downward API
func NewAgentContext() context.Context {
	json, err := json.Marshal(MakeAgentInfo())
	if err != nil {
		panic(err)
	}
	return metadata.NewOutgoingContext(
		context.Background(), metadata.Pairs(
			AgentInfoKey, string(json),
		))
}

func AgentInfoFromContext(ctx context.Context) (info *types.AgentInfo, err error) {
	info = &types.AgentInfo{}
	err = status.Error(codes.InvalidArgument, "Context does not contain AgentInfo")
	meta, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return
	}
	values := meta.Get(AgentInfoKey)
	if len(values) != 1 {
		return
	}
	err = json.Unmarshal([]byte(values[0]), info)
	return
}
