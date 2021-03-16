package consumerd

import (
	"context"
	"sync"

	"github.com/cobalt77/kubecc/pkg/run"
)

type remoteStatus int

const (
	unavailable remoteStatus = iota
	available
	full
)

type remoteStatusManager struct {
	status remoteStatus
	cond   *sync.Cond
}

func newRemoteStatusManager() *remoteStatusManager {
	rsm := &remoteStatusManager{
		status: unavailable,
		cond:   sync.NewCond(&sync.Mutex{}),
	}

	// todo: watch monitor for scheduler status
	return rsm
}

func (rsm *remoteStatusManager) EnsureStatus(stat remoteStatus) <-chan struct{} {
	ch := make(chan struct{})
	defer func() {
		go func() {
			rsm.cond.L.Lock()
			defer rsm.cond.L.Unlock()

			for {
				if rsm.status != stat {
					close(ch)
					return
				}
				rsm.cond.Wait()
			}
		}()
	}()

	rsm.cond.L.Lock()
	defer rsm.cond.L.Unlock()

	for {
		if rsm.status == stat {
			return ch
		}
		rsm.cond.Wait()
	}
}

func (rsm *remoteStatusManager) SetStatus(stat remoteStatus) {
	rsm.cond.L.Lock()
	defer rsm.cond.L.Unlock()
	rsm.status = stat
	rsm.cond.Broadcast()
}

type splitTask struct {
	local, remote run.PackagedTask
}

func (s *splitTask) Wait() (interface{}, error) {
	select {
	case results := <-s.local.C:
		return results.Response, results.Err
	case results := <-s.remote.C:
		return results.Response, results.Err
	}
}

type splitQueue struct {
	ctx       context.Context
	rsm       *remoteStatusManager
	taskQueue chan *splitTask
}

type queueAction int

const (
	requeue queueAction = iota
	doNotRequeue
)

func NewSplitQueue(
	ctx context.Context,
) *splitQueue {
	sq := &splitQueue{
		ctx:       ctx,
		taskQueue: make(chan *splitTask),
		rsm:       newRemoteStatusManager(),
	}

	go sq.runLocalQueue()
	go sq.runRemoteQueue()
	return sq
}

func (s *splitQueue) In() chan<- *splitTask {
	return s.taskQueue
}

func (s *splitQueue) processTask(pt run.PackagedTask) queueAction {
	response, err := pt.F()
	if err != nil {
		return requeue
	}
	pt.C <- struct {
		Response interface{}
		Err      error
	}{
		Response: response,
		Err:      err,
	}
	return doNotRequeue
}

func (s *splitQueue) runLocalQueue() {
	for {
		select {
		case <-s.ctx.Done():
			return
		case task := <-s.taskQueue:
			switch s.processTask(task.local) {
			case requeue:
				s.In() <- task
			}
		}
	}
}

func (s *splitQueue) runRemoteQueue() {
	for {
		statusChanged := s.rsm.EnsureStatus(available)
		for {
			select {
			case <-s.ctx.Done():
				return
			case <-statusChanged:
				goto restart
			case task := <-s.taskQueue:
				switch s.processTask(task.remote) {
				case requeue:
					s.In() <- task
				}
			}
		}
	restart:
	}
}
