package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"

	"github.com/cobalt77/kubecc/pkg/cc"
	"github.com/cobalt77/kubecc/pkg/run"
	"github.com/cobalt77/kubecc/pkg/types"
	"go.uber.org/zap"

	"github.com/spf13/viper"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/status"
)

type consumerd struct {
	types.ConsumerdServer

	schedulerClient types.SchedulerClient
	connection      *grpc.ClientConn
	executor        *run.Executor
}

func (c *consumerd) schedulerConnected() bool {
	return c.schedulerClient != nil &&
		c.connection.GetState() == connectivity.Ready
}

func (s *consumerd) Run(
	ctx context.Context,
	req *types.RunRequest,
) (*types.RunResponse, error) {
	log.Debug("Running request")
	if req.UID == 0 || req.GID == 0 {
		return nil, status.Error(codes.InvalidArgument,
			"UID or GID cannot be 0")
	}

	info := cc.NewArgsInfo(req.Args, log)
	info.Parse()

	mode := info.Mode

	if !s.schedulerConnected() {
		log.Info("Running local, scheduler disconnected")
		mode = cc.RunLocal
	}
	if !s.executor.AtCapacity() {
		log.Info("Running local, not at capacity yet")
		mode = cc.RunLocal
	}
	if s.schedulerAtCapacity() {
		log.Info("Running local, scheduler says it is at capacity")
		mode = cc.RunLocal
	}

	switch mode {
	case cc.RunLocal:
		return runRequestLocal(req, info, s.executor)
	case cc.RunRemote:
		return runRequestRemote(ctx, req, info, s.schedulerClient)
	case cc.RunError:
		return nil, status.Error(codes.Internal, "Encountered RunError state")
	default:
		return nil, status.Error(codes.Internal, "Encountered unknown state")
	}
}

func (s *consumerd) connectToRemote() {
	addr := viper.GetString("schedulerAddress")
	if addr == "" {
		log.Debug("Remote compilation unavailable: scheduler address not configured")
		return
	}
	conn, err := grpc.Dial(addr, func() []grpc.DialOption {
		options := []grpc.DialOption{}
		if viper.GetBool("tls") {
			options = append(options, grpc.WithTransportCredentials(credentials.NewTLS(
				&tls.Config{
					InsecureSkipVerify: false,
				},
			)))
		} else {
			log.Warn("** TLS disabled **")
			options = append(options, grpc.WithInsecure())
		}
		return options
	}()...)
	if err != nil {
		log.With(zap.Error(err)).Info("Remote compilation unavailable")
	} else {
		s.connection = conn
		s.schedulerClient = types.NewSchedulerClient(conn)
	}
}

func (s *consumerd) schedulerAtCapacity() bool {
	value, err := s.schedulerClient.AtCapacity(
		context.Background(), &types.Empty{})
	if err != nil {
		log.With(zap.Error(err)).Error("Scheduler error")
		return false
	}
	return value.GetValue()
}

func startConsumerd() {
	agent := &consumerd{
		executor: run.NewExecutor(),
	}
	go agent.connectToRemote()

	port := viper.GetInt("port")
	listener, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		log.With(zap.Error(err)).Fatal("Could not start consumerd")
	}
	srv := grpc.NewServer()

	types.RegisterConsumerdServer(srv, agent)

	err = srv.Serve(listener)
	if err != nil {
		log.Error(err.Error())
	}
}
