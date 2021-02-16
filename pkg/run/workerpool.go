package run

import (
	"sync"

	mapset "github.com/deckarep/golang-set"
)

type WorkerPool struct {
	taskQueue  <-chan *Task
	stopQueue  chan struct{}
	workers    mapset.Set // map[*worker]
	workerLock *sync.Mutex
}

func NewWorkerPool(taskQueue <-chan *Task) *WorkerPool {
	return &WorkerPool{
		taskQueue:  taskQueue,
		stopQueue:  make(chan struct{}),
		workers:    mapset.NewSet(),
		workerLock: &sync.Mutex{},
	}
}

func (wp *WorkerPool) SetWorkerCount(count int) {
	wp.workerLock.Lock()
	defer wp.workerLock.Unlock()
	if numWorkers := wp.workers.Cardinality(); count > numWorkers {
		for wp.workers.Cardinality() != count {
			w := &worker{
				taskQueue: wp.taskQueue,
				stopQueue: wp.stopQueue,
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
	taskQueue <-chan *Task
	stopQueue <-chan struct{}
}

func (w *worker) Run() {
	for {
		select {
		case <-w.stopQueue:
			return
		default:
		}
		task := <-w.taskQueue
		task.Run()
		select {
		case <-task.Done():
		case <-task.ctx.Done():
		}
	}
}
