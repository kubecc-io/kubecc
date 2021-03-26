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
	"context"

	"github.com/cobalt77/kubecc/pkg/run"
	"github.com/cobalt77/kubecc/pkg/types"
)

type SleepToolchainCtrl struct{}

func (r *SleepToolchainCtrl) RunLocal(run.ArgParser) run.RequestManager {
	return &localRunnerManager{}
}

func (r *SleepToolchainCtrl) SendRemote(_ run.ArgParser, client run.SchedulerClientStream) run.RequestManager {
	return &sendRemoteRunnerManager{
		client: client,
	}
}

func (r *SleepToolchainCtrl) RecvRemote() run.RequestManager {
	return &recvRemoteRunnerManager{}
}

func (r *SleepToolchainCtrl) NewArgParser(context.Context, []string) run.ArgParser {
	return &NoopArgParser{}
}

func AddToStore(store *run.ToolchainRunnerStore) {
	store.Add(types.Sleep, &SleepToolchainCtrl{})
}

type NoopArgParser struct{}

func (ap *NoopArgParser) Parse() {}

func (NoopArgParser) CanRunRemote() bool {
	return true
}
