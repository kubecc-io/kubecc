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

package controller

import (
	"errors"

	"github.com/cobalt77/kubecc/pkg/clients"
	"github.com/cobalt77/kubecc/pkg/meta"
	"github.com/cobalt77/kubecc/pkg/run"
	"github.com/cobalt77/kubecc/pkg/types"
	"github.com/google/uuid"
	"github.com/opentracing/opentracing-go"
)

type sendRemoteRunnerManager struct {
	client *clients.CompileRequestClient
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

	_, err = m.client.Compile(&types.CompileRequest{
		RequestID: uuid.NewString(),
		Toolchain: req.GetToolchain(),
		Args:      req.Args,
	})
	if err != nil {
		if errors.Is(err, clients.ErrStreamNotReady) {
			return nil, err
		}
		lg.Error(err)
		return &types.RunResponse{
			ReturnCode: 1,
		}, err
	}
	return &types.RunResponse{
		ReturnCode: 0,
	}, nil
}
