package run

import (
	"github.com/cobalt77/kubecc/pkg/cpuconfig"
	"github.com/cobalt77/kubecc/pkg/types"
	"go.uber.org/atomic"
)

type ExecutorStatus int

type Executor interface {
	Exec(task *Task, opts ...ExecutorOption) error
	Status() types.QueueStatus
}

type QueuedExecutor struct {
	ExecutorOptions
	workerPool *WorkerPool
	taskQueue  chan *Task
	numRunning *atomic.Int32
}

type ExecutorOptions struct {
	cpuConfig *types.CpuConfig
}
type ExecutorOption func(*ExecutorOptions)

func (o *ExecutorOptions) Apply(opts ...ExecutorOption) {
	for _, op := range opts {
		op(o)
	}
}

func WithCpuConfig(cfg *types.CpuConfig) ExecutorOption {
	return func(o *ExecutorOptions) {
		o.cpuConfig = cfg
	}
}

func NewQueuedExecutor(opts ...ExecutorOption) *QueuedExecutor {
	options := ExecutorOptions{
		cpuConfig: cpuconfig.Default(),
	}
	options.Apply(opts...)
	queue := make(chan *Task)
	s := &QueuedExecutor{
		ExecutorOptions: options,
		taskQueue:       queue,
		workerPool:      NewWorkerPool(queue),
		numRunning:      atomic.NewInt32(0),
	}
	s.workerPool.SetWorkerCount(int(s.cpuConfig.MaxRunningProcesses))
	return s
}

func NewUnqueuedExecutor() *UnqueuedExecutor {
	return &UnqueuedExecutor{}
}

func (x *QueuedExecutor) SetCpuConfig(cfg *types.CpuConfig) {
	go x.workerPool.SetWorkerCount(int(cfg.GetMaxRunningProcesses()))
}

func (x *QueuedExecutor) Exec(
	task *Task,
	opts ...ExecutorOption,
) error {
	options := ExecutorOptions{}
	for _, op := range opts {
		op(&options)
	}

	x.taskQueue <- task
	x.numRunning.Inc()
	select {
	case <-task.Done():
	case <-task.ctx.Done():
	}
	x.numRunning.Dec()
	return task.Error()
}

func (x *QueuedExecutor) Status() types.QueueStatus {
	queueLen := len(x.taskQueue)
	switch {
	case x.numRunning.Load() < x.cpuConfig.MaxRunningProcesses:
		return types.Available
	case queueLen < int(float64(x.cpuConfig.MaxRunningProcesses)*
		x.cpuConfig.QueuePressureThreshold):
		return types.Queueing
	case queueLen < int(float64(x.cpuConfig.MaxRunningProcesses)*
		x.cpuConfig.QueueRejectThreshold):
		return types.QueuePressure
	}
	return types.QueueFull
}

// UnqueuedExecutor is an executor that does not run a worker pool,
// runs all tasks as soon as possible, and is always available.
type UnqueuedExecutor struct{}

func (x *UnqueuedExecutor) Exec(task *Task, opts ...ExecutorOption) error {
	go task.Run()
	select {
	case <-task.Done():
	case <-task.ctx.Done():
	}
	return task.Error()
}

func (x *UnqueuedExecutor) Status() types.QueueStatus {
	return types.Available
}
