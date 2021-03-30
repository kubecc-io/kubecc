/*
Copyright 2021 The Kubecc Authors.

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU General Public License for more details.

You should have received a copy of the GNU General Public License
along with this program.  If not, see <http://www.gnu.org/licenses/>.
*/

package test

import (
	"errors"

	"github.com/google/uuid"
	"github.com/kubecc-io/kubecc/pkg/clients"
	"github.com/kubecc-io/kubecc/pkg/meta"
	"github.com/kubecc-io/kubecc/pkg/run"
	"github.com/kubecc-io/kubecc/pkg/types"
	"github.com/opentracing/opentracing-go"
)

type sendRemoteRunnerManager struct {
	client run.SchedulerClientStream
}

func (m sendRemoteRunnerManager) Process(
	ctx run.Contexts,
	request interface{},
) (response interface{}, err error) {
	lg := meta.Log(ctx.ServerContext)
	tracer := meta.Tracer(ctx.ServerContext)

	lg.Info("Sending remote")
	span, _ := opentracing.StartSpanFromContextWithTracer(
		ctx.ClientContext, tracer, "run-send")
	defer span.Finish()
	req := request.(*types.RunRequest)
	resp, err := m.client.Compile(&types.CompileRequest{
		RequestID: uuid.NewString(),
		Toolchain: req.GetToolchain(),
		Args:      req.Args,
	})
	if err != nil {
		if errors.Is(err, clients.ErrStreamNotReady) {
			return nil, err
		}
		panic(err)
	}
	switch data := resp.Data.(type) {
	case *types.CompileResponse_CompiledSource:
		return &types.RunResponse{
			ReturnCode: 0,
			Stdout:     data.CompiledSource,
		}, nil
	case *types.CompileResponse_Error:
		return &types.RunResponse{
			ReturnCode: 0,
			Stderr:     []byte(data.Error),
		}, nil
	default:
		panic("Invalid test")
	}
}

type sendRemoteRunnerManagerSim struct{}

func (m sendRemoteRunnerManagerSim) Process(
	ctx run.Contexts,
	request interface{},
) (response interface{}, err error) {
	lg := meta.Log(ctx.ServerContext)

	lg.Info("=> Receiving remote")
	req := request.(*types.RunRequest)
	ap := TestArgParser{
		Args: req.Args,
	}
	ap.Parse()
	out, err := doTestAction(&ap)
	if err != nil {
		return &types.RunResponse{
			ReturnCode: 1,
			Stderr:     []byte(err.Error()),
		}, nil
	}
	return &types.RunResponse{
		ReturnCode: 0,
		Stdout:     out,
	}, nil
}
