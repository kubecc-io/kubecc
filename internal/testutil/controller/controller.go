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
	"bytes"
	"context"

	"github.com/cobalt77/kubecc/internal/testutil"
	"github.com/cobalt77/kubecc/pkg/run"
	"github.com/cobalt77/kubecc/pkg/types"
)

type TestToolchainCtrl struct{}

func (r *TestToolchainCtrl) RunLocal(run.ArgParser) run.RequestManager {
	return &localRunnerManager{}
}

func (r *TestToolchainCtrl) SendRemote(_ run.ArgParser, client run.SchedulerClientStream) run.RequestManager {
	return &sendRemoteRunnerManager{
		client: client,
	}
}

func (r *TestToolchainCtrl) RecvRemote() run.RequestManager {
	return &recvRemoteRunnerManager{}
}

func (r *TestToolchainCtrl) NewArgParser(_ context.Context, args []string) run.ArgParser {
	return &testutil.TestArgParser{
		Args: args,
	}
}

func AddToStore(store *run.ToolchainRunnerStore) {
	store.Add(types.TestToolchain, &TestToolchainCtrl{})
}

// TestToolchainCtrlLocal is similar to TestToolchainRunner, except that it
// will simulate a remote connection. All tasks are run locally but separate
// executors can be used to measure local and remote usage.
type TestToolchainCtrlLocal struct{}

func (r *TestToolchainCtrlLocal) RunLocal(run.ArgParser) run.RequestManager {
	return &localRunnerManager{}
}

func (r *TestToolchainCtrlLocal) SendRemote(run.ArgParser, run.SchedulerClientStream) run.RequestManager {
	return &sendRemoteRunnerManagerSim{}
}

func (r *TestToolchainCtrlLocal) RecvRemote() run.RequestManager {
	return &recvRemoteRunnerManagerSim{}
}

func (r *TestToolchainCtrlLocal) NewArgParser(_ context.Context, args []string) run.ArgParser {
	return &testutil.TestArgParser{
		Args: args,
	}
}

func AddToStoreSim(store *run.ToolchainRunnerStore) {
	store.Add(types.TestToolchain, &TestToolchainCtrlLocal{})
}

func doTestAction(ap *testutil.TestArgParser) ([]byte, error) {
	switch ap.Action {
	case testutil.Sleep:
		t := &testutil.SleepTask{
			Duration: ap.Duration,
		}
		t.Run()
		if err := t.Err(); err != nil {
			return nil, err
		}
		return []byte{}, nil
	case testutil.Hash:
		out := new(bytes.Buffer)
		t := &testutil.HashTask{
			TaskOptions: run.TaskOptions{
				ResultOptions: run.ResultOptions{
					OutputWriter: out,
				},
			},
			Input: ap.Input,
		}
		t.Run()
		if err := t.Err(); err != nil {
			return nil, err
		}
		return out.Bytes(), nil
	default:
		panic("Invalid action")
	}
}
