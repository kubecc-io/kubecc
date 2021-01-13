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
}

var (
	defaultExecutorOptions = ExecutorOptions{}
	cpuCount               = runtime.NumCPU()
)

type ExecutorOption interface {
	apply(*ExecutorOptions)
}

type funcExecutorOption struct {
	f func(*ExecutorOptions)
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

func worker(queue <-chan *Task, count *atomic.Int32) {
	for {
		task := <-queue
		if task == nil {
			break
		}
		count.Inc()
		task.Run()
		select {
		case <-task.Done():
		case <-task.ctx.Done():
		}
		count.Dec()
	}
}

func NewExecutor() *Executor {
	s := &Executor{
		taskQueue:    make(chan *Task),
		workingCount: atomic.NewInt32(0),
	}
	for i := 0; i < cpuCount/2; i++ {
		go worker(s.taskQueue, s.workingCount)
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
	s.taskQueue <- task
	select {
	case <-task.Done():
	case <-task.ctx.Done():
	}
	return task.Error()
}

func (s *Executor) AtCapacity() bool {
	return s.workingCount.Load() >= int32(cpuCount/2)
}
