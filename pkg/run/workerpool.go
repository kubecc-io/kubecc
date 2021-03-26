/*
Copyright 2021 The Kubecc Authors.

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU General Public License for more details.

You should have received a copy of the GNU General Public License
along with this program.  If not, see <http://www.gnu.org/licenses/>.
*/

package run

import (
	"sync"

	"github.com/cobalt77/kubecc/pkg/util"
	mapset "github.com/deckarep/golang-set"
)

// WorkerPool is a dynamic pool of worker goroutines which run on a shared
// task queue. The number of workers can be changed at any time, and the
// stream itself can be paused and unpaused, which can be used to temporarily
// stop/start all workers.
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

// WithRunner sets the function that will be run by a worker when processing
// a task.
func WithRunner(f func(Task)) WorkerPoolOption {
	return func(o *WorkerPoolOptions) {
		o.runner = f
	}
}

// DefaultPaused indicates the worker pool should start in the paused state.
// This should be used instead of starting the pool and immediately pausing
// it to avoid race conditions.
func DefaultPaused() WorkerPoolOption {
	return func(o *WorkerPoolOptions) {
		o.paused = true
	}
}

// NewWorkerPool creates a new WorkerPool with the provided task queue.
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
		stopQueue:       make(chan struct{}), // Note the stop queue is not buffered
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

// Resize sets the target number of workers that should be running
// in the pool. When decreasing the number of workers, only workers which
// are not currently running a task will be stopped. If all workers are busy,
// the pool will stop the next available workers when they have finished
// their current task.
func (wp *WorkerPool) Resize(count int64) {
	wp.workerLock.Lock()
	defer wp.workerLock.Unlock()
	if numWorkers := wp.workers.Cardinality(); int(count) > numWorkers {
		for int64(wp.workers.Cardinality()) != count {
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
	} else if int(count) < numWorkers && count >= 0 {
		for i := 0; i < numWorkers-int(count); i++ {
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
