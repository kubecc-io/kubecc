package run

import (
	"sync"

	mapset "github.com/deckarep/golang-set"
)

type WorkerPool struct {
	taskQueue  <-chan Task
	stopQueue  chan struct{}
	workers    mapset.Set // *worker
	workerLock *sync.Mutex
	runner     func(Task)
	paused     bool
	pause      *sync.Cond
}

type WorkerPoolOptions struct {
	runner func(Task)
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

func NewWorkerPool(taskQueue <-chan Task, opts ...WorkerPoolOption) *WorkerPool {
	options := WorkerPoolOptions{
		runner: func(t Task) {
			t.Run()
		},
	}
	options.Apply(opts...)

	queue := make(chan Task)
	wp := &WorkerPool{
		taskQueue:  queue,
		stopQueue:  make(chan struct{}),
		runner:     options.runner,
		workers:    mapset.NewSet(),
		workerLock: &sync.Mutex{},
		pause:      sync.NewCond(&sync.Mutex{}),
	}

	go func() {
		defer close(queue)
		for {
			wp.pause.L.Lock()
			for wp.paused {
				wp.pause.Wait()
			}
			wp.pause.L.Unlock()
			task, open := <-taskQueue
			if !open {
				return
			}
			queue <- task
		}
	}()

	return wp
}

func (wp *WorkerPool) Pause() {
	wp.pause.L.Lock()
	wp.paused = true
	wp.pause.L.Unlock()
}

func (wp *WorkerPool) Resume() {
	wp.pause.L.Lock()
	wp.paused = false
	wp.pause.L.Unlock()
	wp.pause.Signal()
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
