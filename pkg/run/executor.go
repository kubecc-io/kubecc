package run

import (
	"github.com/cobalt77/kubecc/pkg/cpuconfig"
	"github.com/cobalt77/kubecc/pkg/types"
	"go.uber.org/atomic"
)

type ExecutorStatus int

type Executor struct {
	ExecutorOptions
	workerPool *WorkerPool
	taskQueue  chan *Task
	numRunning *atomic.Int32
}

type ExecutorOptions struct {
	cpuConfig *types.CpuConfig
}
type executorOption func(*ExecutorOptions)

func (o *ExecutorOptions) Apply(opts ...executorOption) {
	for _, op := range opts {
		op(o)
	}
}

func WithCpuConfig(cfg *types.CpuConfig) executorOption {
	return func(o *ExecutorOptions) {
		o.cpuConfig = cfg
	}
}

func NewExecutor(opts ...executorOption) *Executor {
	options := ExecutorOptions{
		cpuConfig: cpuconfig.Default(),
	}
	options.Apply(opts...)
	queue := make(chan *Task)
	s := &Executor{
		ExecutorOptions: options,
		taskQueue:       queue,
		workerPool:      NewWorkerPool(queue),
		numRunning:      atomic.NewInt32(0),
	}
	s.workerPool.SetWorkerCount(int(s.cpuConfig.MaxRunningProcesses))
	return s
}

func (x *Executor) SetCpuConfig(cfg *types.CpuConfig) {
	go x.workerPool.SetWorkerCount(int(cfg.GetMaxRunningProcesses()))
}

func (x *Executor) Exec(
	task *Task,
	opts ...executorOption,
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

func (x *Executor) Status() types.QueueStatus {
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
