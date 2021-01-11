package main

import (
	"bytes"

	"github.com/cobalt77/kubecc/pkg/cc"
	"github.com/cobalt77/kubecc/pkg/cluster"
	"github.com/cobalt77/kubecc/pkg/run"
	"github.com/cobalt77/kubecc/pkg/types"
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
	srv.Send(&types.CompileStatus{
		CompileStatus: types.CompileStatus_Accept,
		Data: &types.CompileStatus_Info{
			Info: cluster.MakeAgentInfo(),
		},
	})
	info := cc.NewArgsInfo(append([]string{req.Command}, req.Args...), log)
	stderrBuf := new(bytes.Buffer)
	tmpFilename := new(bytes.Buffer)
	runner := run.NewCompileRunner(
		run.WithLogger(log),
		run.WithOutputWriter(tmpFilename),
		run.WithStderr(stderrBuf),
	)
	err := runner.Run(info)
	if err != nil && run.IsCompilerError(err) {
		srv.Send(
			&types.CompileStatus{
				CompileStatus: types.CompileStatus_Fail,
				Data: &types.CompileStatus_Error{
					Error: err.Error(),
				},
			})
	} else if err != nil {
		return status.Error(codes.Internal, err.Error())
	}
	data, err := readAndCompressFile(tmpFilename.String())
	if err != nil {
		return status.Error(codes.Internal, err.Error())
	}
	return srv.Send(&types.CompileStatus{
		CompileStatus: types.CompileStatus_Success,
		Data: &types.CompileStatus_CompiledSource{
			CompiledSource: data,
		},
	})
}
