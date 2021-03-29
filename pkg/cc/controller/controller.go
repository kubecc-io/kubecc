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

package toolchain

import (
	"context"

	"github.com/kubecc-io/kubecc/pkg/cc"
	"github.com/kubecc-io/kubecc/pkg/run"
	"github.com/kubecc-io/kubecc/pkg/types"
)

type CCToolchainCtrl struct {
	tc *types.Toolchain
}

func (r *CCToolchainCtrl) With(tc *types.Toolchain) {
	r.tc = tc
}

func (r *CCToolchainCtrl) RunLocal(ap run.ArgParser) run.RequestManager {
	return &localRunnerManager{
		tc: r.tc,
		ap: ap.(*cc.ArgParser),
	}
}

func (r *CCToolchainCtrl) SendRemote(ap run.ArgParser, client run.SchedulerClientStream) run.RequestManager {
	return &sendRemoteRunnerManager{
		tc:        r.tc,
		ap:        ap.(*cc.ArgParser),
		reqClient: client,
	}
}

func (r *CCToolchainCtrl) RecvRemote() run.RequestManager {
	return &recvRemoteRunnerManager{
		tc: r.tc,
	}
}

func (r *CCToolchainCtrl) NewArgParser(ctx context.Context, args []string) run.ArgParser {
	return cc.NewArgParser(ctx, args)
}

func AddToStore(store *run.ToolchainRunnerStore) {
	runner := &CCToolchainCtrl{}
	store.Add(types.Gnu, runner)
	store.Add(types.Clang, runner)
}
