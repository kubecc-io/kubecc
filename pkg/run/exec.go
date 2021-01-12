package run

import (
	"runtime"

	"go.uber.org/atomic"
)

type Executor struct {
	taskQueue    chan *Task
	workingCount *atomic.Int32
}

type ExecutorOptions struct {
	failFast bool
}

var (
	defaultExecutorOptions = ExecutorOptions{
		failFast: false,
	}
	cpuCount = runtime.NumCPU()
)

type ExecutorOption interface {
	apply(*ExecutorOptions)
}

type funcExecutorOption struct {
	f func(*ExecutorOptions)
}

func FailFast() ExecutorOption {
	return &funcExecutorOption{
		f: func(eo *ExecutorOptions) {
			eo.failFast = true
		},
	}
}

func (fso *funcExecutorOption) apply(ops *ExecutorOptions) {
	fso.f(ops)
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
		task.Run()
		select {
		case <-task.Done():
		case <-task.ctx.Done():
		}
	}
}

func NewExecutor() *Executor {
	s := &Executor{
		taskQueue:    make(chan *Task),
		workingCount: atomic.NewInt32(0),
	}
	for i := 0; i < cpuCount; i++ {
		go worker(s.taskQueue)
	}
	return s
}

func (s *Executor) Exec(
	task *Task,
	opts ...ExecutorOption,
) error {
	options := defaultExecutorOptions
	for _, op := range opts {
		op.apply(&options)
	}
	if options.failFast && s.AtCapacity() {
		return &AllThreadsBusy{}
	}
	s.workingCount.Inc()
	defer s.workingCount.Dec()
	s.taskQueue <- task
	select {
	case <-task.Done():
	case <-task.ctx.Done():
	}
	return task.Error()
}

func (s *Executor) AtCapacity() bool {
	return s.workingCount.Load() >= int32(cpuCount)-1
}
