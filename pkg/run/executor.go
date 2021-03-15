package run

import (
	"github.com/cobalt77/kubecc/pkg/host"
	"github.com/cobalt77/kubecc/pkg/metrics"
	"go.uber.org/atomic"
)

type ExecutorStatus int

type Executor interface {
	metrics.UsageLimitsCompleter
	metrics.TaskStatusCompleter
	Exec(task *Task) error
}

type QueuedExecutor struct {
	ExecutorOptions
	workerPool *WorkerPool
	taskQueue  chan *Task
	numRunning *atomic.Int32
	numQueued  *atomic.Int32
}

type ExecutorOptions struct {
	usageLimits *metrics.UsageLimits
}
type ExecutorOption func(*ExecutorOptions)

func (o *ExecutorOptions) Apply(opts ...ExecutorOption) {
	for _, op := range opts {
		op(o)
	}
}

func WithUsageLimits(cfg *metrics.UsageLimits) ExecutorOption {
	return func(o *ExecutorOptions) {
		o.usageLimits = cfg
	}
}

func NewQueuedExecutor(opts ...ExecutorOption) *QueuedExecutor {
	options := ExecutorOptions{}
	options.Apply(opts...)

	if options.usageLimits == nil {
		options.usageLimits = &metrics.UsageLimits{
			ConcurrentProcessLimit:  host.AutoConcurrentProcessLimit(),
			QueuePressureMultiplier: 1,
			QueueRejectMultiplier:   1,
		}
	} else if options.usageLimits.ConcurrentProcessLimit == -1 {
		options.usageLimits.ConcurrentProcessLimit =
			host.AutoConcurrentProcessLimit()
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

func NewDelegatingExecutor() *DelegatingExecutor {
	return &DelegatingExecutor{
		numTasks: atomic.NewInt32(0),
	}
}

func (x *QueuedExecutor) SetUsageLimits(cfg *metrics.UsageLimits) {
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

func (x *QueuedExecutor) CompleteUsageLimits(stat *metrics.UsageLimits) {
	stat.ConcurrentProcessLimit = x.usageLimits.ConcurrentProcessLimit
	stat.QueuePressureMultiplier = x.usageLimits.QueuePressureMultiplier
	stat.QueueRejectMultiplier = x.usageLimits.QueueRejectMultiplier
}

func (x *QueuedExecutor) CompleteTaskStatus(stat *metrics.TaskStatus) {
	stat.NumQueued = x.numQueued.Load()
	stat.NumRunning = x.numRunning.Load()
}

// DelegatingExecutor is an executor that does not run a worker pool,
// runs all tasks as soon as possible, and is always available.
// It will report that all of its tasks are Delegated, and will not report
// counts for queued or running tasks.
type DelegatingExecutor struct {
	numTasks *atomic.Int32
}

func (x *DelegatingExecutor) Exec(task *Task) error {
	x.numTasks.Inc()
	defer x.numTasks.Dec()

	go task.Run()
	select {
	case <-task.Done():
	case <-task.ctx.Done():
	}
	return task.Error()
}

func (x *DelegatingExecutor) CompleteUsageLimits(stat *metrics.UsageLimits) {}

func (x *DelegatingExecutor) CompleteTaskStatus(stat *metrics.TaskStatus) {
	stat.NumDelegated = x.numTasks.Load()
}
