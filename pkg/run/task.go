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

// Package run contains machinery for working with runnable tasks and objects
// which control tasks.
package run

import (
	"context"
	"errors"
	"io"

	"github.com/kubecc-io/kubecc/pkg/types"
	"github.com/kubecc-io/kubecc/pkg/util"
	"go.uber.org/zap"
)

var (
	ErrUnsupportedTask = errors.New("Task not supported")
)

// Task represents a single runnable action.
type Task interface {
	// Run will run the task. It will block until the task is completed.
	Run()
	// Err will return the task's error value once it has completed.
	// If called before Run() returns, it should panic.
	Err() error
}

// RunAsync will run the given task and return a channel which will
// eventually contain the task's error value, then the channel will be closed.
// Tasks are responsible for their own cancellation. If a task should be
// canceled, it should take the necessary actions and return the error
// context.Canceled.
func RunAsync(t Task) <-chan error {
	ch := make(chan error)
	go func() {
		defer close(ch)
		t.Run()
		ch <- t.Err()
	}()
	return ch
}

// RunWait will run the task, wait for it to complete, then return its error.
func RunWait(t Task) error {
	t.Run()
	return t.Err()
}

// Contexts is a pair of client and server contexts.
type Contexts struct {
	ServerContext context.Context
	ClientContext context.Context
}

// RequestManager represents an entity that is responsible for the entire
// lifecycle of a request (right now either a RunRequest or CompileRequest)
// by creating and running tasks.
type RequestManager interface {
	// Process consumes the request and blocks until it is complete, returning
	// a matching response object and an error. Errors returned from this function
	// do not indicate the request has failed (where "failure" is specific to and
	// defined by the request itself), rather it would indicate that the request
	// could not be completed, either due to a network error, an internal error,
	// or a similar issue. Responses should encode success or failure within
	// the response type itself.
	Process(ctx Contexts, request interface{}) (response interface{}, err error)
}

// PackagedRequest is a runnable closure which can invoke a RequestManager's
// Process method with the provided arguments when desired.
type PackagedRequest struct {
	util.NullableError
	f func() (response interface{}, err error)
	c chan interface{}
}

// Invoke will run the PackagedRequest by calling the packaged RequestManager's
// Process method with the arguments given at the time of its creation.
func (pr *PackagedRequest) Run() {
	response, err := pr.f()
	pr.SetErr(err)
	pr.c <- response
	close(pr.c)
}

func (pr *PackagedRequest) Response() chan interface{} {
	return pr.c
}

// PackageRequest creates a PackagedRequest object and returns it. It does
// not run the request.
func PackageRequest(
	rm RequestManager,
	ctx Contexts,
	request interface{},
) PackagedRequest {
	return PackagedRequest{
		f: func() (response interface{}, err error) {
			return rm.Process(ctx, request)
		},
		c: make(chan interface{}, 1),
	}
}

// ArgParser is a high-level interface that represents an object capable of
// parsing a set of command-line arguments, and determining whether the
// associated request can be run remotely based on the arguments given.
// The concrete object should manage its own data, but should not perform
// any parsing until the Parse function is called. CanRunRemote will always
// be called after Parse.
type ArgParser interface {
	Parse()
	CanRunRemote() bool
}

// Controller represents an object that can control requests for a particular
// toolchain, when provided with a concrete instance of such a toolchain
// with parameters set correctly for the host.
type Controller interface {
	// With transforms the Controller into a ToolchainController by providing
	// it with a concrete toolchain instance.
	With(*types.Toolchain) ToolchainController
}

type SchedulerClientStream interface {
	LoadNewStream(types.Scheduler_StreamOutgoingTasksClient)
	Compile(*types.CompileRequest) (*types.CompileResponse, error)
}

// A ToolchainController is an object capable of managing the entire lifecycle
// of requests for a given toolchain.
type ToolchainController interface {
	// RunLocal returns a RequestManager which can handle a request locally
	// without dispatching any or all of its tasks to a remote agent.
	RunLocal(ArgParser) RequestManager
	// SendRemote returns a RequestManager which will dispatch some or all of
	// its tasks to be processed by a remote agent. This RequestManager is
	// only responsible for local tasks associated with the request (if any),
	// and sending/waiting on tasks using the provided client.
	SendRemote(ArgParser, SchedulerClientStream) RequestManager
	// RecvRemote returns a RequestManager which will run its tasks locally
	// under the assumption that it is running them on behalf of a consumer
	// somewhere else on the network.
	RecvRemote() RequestManager
	// NewArgParser returns a new concrete ArgParser for this toolchain.
	NewArgParser(ctx context.Context, args []string) ArgParser
}

type ProcessOptions struct {
	Stdout  io.Writer
	Stderr  io.Writer
	Stdin   io.Reader
	Env     []string
	Args    []string
	WorkDir string
	UID     uint32
	GID     uint32
}

type ResultOptions struct {
	OutputWriter io.Writer
	OutputVar    interface{}
	NoTempFile   bool
}

type TaskOptions struct {
	ProcessOptions
	ResultOptions

	Context context.Context
	Log     *zap.SugaredLogger
}

func (o *TaskOptions) Apply(opts ...TaskOption) {
	for _, opt := range opts {
		opt(o)
	}
}

type TaskOption func(*TaskOptions)

func WithEnv(env []string) TaskOption {
	return func(ro *TaskOptions) {
		ro.Env = env
	}
}

func WithArgs(args []string) TaskOption {
	return func(ro *TaskOptions) {
		ro.Args = args
	}
}

func WithWorkDir(dir string) TaskOption {
	return func(ro *TaskOptions) {
		ro.WorkDir = dir
	}
}

func WithOutputStreams(stdout, stderr io.Writer) TaskOption {
	return func(ro *TaskOptions) {
		ro.Stdout = stdout
		ro.Stderr = stderr
	}
}

func WithStdin(stdin io.Reader) TaskOption {
	return func(ro *TaskOptions) {
		ro.Stdin = stdin
	}
}

func WithUidGid(uid, gid uint32) TaskOption {
	return func(ro *TaskOptions) {
		ro.UID = uid
		ro.GID = gid
	}
}

func WithOutputWriter(w io.Writer) TaskOption {
	return func(ro *TaskOptions) {
		ro.OutputWriter = w
	}
}

func WithOutputVar(v interface{}) TaskOption {
	return func(ro *TaskOptions) {
		ro.OutputVar = v
	}
}

func InPlace(inPlace bool) TaskOption {
	return func(ro *TaskOptions) {
		ro.NoTempFile = inPlace
	}
}

func WithContext(ctx context.Context) TaskOption {
	return func(ro *TaskOptions) {
		ro.Context = ctx
	}
}

func WithLog(lg *zap.SugaredLogger) TaskOption {
	return func(ro *TaskOptions) {
		ro.Log = lg
	}
}
