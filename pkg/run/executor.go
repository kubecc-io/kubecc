package run

import (
	"github.com/cobalt77/kubecc/pkg/cpuconfig"
	"github.com/cobalt77/kubecc/pkg/metrics/status"
	"github.com/cobalt77/kubecc/pkg/types"
	"go.uber.org/atomic"
)

type ExecutorStatus int

type Executor interface {
	status.QueueParamsCompleter
	status.TaskStatusCompleter
	status.QueueStatusCompleter
	Exec(task *Task, opts ...ExecutorOption) error
	Status() types.QueueStatus
}

type QueuedExecutor struct {
	ExecutorOptions
	workerPool *WorkerPool
	taskQueue  chan *Task
	numRunning *atomic.Int32
	numQueued  *atomic.Int32
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
	options := ExecutorOptions{}
	options.Apply(opts...)
	if options.cpuConfig == nil {
		options.cpuConfig = cpuconfig.Default()
	}

	queue := make(chan *Task)
	s := &QueuedExecutor{
		ExecutorOptions: options,
		taskQueue:       queue,
		workerPool:      NewWorkerPool(queue),
		numRunning:      atomic.NewInt32(0),
		numQueued:       atomic.NewInt32(0),
	}
	s.workerPool.SetWorkerCount(int(s.cpuConfig.MaxRunningProcesses))
	return s
}

func NewUnqueuedExecutor() *UnqueuedExecutor {
	return &UnqueuedExecutor{}
}

func (x *QueuedExecutor) SetCpuConfig(cfg *types.CpuConfig) {
	x.cpuConfig = cfg
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

	x.numQueued.Inc()
	x.taskQueue <- task
	x.numQueued.Dec()

	x.numRunning.Inc()
	select {
	case <-task.Done():
	case <-task.ctx.Done():
	}
	x.numRunning.Dec()

	return task.Error()
}

func (x *QueuedExecutor) Status() types.QueueStatus {
	queued := x.numQueued.Load()
	running := x.numRunning.Load()

	switch {
	case running < x.cpuConfig.MaxRunningProcesses:
		return types.Available
	case queued < int32(float64(x.cpuConfig.MaxRunningProcesses)*
		x.cpuConfig.QueuePressureThreshold):
		return types.Queueing
	case queued < int32(float64(x.cpuConfig.MaxRunningProcesses)*
		x.cpuConfig.QueueRejectThreshold):
		return types.QueuePressure
	}
	return types.QueueFull
}

func (x *QueuedExecutor) CompleteQueueParams(stat *status.QueueParams) {
	stat.MaxRunningProcesses = x.cpuConfig.MaxRunningProcesses
	stat.QueuePressureThreshold = x.cpuConfig.QueuePressureThreshold
	stat.QueueRejectThreshold = x.cpuConfig.QueueRejectThreshold
}

func (x *QueuedExecutor) CompleteTaskStatus(stat *status.TaskStatus) {
	stat.NumQueuedProcesses = x.numQueued.Load()
	stat.NumRunningProcesses = x.numRunning.Load()
}

func (x *QueuedExecutor) CompleteQueueStatus(stat *status.QueueStatus) {
	stat.QueueStatus = int32(x.Status())
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

func (x *UnqueuedExecutor) CompleteQueueParams(stat *status.QueueParams) {
	stat.MaxRunningProcesses = 0
	stat.QueuePressureThreshold = 0
	stat.QueueRejectThreshold = 0
}

func (x *UnqueuedExecutor) CompleteTaskStatus(stat *status.TaskStatus) {
	stat.NumQueuedProcesses = 0
	stat.NumRunningProcesses = 0
}

func (x *UnqueuedExecutor) CompleteQueueStatus(stat *status.QueueStatus) {
	stat.QueueStatus = int32(x.Status())
}
