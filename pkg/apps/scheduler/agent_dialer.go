package scheduler

import (
	"fmt"
	"net"

	"github.com/cobalt77/kubecc/pkg/meta"
	"github.com/cobalt77/kubecc/pkg/servers"
	"github.com/cobalt77/kubecc/pkg/types"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
)

type AgentDialer interface {
	Dial(ctx meta.Context) (types.AgentClient, error)
}

type tcpDialer struct{}

func (d *tcpDialer) Dial(ctx meta.Context) (types.AgentClient, error) {
	peer, ok := peer.FromContext(ctx)
	if !ok {
		return nil, status.Error(codes.Internal,
			"Error identifying agent peer")
	}

	cc, err := servers.Dial(ctx,
		fmt.Sprintf("%s:9090", peer.Addr.(*net.TCPAddr).IP.String()))
	if err != nil {
		if !ok {
			return nil, status.Error(codes.Internal,
				"Error establishing connection to agent's server")
		}
	}
	return types.NewAgentClient(cc), nil
}
