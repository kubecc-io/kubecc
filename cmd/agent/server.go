package main

import (
	"bytes"
	"io/ioutil"

	"github.com/cobalt77/kubecc/pkg/cc"
	"github.com/cobalt77/kubecc/pkg/cluster"
	"github.com/cobalt77/kubecc/pkg/run"
	"github.com/cobalt77/kubecc/pkg/types"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type agentServer struct {
	types.AgentServer
}

func (s *agentServer) Compile(
	req *types.CompileRequest,
	srv types.Agent_CompileServer,
) error {
	log.Info("Compile requested")
	srv.Send(&types.CompileStatus{
		CompileStatus: types.CompileStatus_Accept,
		Data: &types.CompileStatus_Info{
			Info: cluster.MakeAgentInfo(),
		},
	})
	info := cc.NewArgsInfo(req.Args, log.Desugar())
	log.With(zap.Object("args", info)).Info("Compile starting")
	stderrBuf := new(bytes.Buffer)
	tmpFilename := new(bytes.Buffer)
	runner := run.NewCompileRunner(
		run.WithLogger(log.Desugar()),
		run.WithOutputWriter(tmpFilename),
		run.WithStderr(stderrBuf),
		run.WithStdin(bytes.NewReader(req.GetPreprocessedSource())),
	)
	err := runner.Run(info)
	log.With(zap.Error(err)).Info("Compile finished")
	if err != nil && run.IsCompilerError(err) {
		srv.Send(
			&types.CompileStatus{
				CompileStatus: types.CompileStatus_Fail,
				Data: &types.CompileStatus_Error{
					Error: stderrBuf.String(),
				},
			})
		return nil
	} else if err != nil {
		return status.Error(codes.Internal, err.Error())
	}
	data, err := ioutil.ReadFile(tmpFilename.String())
	if err != nil {
		log.With(zap.Error(err)).Info("Error reading temp file")
		return status.Error(codes.Internal, err.Error())
	}
	err = srv.Send(&types.CompileStatus{
		CompileStatus: types.CompileStatus_Success,
		Data: &types.CompileStatus_CompiledSource{
			CompiledSource: data,
		},
	})
	log.With(zap.Error(err)).Info("Sending results")
	return err
}
