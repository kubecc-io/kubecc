package agent

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

type Scheduler struct {
	taskPool chan *Task
}

type schedulerOptions struct {
	failFast bool
	name     string
	workDir  string
	args     []string
	env      []string
	stdin    io.Reader
	stdout   io.Writer
	stderr   io.Writer
}

var defaultSchedulerOptions = schedulerOptions{
	failFast: false,
	stdin:    os.Stdin,
	stdout:   os.Stdout,
	stderr:   os.Stderr,
}

type SchedulerOption interface {
	apply(*schedulerOptions)
}

type funcSchedulerOption struct {
	f func(*schedulerOptions)
}

func (fso *funcSchedulerOption) apply(ops *schedulerOptions) {
	fso.f(ops)
}

// FailFast indicates the scheduler should immediately
// fail a request if the active contexts are full.
func FailFast(failFast bool) SchedulerOption {
	return &funcSchedulerOption{
		f: func(so *schedulerOptions) {
			so.failFast = failFast
		},
	}
}

func WithCommand(cmd string) SchedulerOption {
	return &funcSchedulerOption{
		f: func(so *schedulerOptions) {
			so.name = cmd
		},
	}
}

func WithArgs(args []string) SchedulerOption {
	return &funcSchedulerOption{
		f: func(so *schedulerOptions) {
			so.args = args
		},
	}
}

func WithEnv(env []string) SchedulerOption {
	return &funcSchedulerOption{
		f: func(so *schedulerOptions) {
			so.env = env
		},
	}
}

func WithWorkDir(dir string) SchedulerOption {
	return &funcSchedulerOption{
		f: func(so *schedulerOptions) {
			so.workDir = dir
		},
	}
}

func WithInputStream(stdin io.Reader) SchedulerOption {
	return &funcSchedulerOption{
		f: func(so *schedulerOptions) {
			so.stdin = stdin
		},
	}
}

func WithOutputStreams(stdout, stderr io.Writer) SchedulerOption {
	return &funcSchedulerOption{
		f: func(so *schedulerOptions) {
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

func NewScheduler() *Scheduler {
	s := &Scheduler{
		taskPool: make(chan *Task, runtime.NumCPU()),
	}
	for i := 0; i < runtime.NumCPU(); i++ {
		go worker(s.taskPool)
	}
	return s
}

func (s *Scheduler) Run(
	ctx context.Context,
	opts ...SchedulerOption,
) error {
	options := defaultSchedulerOptions
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
