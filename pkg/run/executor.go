package run

import (
	"context"

	"github.com/cobalt77/kubecc/pkg/host"
	"github.com/cobalt77/kubecc/pkg/metrics"
	"go.uber.org/atomic"
)

type ExecutorStatus int

type Executor interface {
	metrics.UsageLimitsCompleter
	metrics.TaskStatusCompleter
	Exec(task Task) error
}

type QueuedExecutor struct {
	ExecutorOptions
	workerPool *WorkerPool
	taskQueue  chan Task
	numRunning *atomic.Int32
	numQueued  *atomic.Int32
}

type ExecutorOptions struct {
	UsageLimits *metrics.UsageLimits
}
type ExecutorOption func(*ExecutorOptions)

type contextTask struct {
	Task
	ctx context.Context
}

func (o *ExecutorOptions) Apply(opts ...ExecutorOption) {
	for _, op := range opts {
		op(o)
	}
}

func WithUsageLimits(cfg *metrics.UsageLimits) ExecutorOption {
	return func(o *ExecutorOptions) {
		o.UsageLimits = cfg
	}
}

func NewQueuedExecutor(opts ...ExecutorOption) *QueuedExecutor {
	options := ExecutorOptions{}
	options.Apply(opts...)

	if options.UsageLimits == nil {
		options.UsageLimits = &metrics.UsageLimits{
			ConcurrentProcessLimit:  host.AutoConcurrentProcessLimit(),
			QueuePressureMultiplier: 1,
			QueueRejectMultiplier:   1,
		}
	} else if options.UsageLimits.ConcurrentProcessLimit == -1 {
		options.UsageLimits.ConcurrentProcessLimit =
			host.AutoConcurrentProcessLimit()
	}

	queue := make(chan Task)
	s := &QueuedExecutor{
		ExecutorOptions: options,
		taskQueue:       queue,
		workerPool:      NewWorkerPool(queue),
		numRunning:      atomic.NewInt32(0),
		numQueued:       atomic.NewInt32(0),
	}
	s.workerPool.SetWorkerCount(int(s.UsageLimits.ConcurrentProcessLimit))
	return s
}

func NewDelegatingExecutor() *DelegatingExecutor {
	return &DelegatingExecutor{
		numTasks: atomic.NewInt32(0),
	}
}

func (x *QueuedExecutor) SetUsageLimits(cfg *metrics.UsageLimits) {
	x.UsageLimits = cfg
	go x.workerPool.SetWorkerCount(int(cfg.GetConcurrentProcessLimit()))
}

func (x *QueuedExecutor) Exec(task Task) error {
	x.numQueued.Inc()
	ctx := context.Background()
	x.taskQueue <- contextTask{
		Task: task,
		ctx:  ctx,
	}
	x.numQueued.Dec()

	x.numRunning.Inc()
	defer x.numRunning.Dec()

	<-ctx.Done()
	return task.Err()
}

func (x *QueuedExecutor) ExecAsync(task Task) <-chan error {
	ch := make(chan error)
	x.numQueued.Inc()
	ctx := context.Background()
	x.taskQueue <- contextTask{
		Task: task,
		ctx:  ctx,
	}
	x.numQueued.Dec()

	go func() {
		// Order is important here. Need to decrement the numRunning counter before
		// writing to the error channel, because the receiver of the channel might
		// not be listening on it yet. We don't want to hold up the running counter,
		// since the task is actually done running.
		x.numRunning.Inc()
		<-ctx.Done()
		x.numRunning.Dec()
		ch <- task.Err()
		close(ch)
	}()

	return ch
}

func (x *QueuedExecutor) CompleteUsageLimits(stat *metrics.UsageLimits) {
	stat.ConcurrentProcessLimit = x.UsageLimits.ConcurrentProcessLimit
	stat.QueuePressureMultiplier = x.UsageLimits.QueuePressureMultiplier
	stat.QueueRejectMultiplier = x.UsageLimits.QueueRejectMultiplier
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

func (x *DelegatingExecutor) Exec(task Task) error {
	x.numTasks.Inc()
	defer x.numTasks.Dec()

	return <-RunAsync(task)
}

func (x *DelegatingExecutor) ExecAsync(task Task) <-chan error {
	ch := make(chan error)

	go func() {
		x.numTasks.Inc()
		task.Run()
		x.numTasks.Dec()
		ch <- task.Err()
		close(ch)
	}()

	return ch
}

func (x *DelegatingExecutor) CompleteUsageLimits(stat *metrics.UsageLimits) {}

func (x *DelegatingExecutor) CompleteTaskStatus(stat *metrics.TaskStatus) {
	stat.NumDelegated = x.numTasks.Load()
}
