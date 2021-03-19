package consumerd

import (
	"context"
	"time"

	"github.com/cobalt77/kubecc/pkg/clients"
	"github.com/cobalt77/kubecc/pkg/meta"
	"github.com/cobalt77/kubecc/pkg/run"
	"github.com/cobalt77/kubecc/pkg/types"
	"go.uber.org/zap"
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
	lg        *zap.SugaredLogger
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
		lg:        meta.Log(ctx),
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
	s.lg.Debug("Processing packaged task")
	response, err := pt.F()
	if err != nil {
		s.lg.With(zap.Error(err)).Debug("Requeueing")
		return Requeue
	}
	pt.C <- struct {
		Response interface{}
		Err      error
	}{
		Response: response,
		Err:      err,
	}
	s.lg.Debug("Success - not requeueing")
	return DoNotRequeue
}

func (s *SplitQueue) runLocalQueue() {
	for {
		select {
		case <-s.ctx.Done():
			return
		case task := <-s.taskQueue:
			s.lg.Debug("Received task on local queue")
			switch s.processTask(task.Local) {
			case Requeue:
				s.In() <- task
			}
			time.Sleep(10 * time.Millisecond)
		}
	}
}

func (s *SplitQueue) runRemoteQueue() {
	for {
		available := s.avc.EnsureAvailable()
		s.lg.Debug("Remote is now available")
		for {
			select {
			case <-s.ctx.Done():
				return
			case <-available.Done():
				s.lg.Debug("Remote is no longer available")
				goto restart
			case task := <-s.taskQueue:
				s.lg.Debug("Received task on remote queue")
				switch s.processTask(task.Remote) {
				case Requeue:
					s.In() <- task
				}
			}
		}
	restart:
	}
}
