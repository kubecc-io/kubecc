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
	ctx run.PairContext,
	request interface{},
) (response interface{}, err error) {
	lg := meta.Log(ctx)
	tracer := meta.Tracer(ctx)

	lg.Info("Sending remote")
	span, _ := opentracing.StartSpanFromContextWithTracer(
		ctx, tracer, "run-send")
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
	switch resp.CompileResult {
	case types.CompileResponse_Success:
		return &types.RunResponse{
			ReturnCode: 0,
			Stdout:     resp.GetCompiledSource(),
		}, nil
	case types.CompileResponse_Fail:
		return &types.RunResponse{
			ReturnCode: 0,
			Stderr:     []byte(resp.GetError()),
		}, nil
	case types.CompileResponse_Retry:
		switch resp.GetRetryAction() {
		case types.RetryAction_Retry:
			return nil, run.ErrNoAgentsRetry
		case types.RetryAction_DoNotRetry:
			return nil, run.ErrNoAgentsRunLocal
		}
	case types.CompileResponse_Defunct:
		panic("Invalid test: received compile response \"Defunct\"")
	case types.CompileResponse_InternalError:
		panic("Invalid test: received compile response \"InternalError\"")
	}
	panic("Invalid test: unknown response")
}

type sendRemoteRunnerManagerSim struct{}

func (m sendRemoteRunnerManagerSim) Process(
	ctx run.PairContext,
	request interface{},
) (response interface{}, err error) {
	lg := meta.Log(ctx)

	lg.Info("=> Receiving remote")
	req := request.(*types.RunRequest)
	ap := TestArgParser{
		Args: req.Args,
	}
	ap.Parse()
	out, err := doTestAction(ctx, &ap)
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
