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
	"github.com/kubecc-io/kubecc/pkg/meta"
	"github.com/kubecc-io/kubecc/pkg/run"
	"github.com/kubecc-io/kubecc/pkg/types"
)

type recvRemoteRunnerManager struct {
}

func (m recvRemoteRunnerManager) Process(
	ctx run.Contexts,
	request interface{},
) (response interface{}, err error) {
	lg := meta.Log(ctx.ServerContext)

	lg.Info("=> Receiving remote")
	req := request.(*types.CompileRequest)
	t := run.NewExecCommandTask(req.GetToolchain(),
		run.WithArgs(req.Args),
		run.WithContext(ctx.ClientContext),
	)
	t.Run()
	if err := t.Err(); err != nil {
		lg.Error(err)
		return &types.CompileResponse{
			RequestID:     req.RequestID,
			CompileResult: types.CompileResponse_Fail,
			Data: &types.CompileResponse_Error{
				Error: err.Error(),
			},
		}, err
	}
	lg.Info("=> Done.")
	return &types.CompileResponse{
		RequestID:     req.RequestID,
		CompileResult: types.CompileResponse_Success,
		Data: &types.CompileResponse_CompiledSource{
			CompiledSource: []byte{},
		},
	}, nil
}
