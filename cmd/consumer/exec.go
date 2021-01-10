package main

import (
	"context"
	"io"
	"os"
	"os/exec"
	"runtime"
)

type TaskFunc func() error
type Task struct {
	ctx context.Context
	f   TaskFunc
	err error
}

func NewTask(tf TaskFunc, ctx context.Context) *Task {
	return &Task{
		ctx: ctx,
	}
}

func (t *Task) Run() {
	t.err = t.f()
}

func (t *Task) Error() error {
	return t.err
}

type Executor struct {
	taskPool chan *Task
}

type ExecutorOptions struct {
	failFast bool
	name     string
	workDir  string
	args     []string
	env      []string
	stdin    io.Reader
	stdout   io.Writer
	stderr   io.Writer
}

var defaultExecutorOptions = ExecutorOptions{
	failFast: false,
	stdin:    os.Stdin,
	stdout:   os.Stdout,
	stderr:   os.Stderr,
}

type ExecutorOption interface {
	apply(*ExecutorOptions)
}

type funcExecutorOption struct {
	f func(*ExecutorOptions)
}

func (fso *funcExecutorOption) apply(ops *ExecutorOptions) {
	fso.f(ops)
}

// FailFast indicates the executor should immediately
// fail a request if the active contexts are full.
func FailFast(failFast bool) ExecutorOption {
	return &funcExecutorOption{
		f: func(so *ExecutorOptions) {
			so.failFast = failFast
		},
	}
}

func WithCommand(cmd string) ExecutorOption {
	return &funcExecutorOption{
		f: func(so *ExecutorOptions) {
			so.name = cmd
		},
	}
}

func WithArgs(args []string) ExecutorOption {
	return &funcExecutorOption{
		f: func(so *ExecutorOptions) {
			so.args = args
		},
	}
}

func WithEnv(env []string) ExecutorOption {
	return &funcExecutorOption{
		f: func(so *ExecutorOptions) {
			so.env = env
		},
	}
}

func WithWorkDir(dir string) ExecutorOption {
	return &funcExecutorOption{
		f: func(so *ExecutorOptions) {
			so.workDir = dir
		},
	}
}

func WithInputStream(stdin io.Reader) ExecutorOption {
	return &funcExecutorOption{
		f: func(so *ExecutorOptions) {
			so.stdin = stdin
		},
	}
}

func WithOutputStreams(stdout, stderr io.Writer) ExecutorOption {
	return &funcExecutorOption{
		f: func(so *ExecutorOptions) {
			so.stdout = stdout
			so.stderr = stderr
		},
	}
}

type AllThreadsBusy struct {
	error
}

func (e *AllThreadsBusy) Error() string {
	return "all threads are busy"
}

func worker(queue <-chan *Task) {
	for {
		task := <-queue
		if task == nil {
			break
		}
		ch := make(chan struct{})
		go func() {
			task.Run()
			close(ch)
		}()
		select {
		case <-ch:
		case <-task.ctx.Done():
		}
	}
}

func NewExecutor() *Executor {
	s := &Executor{
		taskPool: make(chan *Task, runtime.NumCPU()),
	}
	for i := 0; i < runtime.NumCPU(); i++ {
		go worker(s.taskPool)
	}
	return s
}

func (s *Executor) Exec(
	ctx context.Context,
	opts ...ExecutorOption,
) error {
	options := defaultExecutorOptions
	for _, op := range opts {
		op.apply(&options)
	}

	t := NewTask(func() error {
		c := exec.Command(options.name, options.args...)
		c.Dir = options.workDir
		c.Env = options.env
		c.Stdin = options.stdin
		c.Stdout = options.stdout
		c.Stderr = options.stderr
		return c.Run()
	}, ctx)

	s.taskPool <- t
	return t.Error()
}
