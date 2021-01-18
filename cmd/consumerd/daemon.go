package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"runtime"
	"strconv"
	"strings"
	"sync"

	"github.com/cobalt77/kubecc/pkg/cc"
	"github.com/cobalt77/kubecc/pkg/run"
	"github.com/cobalt77/kubecc/pkg/types"
	"github.com/opentracing/opentracing-go"
	"go.uber.org/zap"

	"github.com/cobalt77/kubecc/internal/lll"
	"github.com/cobalt77/kubecc/pkg/tracing"
	"github.com/grpc-ecosystem/grpc-opentracing/go/otgrpc"
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

	remoteOnly      bool
	remoteWaitGroup sync.WaitGroup
}

func (c *consumerd) schedulerConnected() bool {
	return c.schedulerClient != nil &&
		c.connection.GetState() == connectivity.Ready
}

func (s *consumerd) Run(
	ctx context.Context,
	req *types.RunRequest,
) (*types.RunResponse, error) {
	lll.Debug("Running request")
	rootContext := ctx
	for _, env := range req.Env {
		spl := strings.Split(env, "=")
		if len(spl) == 2 && spl[0] == "KUBECC_MAKE_PID" {
			pid, err := strconv.Atoi(spl[1])
			if err != nil {
				lll.Debug("Invalid value for KUBECC_MAKE_PID, should be a number")
				break
			}
			rootContext = tracing.PIDSpanContext(pid)
		}
	}
	span, sctx := opentracing.StartSpanFromContext(rootContext, "run")
	defer span.Finish()

	if req.UID == 0 || req.GID == 0 {
		return nil, status.Error(codes.InvalidArgument,
			"UID or GID cannot be 0")
	}

	info := cc.NewArgsInfo(req.Args)
	info.Parse()

	mode := info.Mode

	if !s.schedulerConnected() {
		lll.Info("Running local, scheduler disconnected")
		mode = cc.RunLocal
	}
	if !s.remoteOnly {
		if !s.executor.AtCapacity() {
			lll.Info("Running local, not at capacity yet")
			mode = cc.RunLocal
		}
		// if s.schedulerAtCapacity() {
		// 	lll.Info("Running local, scheduler says it is at capacity")
		// 	mode = cc.RunLocal
		// }
	}

	switch mode {
	case cc.RunLocal:
		return runRequestLocal(sctx, req, info, s.executor)
	case cc.RunRemote:
		return runRequestRemote(sctx, req, info, s.schedulerClient)
	case cc.Unset:
		return nil, status.Error(codes.Internal, "Encountered RunError state")
	default:
		return nil, status.Error(codes.Internal, "Encountered unknown state")
	}
}

func (s *consumerd) connectToRemote() {
	addr := viper.GetString("schedulerAddress")
	if addr == "" {
		lll.Debug("Remote compilation unavailable: scheduler address not configured")
		return
	}
	conn, err := grpc.Dial(addr, func() []grpc.DialOption {
		options := []grpc.DialOption{
			grpc.WithUnaryInterceptor(
				otgrpc.OpenTracingClientInterceptor(opentracing.GlobalTracer())),
			grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(1e8)), // 100 MB
		}
		if viper.GetBool("tls") {
			options = append(options, grpc.WithTransportCredentials(credentials.NewTLS(
				&tls.Config{
					InsecureSkipVerify: false,
				},
			)))
		} else {
			lll.Warn("** TLS disabled **")
			options = append(options, grpc.WithInsecure())
		}
		return options
	}()...)
	if err != nil {
		lll.With(zap.Error(err)).Info("Remote compilation unavailable")
	} else {
		s.connection = conn
		s.schedulerClient = types.NewSchedulerClient(conn)
	}
}

func (s *consumerd) schedulerAtCapacity() bool {
	value, err := s.schedulerClient.AtCapacity(
		context.Background(), &types.Empty{})
	if err != nil {
		lll.With(zap.Error(err)).Error("Scheduler error")
		return false
	}
	return value.GetValue()
}

func startConsumerd() {
	lll.Info("Starting consumerd")
	d := &consumerd{
		executor: run.NewExecutor(runtime.NumCPU()),
	}
	go d.connectToRemote()
	d.remoteOnly = viper.GetBool("remoteOnly")
	port := viper.GetInt("port")
	listener, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		lll.With(zap.Error(err), zap.Int("port", port)).
			Fatal("Error listening on socket")
	}
	lll.With("addr", listener.Addr()).Info("Listening")
	if err != nil {
		lll.With(zap.Error(err)).Fatal("Could not start consumerd")
	}
	srv := grpc.NewServer(
		grpc.MaxRecvMsgSize(1e8), // 100MB
		grpc.UnaryInterceptor(
			otgrpc.OpenTracingServerInterceptor(opentracing.GlobalTracer())),
	)

	types.RegisterConsumerdServer(srv, d)

	err = srv.Serve(listener)
	if err != nil {
		lll.Error(err.Error())
	}
}
