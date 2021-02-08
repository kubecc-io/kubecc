package agent

import (
	"bytes"
	"context"
	"io"
	"os"
	"runtime"

	"github.com/cobalt77/kubecc/internal/logkc"
	"github.com/cobalt77/kubecc/pkg/cc"
	"github.com/cobalt77/kubecc/pkg/run"
	"github.com/cobalt77/kubecc/pkg/types"
	"github.com/opentracing/opentracing-go"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type agentServer struct {
	types.AgentServer
	srvContext context.Context
	lg         *zap.SugaredLogger

	// tasks           *atomic.Int32
	// maxRunningTasks int
	// maxWaitingTasks int
	// runQueue        chan struct{}
	executor *run.Executor
}

func NewAgentServer(ctx context.Context) types.AgentServer {
	srv := &agentServer{
		executor:   run.NewExecutor((runtime.NumCPU() * 3) / 2),
		srvContext: ctx,
		lg:         logkc.LogFromContext(ctx),
	}
	// srv.tasks = atomic.NewInt32(0)
	// srv.maxRunningTasks = 2 * runtime.NumCPU()
	// srv.maxWaitingTasks = 10 * runtime.NumCPU()
	// srv.runQueue = make(chan struct{}, srv.maxRunningTasks)
	// for i := 0; i < cap(srv.runQueue); i++ {
	// 	srv.runQueue <- struct{}{}
	// }
	return srv
}

func (s *agentServer) Compile(
	ctx context.Context,
	req *types.CompileRequest,
) (*types.CompileResponse, error) {
	span, sctx := opentracing.StartSpanFromContext(ctx, "queue")
	defer span.Finish()
	// if s.tasks.Load() >= int32(s.maxRunningTasks+s.maxWaitingTasks) {
	// 	logkc.Error("*** Hit the max number of tasks, rejecting")
	// 	return nil, status.Error(codes.Unavailable, "Max number of concurrent tasks reached")
	// }
	// s.tasks.Inc()
	// token := <-s.runQueue
	// span.Finish()
	// span, _ = opentracing.StartSpanFromContext(ctx, "compile",
	// 	opentracing.FollowsFrom(span.Context()))
	// defer func() {
	// 	span.Finish()
	// 	s.runQueue <- token
	// 	s.tasks.Dec()
	// }()
	ap := cc.NewArgParser(s.srvContext, req.Args)
	ap.Parse()
	s.lg.With(zap.Object("args", ap)).Info("Compile starting")
	stderrBuf := new(bytes.Buffer)
	tmpFilename := new(bytes.Buffer)
	runner := run.NewCompileRunner(
		run.WithContext(logkc.ContextWithLog(ctx, s.lg)),
		run.WithOutputWriter(tmpFilename),
		run.WithOutputStreams(io.Discard, stderrBuf),
		run.WithStdin(bytes.NewReader(req.GetPreprocessedSource())),
	)
	task := run.NewTask(sctx, runner, req.Command, ap)
	err := s.executor.Exec(task)
	s.lg.With(zap.Error(err)).Info("Compile finished")
	if err != nil && run.IsCompilerError(err) {
		return &types.CompileResponse{
			CompileResult: types.CompileResponse_Fail,
			Data: &types.CompileResponse_Error{
				Error: stderrBuf.String(),
			},
		}, nil
	} else if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	data, err := os.ReadFile(tmpFilename.String())
	if err != nil {
		s.lg.With(zap.Error(err)).Info("Error reading temp file")
		return nil, status.Error(codes.Internal, err.Error())
	}
	err = os.Remove(tmpFilename.String())
	if err != nil {
		s.lg.With(zap.Error(err)).Info("Error removing temp file")
	}
	s.lg.With(zap.Error(err)).Info("Sending results")
	return &types.CompileResponse{
		CompileResult: types.CompileResponse_Success,
		Data: &types.CompileResponse_CompiledSource{
			CompiledSource: data,
		},
	}, nil
}

// func (s *agentServer) Compile(
// 	req *types.CompileRequest,
// 	srv types.Agent_CompileServer,
// ) error {
// 	if s.tasks.Load() >= int32(s.maxRunningTasks+s.maxWaitingTasks) {
// 		s.lg.Error("*** Hit the max number of tasks, rejecting")
// 		srv.Send(&types.CompileStatus{
// 			CompileStatus: types.CompileStatus_Reject,
// 			Data: &types.CompileStatus_Error{
// 				Error: "Hit the max number of concurrent tasks",
// 			},
// 		})
// 	}
// 	s.tasks.Inc()
// 	token := <-s.runQueue
// 	defer func() {
// 		s.runQueue <- token
// 		s.tasks.Dec()
// 	}()

// 	s.lg.Info("Compile requested")
// 	srv.Send(&types.CompileStatus{
// 		CompileStatus: types.CompileStatus_Accept,
// 		Data: &types.CompileStatus_Info{
// 			Info: cluster.MakeAgentInfo(),
// 		},
// 	})
// 	info := cc.NewArgsInfo(req.Args, s.lg.Desugar())
// 	info.Parse()
// 	s.lg.With(zap.Object("args", info)).Info("Compile starting")
// 	stderrBuf := new(bytes.Buffer)
// 	tmpFilename := new(bytes.Buffer)
// 	runner := run.NewCompileRunner(
// 		run.WithLogger(s.lg.Desugar()),
// 		run.WithOutputWriter(tmpFilename),
// 		run.WithStderr(stderrBuf),
// 		run.WithStdin(bytes.NewReader(req.GetPreprocessedSource())),
// 	)
// 	err := runner.Run(info)
// 	s.lg.With(zap.Error(err)).Info("Compile finished")
// 	if err != nil && run.IsCompilerError(err) {
// 		srv.Send(
// 			&types.CompileStatus{
// 				CompileStatus: types.CompileStatus_Fail,
// 				Data: &types.CompileStatus_Error{
// 					Error: stderrBuf.String(),
// 				},
// 			})
// 		return nil
// 	} else if err != nil {
// 		return status.Error(codes.Internal, err.Error())
// 	}
// 	data, err := os.ReadFile(tmpFilename.String())
// 	if err != nil {
// 		s.lg.With(zap.Error(err)).Info("Error reading temp file")
// 		return status.Error(codes.Internal, err.Error())
// 	}
// 	err = srv.Send(&types.CompileStatus{
// 		CompileStatus: types.CompileStatus_Success,
// 		Data: &types.CompileStatus_CompiledSource{
// 			CompiledSource: data,
// 		},
// 	})
// 	s.lg.With(zap.Error(err)).Info("Sending results")
// 	return err
// }
