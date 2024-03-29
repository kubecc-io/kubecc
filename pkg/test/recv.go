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
	"github.com/kubecc-io/kubecc/pkg/meta"
	"github.com/kubecc-io/kubecc/pkg/run"
	"github.com/kubecc-io/kubecc/pkg/types"
)

type recvRemoteRunnerManager struct{}

func (m recvRemoteRunnerManager) Process(
	ctx run.PairContext,
	request interface{},
) (response interface{}, err error) {
	lg := meta.Log(ctx)

	lg.Info("=> Receiving remote")
	defer lg.Info("=> Done.")
	req := request.(*types.CompileRequest)
	ap := TestArgParser{
		Args: req.Args,
	}
	ap.Parse()
	out, err := doTestAction(ctx, &ap)
	if err != nil {
		return nil, err
	}
	return &types.CompileResponse{
		RequestID:     req.RequestID,
		CompileResult: types.CompileResponse_Success,
		Data: &types.CompileResponse_CompiledSource{
			CompiledSource: out,
		},
	}, nil
}

type recvRemoteRunnerManagerSim struct{}

func (m recvRemoteRunnerManagerSim) Process(
	ctx run.PairContext,
	request interface{},
) (response interface{}, err error) {
	panic("Unimplemented")
}
