package agent

import (
	"context"
	"fmt"
	"net"

	"github.com/cobalt77/kube-distcc/cc"
	types "github.com/cobalt77/kube-distcc/types"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"google.golang.org/grpc"
)

type remoteAgentServer struct {
	types.RemoteAgentServer
}

func (s *remoteAgentServer) Compile(
	ctx context.Context,
	req *types.CompileRequest,
) (*types.CompileResponse, error) {
	info := cc.NewArgsInfo(req.Command, req.Args)
	out, err := cc.Run(info, cc.WithCompressOutput())
	if err != nil {
		return nil, err
	}
	return &types.CompileResponse{
		CompiledSource: out,
	}, nil
}

func StartRemoteAgent() {
	srv := grpc.NewServer()
	port := viper.GetInt("agentPort")
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		log.Fatal(err)
	}
	agent := &localAgentServer{}
	types.RegisterLocalAgentServer(srv, agent)

	err = srv.Serve(listener)
	if err != nil {
		log.Error(err)
	}
}
