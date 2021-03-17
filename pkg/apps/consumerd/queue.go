package consumerd

import (
	"context"

	"github.com/cobalt77/kubecc/pkg/clients"
	"github.com/cobalt77/kubecc/pkg/run"
	"github.com/cobalt77/kubecc/pkg/types"
)

type SplitTask struct {
	Local, Remote run.PackagedTask
}

func (s *SplitTask) Wait() (interface{}, error) {
	select {
	case results := <-s.Local.C:
		return results.Response, results.Err
	case results := <-s.Remote.C:
		return results.Response, results.Err
	}
}

type SplitQueue struct {
	ctx       context.Context
	avc       *clients.AvailabilityChecker
	taskQueue chan *SplitTask
}

type QueueAction int

const (
	Requeue QueueAction = iota
	DoNotRequeue
)

func NewSplitQueue(
	ctx context.Context,
	monClient types.MonitorClient,
) *SplitQueue {
	sq := &SplitQueue{
		ctx:       ctx,
		taskQueue: make(chan *SplitTask),
		avc: clients.NewAvailabilityChecker(
			clients.ComponentFilter(types.Scheduler),
		),
	}
	clients.WatchAvailability(ctx, types.Scheduler, monClient, sq.avc)

	go sq.runLocalQueue()
	go sq.runRemoteQueue()
	return sq
}

func (s *SplitQueue) In() chan<- *SplitTask {
	return s.taskQueue
}

func (s *SplitQueue) processTask(pt run.PackagedTask) QueueAction {
	response, err := pt.F()
	if err != nil {
		return Requeue
	}
	pt.C <- struct {
		Response interface{}
		Err      error
	}{
		Response: response,
		Err:      err,
	}
	return DoNotRequeue
}

func (s *SplitQueue) runLocalQueue() {
	for {
		select {
		case <-s.ctx.Done():
			return
		case task := <-s.taskQueue:
			switch s.processTask(task.Local) {
			case Requeue:
				s.In() <- task
			}
		}
	}
}

func (s *SplitQueue) runRemoteQueue() {
	for {
		available := s.avc.EnsureAvailable()
		for {
			select {
			case <-s.ctx.Done():
				return
			case <-available.Done():
				goto restart
			case task := <-s.taskQueue:
				switch s.processTask(task.Remote) {
				case Requeue:
					s.In() <- task
				}
			}
		}
	restart:
	}
}
