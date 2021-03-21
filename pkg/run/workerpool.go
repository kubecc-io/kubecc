package run

import (
	"sync"

	"github.com/cobalt77/kubecc/pkg/util"
	mapset "github.com/deckarep/golang-set"
)

type WorkerPool struct {
	*util.PauseController
	taskQueue  <-chan Task
	stopQueue  chan struct{}
	workers    mapset.Set // *worker
	workerLock *sync.Mutex
	runner     func(Task)
}

type WorkerPoolOptions struct {
	runner func(Task)
	paused bool
}

type WorkerPoolOption func(*WorkerPoolOptions)

func (o *WorkerPoolOptions) Apply(opts ...WorkerPoolOption) {
	for _, op := range opts {
		op(o)
	}
}

func WithRunner(f func(Task)) WorkerPoolOption {
	return func(o *WorkerPoolOptions) {
		o.runner = f
	}
}

func DefaultPaused() WorkerPoolOption {
	return func(o *WorkerPoolOptions) {
		o.paused = true
	}
}

func NewWorkerPool(taskQueue <-chan Task, opts ...WorkerPoolOption) *WorkerPool {
	options := WorkerPoolOptions{
		runner: func(t Task) {
			t.Run()
		},
	}
	options.Apply(opts...)

	queue := make(chan Task)
	wp := &WorkerPool{
		PauseController: util.NewPauseController(util.DefaultPaused(options.paused)),
		taskQueue:       queue,
		stopQueue:       make(chan struct{}),
		runner:          options.runner,
		workers:         mapset.NewSet(),
		workerLock:      &sync.Mutex{},
	}

	go func() {
		defer close(queue)
		for {
			wp.CheckPaused()
			task, open := <-taskQueue
			if !open {
				return
			}
			queue <- task
		}
	}()

	return wp
}

func (wp *WorkerPool) SetWorkerCount(count int) {
	wp.workerLock.Lock()
	defer wp.workerLock.Unlock()
	if numWorkers := wp.workers.Cardinality(); count > numWorkers {
		for wp.workers.Cardinality() != count {
			w := &worker{
				taskQueue: wp.taskQueue,
				stopQueue: wp.stopQueue,
				runner:    wp.runner,
			}
			wp.workers.Add(w)
			go func() {
				w.Run()
				wp.workers.Remove(w)
			}()
		}
	} else if count < numWorkers && count >= 0 {
		for i := 0; i < numWorkers-count; i++ {
			wp.stopQueue <- struct{}{}
		}
	}
}

type worker struct {
	taskQueue <-chan Task
	stopQueue <-chan struct{}
	runner    func(Task)
}

func (w *worker) Run() {
	for {
		// Checking stopQueue up front allows it to terminate immediately,
		// since if both channels can be read from, go will pick one at random.
		select {
		case <-w.stopQueue:
			return
		default:
		}
		task := <-w.taskQueue
		w.runner(task)
	}
}
