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

var futureTaskPool = sync.Pool{
	New: func() interface{} {
		return &futureTask{
			// The channel has a buffer of 1 to reduce contention between the
			// goroutine managing the upstream queue and the worker goroutine
			C: make(chan Task, 1),
		}
	},
}

// WorkerPool is a dynamic pool of worker goroutines which run on a shared
// task queue. The number of workers can be changed at any time, and the
// stream itself can be paused and unpaused, which can be used to temporarily
// stop/start all workers.
type WorkerPool struct {
	*util.PauseController
	taskQueue  chan *futureTask
	stopQueue  chan struct{}
	workers    mapset.Set // *worker
	workerLock *sync.Mutex
	runner     func(Task)
}

type futureTask struct {
	C chan Task
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

	wp := &WorkerPool{
		PauseController: util.NewPauseController(util.DefaultPaused(options.paused)),
		taskQueue:       make(chan *futureTask),
		stopQueue:       make(chan struct{}), // Note the stop queue is not buffered
		runner:          options.runner,
		workers:         mapset.NewSet(),
		workerLock:      &sync.Mutex{},
	}

	go wp.manageQueue(taskQueue)
	return wp
}

func (wp *WorkerPool) manageQueue(upstream <-chan Task) {
	defer close(wp.taskQueue)
	for {
		// ensure the worker pool is not paused
		wp.CheckPaused()

		// Take a futureTask from the pool. This object contains a channel that
		// will eventually hold a single Task.
		ft := futureTaskPool.Get().(*futureTask)

		// Send the futureTask to a worker
		wp.taskQueue <- ft

		// Only when a worker has accepted the task, try to take a task from
		// the upstream queue and write it on the futureTask's channel. This is
		// done because if there are no available workers, a single task will be
		// stuck here in limbo, unable to be processed by a different worker pool.
		task, open := <-upstream
		if !open {
			// upstream is closed, we are done
			return
		}
		// Send the task to the futureTask's channel. The channel has a single
		// element buffer, so this won't block. The worker is responsible for
		// putting the futureTask back into the pool.
		ft.C <- task
	}
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
	taskQueue <-chan *futureTask
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

		// Get a futureTask from the queue
		ft := <-w.taskQueue

		// Wait until a task is sent on the futureTask's channel
		task := <-ft.C

		// Put the futureTask back into the global pool
		futureTaskPool.Put(ft)

		// Run the task
		w.runner(task)
	}
}
