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
	"bytes"
	"context"
	"crypto/md5"
	"flag"
	"time"

	"github.com/kubecc-io/kubecc/pkg/run"
	"github.com/kubecc-io/kubecc/pkg/toolchains"
	"github.com/kubecc-io/kubecc/pkg/types"
	"github.com/kubecc-io/kubecc/pkg/util"
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
	return &TestArgParser{
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
	return &TestArgParser{
		Args: args,
	}
}

func AddToStoreSim(store *run.ToolchainRunnerStore) {
	store.Add(types.TestToolchain, &TestToolchainCtrlLocal{})
}

func doTestAction(ctx context.Context, ap *TestArgParser) ([]byte, error) {
	switch ap.Action {
	case Sleep:
		t := &SleepTask{
			Context:  ctx,
			Duration: ap.Duration,
		}
		t.Run()
		if err := t.Err(); err != nil {
			return nil, err
		}
		return []byte{}, nil
	case Hash:
		out := new(bytes.Buffer)
		t := &HashTask{
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

var TestToolchainExecutable = "/path/to/test-toolchain"

type TestQuerier struct{}

func (q TestQuerier) Version(compiler string) (string, error) {
	return "0", nil
}

func (q TestQuerier) TargetArch(compiler string) (string, error) {
	return "testarch", nil
}

func (q TestQuerier) IsPicDefault(compiler string) (bool, error) {
	return true, nil
}

func (q TestQuerier) Kind(compiler string) (types.ToolchainKind, error) {
	return types.TestToolchain, nil
}

func (q TestQuerier) Lang(compiler string) (types.ToolchainLang, error) {
	return types.ToolchainLang_ToolchainLang_Unknown, nil
}

func (q TestQuerier) ModTime(compiler string) (time.Time, error) {
	return time.Unix(0, 0), nil
}

var DefaultTestToolchain *types.Toolchain

func init() {
	store := toolchains.NewStore()
	DefaultTestToolchain, _ = store.Add(TestToolchainExecutable, TestQuerier{})
}

type TestToolchainFinder struct{}

func (f TestToolchainFinder) FindToolchains(
	ctx context.Context,
	opts ...toolchains.FindOption,
) *toolchains.Store {
	store := toolchains.NewStore()
	_, _ = store.Add(TestToolchainExecutable, TestQuerier{})
	return store
}

type SleepTask struct {
	util.NullableError
	Context  context.Context
	Duration time.Duration
}

func (t *SleepTask) Run() {
	select {
	case <-time.After(t.Duration):
		t.SetErr(nil)
	case <-t.Context.Done():
		t.SetErr(t.Context.Err())
	}
}

// HashTask will compute the md5 sum of the Input string and write the output
// to the OutputWriter specified in the task options.
type HashTask struct {
	run.TaskOptions
	util.NullableError
	Input string
}

func (t *HashTask) Run() {
	h := md5.New()
	h.Write([]byte(t.Input))
	result := h.Sum(nil)
	t.TaskOptions.OutputWriter.Write(result)
	t.SetErr(nil)
}

type NoopRunner struct{}

func (*NoopRunner) Run() error {
	return nil
}

type Action int

const (
	Sleep Action = iota
	Hash
)

type TestArgParser struct {
	Args     []string
	Duration time.Duration
	Input    string
	Action   Action
}

func (ap *TestArgParser) Parse() {
	var sleep string
	var hash string
	set := flag.NewFlagSet("test", flag.PanicOnError)
	set.StringVar(&sleep, "sleep", "", "")
	set.StringVar(&hash, "hash", "", "")
	if err := set.Parse(ap.Args); err != nil {
		panic(err)
	}

	switch {
	case sleep != "":
		d, err := time.ParseDuration(sleep)
		if err != nil {
			panic(err)
		}
		ap.Duration = d
		ap.Action = Sleep
	case hash != "":
		ap.Input = hash
		ap.Action = Hash
	default:
		panic("Invalid arguments to test arg parser")
	}
}

func (TestArgParser) CanRunRemote() bool {
	return true
}

func (t TestArgParser) DeepCopy() run.ArgParser {
	return &t
}

type NoopArgParser struct{}

func (*NoopArgParser) Parse() {}

func (NoopArgParser) CanRunRemote() bool {
	return true
}
