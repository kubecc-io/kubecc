package consumerd

import (
	"context"

	"github.com/cobalt77/kubecc/pkg/clients"
	"github.com/cobalt77/kubecc/pkg/meta"
	"github.com/cobalt77/kubecc/pkg/run"
	"github.com/cobalt77/kubecc/pkg/types"
	"go.uber.org/zap"
)

type SplitTask struct {
	Local, Remote run.PackagedRequest
}

func (s *SplitTask) Wait() (interface{}, error) {
	select {
	case resp := <-s.Local.Response():
		return resp, s.Local.Err()
	case resp := <-s.Remote.Response():
		return resp, s.Remote.Err()
	}
}

func (*SplitTask) Run() {
	panic("Run is not implemented for SplitTask; Use Local.Run or Remote.Run")
}

func (*SplitTask) Err() error {
	panic("Err is not implemented for SplitTask; Use Local.Err or Remote.Err")
}

type SplitQueue struct {
	ctx        context.Context
	lg         *zap.SugaredLogger
	avc        *clients.AvailabilityChecker
	inputQueue chan run.Task

	localWorkers  *run.WorkerPool
	remoteWorkers *run.WorkerPool
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
	queue := make(chan run.Task)
	sq := &SplitQueue{
		ctx:        ctx,
		lg:         meta.Log(ctx),
		inputQueue: queue,
		avc: clients.NewAvailabilityChecker(
			clients.ComponentFilter(types.Scheduler),
		),
		localWorkers: run.NewWorkerPool(queue, run.WithRunner(func(t run.Task) {
			t.(*SplitTask).Local.Run()
		})),
		remoteWorkers: run.NewWorkerPool(queue, run.WithRunner(func(t run.Task) {
			t.(*SplitTask).Remote.Run()
		})),
	}
	sq.remoteWorkers.Pause() // Starts paused, resumes when scheduler connects
	clients.WatchAvailability(ctx, types.Scheduler, monClient, sq.avc)

	go sq.handleAvailabilityChanged()
	return sq
}

func (s *SplitQueue) Put(st *SplitTask) {
	s.inputQueue <- st
}

func (s *SplitQueue) handleAvailabilityChanged() {
	for {
		if s.ctx.Err() != nil {
			return
		}
		available := s.avc.EnsureAvailable()
		s.remoteWorkers.Resume()
		s.lg.Debug("Remote is now available")
		<-available.Done()
		s.remoteWorkers.Pause()
		s.lg.Debug("Remote is no longer available")
	}
}
