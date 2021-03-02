package run

import (
	"github.com/cobalt77/kubecc/pkg/metrics/status"
	"github.com/cobalt77/kubecc/pkg/types"
	"go.uber.org/atomic"
)

type ExecutorStatus int

type Executor interface {
	status.QueueParamsCompleter
	status.TaskStatusCompleter
	status.QueueStatusCompleter
	Exec(task *Task) error
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
	usageLimits *types.UsageLimits
}
type ExecutorOption func(*ExecutorOptions)

func (o *ExecutorOptions) Apply(opts ...ExecutorOption) {
	for _, op := range opts {
		op(o)
	}
}

func WithUsageLimits(cfg *types.UsageLimits) ExecutorOption {
	return func(o *ExecutorOptions) {
		o.usageLimits = cfg
	}
}

func NewQueuedExecutor(opts ...ExecutorOption) *QueuedExecutor {
	options := ExecutorOptions{}
	options.Apply(opts...)
	if options.usageLimits == nil {
		// options.usageLimits = host.AutoConcurrentProcessLimit()
	}

	queue := make(chan *Task)
	s := &QueuedExecutor{
		ExecutorOptions: options,
		taskQueue:       queue,
		workerPool:      NewWorkerPool(queue),
		numRunning:      atomic.NewInt32(0),
		numQueued:       atomic.NewInt32(0),
	}
	s.workerPool.SetWorkerCount(int(s.usageLimits.ConcurrentProcessLimit))
	return s
}

func NewUnqueuedExecutor() *UnqueuedExecutor {
	return &UnqueuedExecutor{}
}

func (x *QueuedExecutor) SetUsageLimits(cfg *types.UsageLimits) {
	x.usageLimits = cfg
	go x.workerPool.SetWorkerCount(int(cfg.GetConcurrentProcessLimit()))
}

func (x *QueuedExecutor) Exec(
	task *Task,
) error {
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
	case running < x.usageLimits.ConcurrentProcessLimit:
		return types.Available
	case queued < int32(float64(x.usageLimits.ConcurrentProcessLimit)*
		x.usageLimits.QueuePressureMultiplier):
		return types.Queueing
	case queued < int32(float64(x.usageLimits.ConcurrentProcessLimit)*
		x.usageLimits.QueueRejectMultiplier):
		return types.QueuePressure
	}
	return types.QueueFull
}

func (x *QueuedExecutor) CompleteQueueParams(stat *status.QueueParams) {
	stat.ConcurrentProcessLimit = x.usageLimits.ConcurrentProcessLimit
	stat.QueuePressureMultiplier = x.usageLimits.QueuePressureMultiplier
	stat.QueueRejectMultiplier = x.usageLimits.QueueRejectMultiplier
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

func (x *UnqueuedExecutor) Exec(task *Task) error {
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
	stat.ConcurrentProcessLimit = 0
	stat.QueuePressureMultiplier = 0
	stat.QueueRejectMultiplier = 0
}

func (x *UnqueuedExecutor) CompleteTaskStatus(stat *status.TaskStatus) {
	stat.NumQueuedProcesses = 0
	stat.NumRunningProcesses = 0
}

func (x *UnqueuedExecutor) CompleteQueueStatus(stat *status.QueueStatus) {
	stat.QueueStatus = int32(x.Status())
}
