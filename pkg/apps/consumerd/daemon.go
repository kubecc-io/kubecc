package consumerd

import (
	"context"
	"runtime"
	"strconv"
	"strings"
	"sync"

	"github.com/cobalt77/kubecc/pkg/cc"
	"github.com/cobalt77/kubecc/pkg/run"
	"github.com/cobalt77/kubecc/pkg/servers"
	"github.com/cobalt77/kubecc/pkg/types"
	"github.com/opentracing/opentracing-go"
	"go.uber.org/zap"

	"github.com/cobalt77/kubecc/internal/lll"
	"github.com/cobalt77/kubecc/pkg/tracing"
	"github.com/spf13/viper"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/status"
)

type consumerd struct {
	types.ConsumerdServer

	srvContext context.Context
	lg         *zap.SugaredLogger

	schedulerClient types.SchedulerClient
	connection      *grpc.ClientConn
	executor        *run.Executor
	remoteOnly      bool
	remoteWaitGroup sync.WaitGroup
}

func NewConsumerdServer(ctx context.Context) *consumerd {
	return &consumerd{
		srvContext: ctx,
		lg:         lll.LogFromContext(ctx),
		executor:   run.NewExecutor(runtime.NumCPU()),
		remoteOnly: viper.GetBool("remoteOnly"),
	}
}

func (c *consumerd) schedulerConnected() bool {
	return c.schedulerClient != nil &&
		c.connection.GetState() == connectivity.Ready
}

func (c *consumerd) Run(
	ctx context.Context,
	req *types.RunRequest,
) (*types.RunResponse, error) {
	c.lg.Debug("Running request")
	rootContext := ctx
	for _, env := range req.Env {
		spl := strings.Split(env, "=")
		if len(spl) == 2 && spl[0] == "KUBECC_MAKE_PID" {
			pid, err := strconv.Atoi(spl[1])
			if err != nil {
				c.lg.Debug("Invalid value for KUBECC_MAKE_PID, should be a number")
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

	info := cc.NewArgParser(c.srvContext, req.Args)
	info.Parse()

	mode := info.Mode

	if !c.schedulerConnected() {
		c.lg.Info("Running local, scheduler disconnected")
		mode = cc.RunLocal
	}
	if !c.remoteOnly {
		if !c.executor.AtCapacity() {
			c.lg.Info("Running local, not at capacity yet")
			mode = cc.RunLocal
		}
		// if s.schedulerAtCapacity() {
		// 	lll.Info("Running local, scheduler says it is at capacity")
		// 	mode = cc.RunLocal
		// }
	}

	switch mode {
	case cc.RunLocal:
		return c.runRequestLocal(sctx, req, info, c.executor)
	case cc.RunRemote:
		return c.runRequestRemote(sctx, req, info, c.schedulerClient)
	case cc.Unset:
		return nil, status.Error(codes.Internal, "Encountered RunError state")
	default:
		return nil, status.Error(codes.Internal, "Encountered unknown state")
	}
}

func (s *consumerd) ConnectToRemote() {
	addr := viper.GetString("schedulerAddress")
	if addr == "" {
		s.lg.Debug("Remote compilation unavailable: scheduler address not configured")
		return
	}
	cc, err := servers.Dial(s.srvContext, addr, servers.WithTLS(viper.GetBool("tls")))
	if err != nil {
		s.lg.With(zap.Error(err)).Info("Remote compilation unavailable")
	} else {
		s.connection = cc
		s.schedulerClient = types.NewSchedulerClient(cc)
	}
}

func (s *consumerd) schedulerAtCapacity() bool {
	value, err := s.schedulerClient.AtCapacity(
		context.Background(), &types.Empty{})
	if err != nil {
		s.lg.With(zap.Error(err)).Error("Scheduler error")
		return false
	}
	return value.GetValue()
}
