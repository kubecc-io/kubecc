package main

import (
	"context"
	"fmt"
	"io"
	"net"

	"github.com/cobalt77/kubecc/pkg/cc"
	"github.com/cobalt77/kubecc/pkg/cluster"
	"github.com/cobalt77/kubecc/pkg/types"
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

type agentServer struct {
	types.AgentServer
}

var (
	log *zap.Logger
)

func init() {
	conf := zap.Config{
		Level:            zap.NewAtomicLevelAt(zap.DebugLevel),
		Development:      true,
		OutputPaths:      []string{"/tmp/agent.log"},
		ErrorOutputPaths: []string{"/tmp/agent.log"},
	}

	lg, err := conf.Build(zap.AddStacktrace(zap.ErrorLevel))

	if err != nil {
		panic(err)
	}
	log = lg
}

func (s *agentServer) Compile(
	req *types.CompileRequest,
	srv types.Agent_CompileServer,
) error {
	srv.Send(&types.CompileStatus{
		CompileStatus: types.CompileStatus_Accept,
		Data: &types.CompileStatus_Info{
			Info: cluster.MakeAgentInfo(),
		},
	})
	info := cc.NewArgsInfo(req.Command, req.Args, log)
	out, err := cc.Run(info, cc.WithCompressOutput())
	if err != nil && cc.IsCompilerError(err) {
		srv.Send(
			&types.CompileStatus{
				CompileStatus: types.CompileStatus_Fail,
				Data: &types.CompileStatus_Error{
					Error: err.Error(),
				},
			})
	} else if err != nil {
		return err
	}
	srv.Send(&types.CompileStatus{
		CompileStatus: types.CompileStatus_Success,
		Data: &types.CompileStatus_CompiledSource{
			CompiledSource: out,
		},
	})
	return nil
}

func connectToScheduler() context.CancelFunc {
	ctx, cancel := cluster.NewAgentContext()
	go func() {
		cc, err := grpc.Dial(
			fmt.Sprintf("kubecc-scheduler.%s.svc.cluster.local:9090",
				cluster.GetNamespace()),
			grpc.WithInsecure())
		if err != nil {
			log.With(zap.Error(err)).Fatal("Error dialing scheduler")
		}
		client := types.NewSchedulerClient(cc)
		for {
			log.Info("Starting connection to the scheduler")
			stream, err := client.Connect(ctx, grpc.WaitForReady(true))
			if err != nil {
				log.With(zap.Error(err)).Error("Error connecting to scheduler")
			}
			log.Info("Connected to the scheduler")
			for {
				_, err := stream.Recv()
				if err == io.EOF {
					log.Info("EOF received from the scheduler, retrying connection")
					break
				}
			}
		}
	}()
	return cancel
}

func main() {
	srv := grpc.NewServer()
	listener, err := net.Listen("tcp", fmt.Sprintf(":9090"))
	if err != nil {
		log.With(
			zap.Error(err),
		).Fatal("Error listening on socket")
	}
	agent := &agentServer{}
	types.RegisterAgentServer(srv, agent)

	cancel := connectToScheduler()
	defer cancel()

	err = srv.Serve(listener)
	if err != nil {
		log.With(zap.Error(err)).Error("GRPC error")
	}
}
