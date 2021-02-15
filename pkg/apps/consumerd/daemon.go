package consumerd

import (
	"context"
	"io/fs"
	"runtime"
	"strconv"
	"strings"
	"sync"

	"github.com/cobalt77/kubecc/pkg/cc"
	"github.com/cobalt77/kubecc/pkg/run"
	"github.com/cobalt77/kubecc/pkg/servers"
	"github.com/cobalt77/kubecc/pkg/toolchains"
	"github.com/cobalt77/kubecc/pkg/types"
	"github.com/opentracing/opentracing-go"
	"github.com/pkg/errors"
	"go.uber.org/zap"

	"github.com/cobalt77/kubecc/internal/logkc"
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

	toolchains      *toolchains.Store
	schedulerClient types.SchedulerClient
	connection      *grpc.ClientConn
	executor        *run.Executor
	remoteOnly      bool
	remoteWaitGroup sync.WaitGroup
}

func NewConsumerdServer(ctx context.Context) *consumerd {
	return &consumerd{
		srvContext: ctx,
		lg:         logkc.LogFromContext(ctx),
		toolchains: toolchains.FindToolchains(
			ctx, toolchains.SearchPathEnv(false)),
		executor:   run.NewExecutor(runtime.NumCPU()),
		remoteOnly: viper.GetBool("remoteOnly"),
	}
}

func (c *consumerd) schedulerConnected() bool {
	return c.schedulerClient != nil &&
		c.connection.GetState() == connectivity.Ready
}

func (c *consumerd) setToolchain(req *types.RunRequest) error {
	path := req.GetPath()
	if path == "" {
		return status.Error(codes.InvalidArgument, "No compiler path given")
	}
	tc, err := c.toolchains.Find(path)
	if err != nil {
		// Add a new toolchain
		c.lg.Info("Consumer sent unknown toolchain; attempting to add it")
		tc, err = c.toolchains.Add(path, toolchains.ExecQuerier{})
		if err != nil {
			c.lg.With(zap.Error(err)).Error("Could not add toolchain")
			return status.Error(codes.InvalidArgument,
				errors.WithMessage(err, "Could not add toolchain").Error())
		}
		c.lg.With("compiler", tc.Executable).Info("New toolchain added")
	} else {
		// Check if the found toolchain is up to date
		if err := c.toolchains.UpdateIfNeeded(tc); err != nil {
			// The toolchain was updated and is no longer valid
			c.lg.With(
				"compiler", tc.Executable,
				zap.Error(err),
			).Error("Error when updating toolchain")
			if _, is := err.(*fs.PathError); is {
				return status.Error(codes.NotFound,
					errors.WithMessage(err, "Compiler no longer exists").Error())
			}
			return status.Error(codes.InvalidArgument,
				errors.WithMessage(err, "Toolchain is no longer valid").Error())
		}
	}
	req.Compiler = &types.RunRequest_Toolchain{
		Toolchain: tc,
	}
	return nil
}

func (c *consumerd) Run(
	ctx context.Context,
	req *types.RunRequest,
) (*types.RunResponse, error) {
	c.lg.Debug("Running request")
	err := c.setToolchain(req)
	if err != nil {
		return nil, err
	}

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
		// 	logkc.Info("Running local, scheduler says it is at capacity")
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
