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

package consumerd

import (
	"context"

	"github.com/kubecc-io/kubecc/pkg/clients"
	"github.com/kubecc-io/kubecc/pkg/host"
	"github.com/kubecc-io/kubecc/pkg/meta"
	"github.com/kubecc-io/kubecc/pkg/metrics"
	"github.com/kubecc-io/kubecc/pkg/run"
	"github.com/kubecc-io/kubecc/pkg/types"
	"github.com/kubecc-io/kubecc/pkg/util"
	"go.uber.org/zap"
)

type SplitTaskLocation int

const (
	Unknown SplitTaskLocation = iota
	Local
	Remote
)

type SplitTask struct {
	Local  run.PackagedRequest
	Remote run.PackagedRequest
	which  SplitTaskLocation
}

func (st *SplitTask) Wait() (interface{}, error) {
	select {
	case resp := <-st.Local.Response():
		st.which = Local
		return resp, st.Local.Err()
	case resp := <-st.Remote.Response():
		st.which = Remote
		return resp, st.Remote.Err()
	}
}

func (st *SplitTask) Which() SplitTaskLocation {
	return st.which
}

func (*SplitTask) Run() {
	panic("Run is not implemented for SplitTask; Use Local.Run or Remote.Run")
}

func (*SplitTask) Err() error {
	panic("Err is not implemented for SplitTask; Use Local.Err or Remote.Err")
}

type SplitQueue struct {
	*util.PauseController
	ctx        context.Context
	lg         *zap.SugaredLogger
	avc        *clients.AvailabilityChecker
	inputQueue chan run.Task

	localWorkers  *run.WorkerPool
	remoteWorkers *run.WorkerPool

	telemetry *Telemetry
}

type SplitQueueOptions struct {
	telemetryCfg   TelemetryConfig
	bufferSize     int
	localUsageMgr  run.ResizerManager
	remoteUsageMgr run.ResizerManager
}

type SplitQueueOption func(*SplitQueueOptions)

func (o *SplitQueueOptions) Apply(opts ...SplitQueueOption) {
	for _, op := range opts {
		op(o)
	}
}

func WithTelemetryConfig(cfg TelemetryConfig) SplitQueueOption {
	return func(o *SplitQueueOptions) {
		o.telemetryCfg = cfg
	}
}

func WithBufferSize(sz int) SplitQueueOption {
	return func(o *SplitQueueOptions) {
		o.bufferSize = sz
	}
}

func WithLocalUsageManager(rm run.ResizerManager) SplitQueueOption {
	return func(o *SplitQueueOptions) {
		o.localUsageMgr = rm
	}
}

func WithRemoteUsageManager(rm run.ResizerManager) SplitQueueOption {
	return func(o *SplitQueueOptions) {
		o.remoteUsageMgr = rm
	}
}

type NoRemoteUsageManager struct{}

func (NoRemoteUsageManager) Manage(run.Resizer) {}

type fixedUsageLimits struct {
	limit int64
}

func (f fixedUsageLimits) Manage(r run.Resizer) {
	r.Resize(f.limit)
}

func FixedUsageLimits(limit int64) run.ResizerManager {
	return fixedUsageLimits{limit: limit}
}

type autoUsageLimits struct {
	limit int64
}

func (a autoUsageLimits) Manage(r run.Resizer) {
	r.Resize(a.limit)
}

func AutoUsageLimits() run.ResizerManager {
	return autoUsageLimits{
		limit: int64(host.AutoConcurrentProcessLimit()),
	}
}

func NewSplitQueue(
	ctx context.Context,
	monClient types.MonitorClient,
	opts ...SplitQueueOption,
) *SplitQueue {
	options := SplitQueueOptions{
		bufferSize:     1000,
		localUsageMgr:  NoRemoteUsageManager{},
		remoteUsageMgr: NoRemoteUsageManager{},
	}
	options.Apply(opts...)
	capacity := int64(options.bufferSize)
	queue := make(chan run.Task, capacity)
	sq := &SplitQueue{
		PauseController: util.NewPauseController(),
		ctx:             ctx,
		lg:              meta.Log(ctx),
		inputQueue:      queue,
		avc: clients.NewAvailabilityChecker(
			clients.ComponentFilter(types.Scheduler),
		),
		telemetry: &Telemetry{
			conf:          options.telemetryCfg,
			queueCapacity: capacity,
		},
	}
	sq.telemetry.init()
	if sq.telemetry.conf.Enabled {
		sq.telemetry.StartRecording()
	}
	sq.localWorkers = run.NewWorkerPool(queue,
		run.WithRunner(sq.localRunner),
	)
	sq.remoteWorkers = run.NewWorkerPool(queue,
		run.WithRunner(sq.remoteRunner),
		run.DefaultPaused(),
	)

	go options.localUsageMgr.Manage(sq.localWorkers)
	go options.remoteUsageMgr.Manage(sq.remoteWorkers)

	clients.WatchAvailability(ctx, monClient, sq.avc)
	go sq.handleAvailabilityChanged()
	return sq
}

func (sq *SplitQueue) localRunner(t run.Task) {
	sq.telemetry.decQueued()
	sq.telemetry.incRunning()
	defer sq.telemetry.decRunning()
	t.(*SplitTask).Local.Run()
}

func (sq *SplitQueue) remoteRunner(t run.Task) {
	sq.telemetry.decQueued()
	sq.telemetry.incDelegated()
	defer sq.telemetry.decDelegated()
	t.(*SplitTask).Remote.Run()
}

func (sq *SplitQueue) Exec(task run.Task) error {
	if _, ok := task.(*SplitTask); !ok {
		return run.ErrUnsupportedTask
	}
	sq.telemetry.incQueued()
	sq.inputQueue <- task
	return nil
}

func (sq *SplitQueue) handleAvailabilityChanged() {
	for {
		if sq.ctx.Err() != nil {
			return
		}
		available := sq.avc.EnsureAvailable()
		sq.remoteWorkers.Resume()
		sq.lg.Debug("Remote is now available")
		<-available.Done()
		sq.remoteWorkers.Pause()
		sq.lg.Debug("Remote is no longer available")
	}
}

func (sq *SplitQueue) CompleteUsageLimits(m *metrics.UsageLimits) {
	m.ConcurrentProcessLimit = int32(sq.localWorkers.Size())
	m.DelegatedTaskLimit = int32(sq.remoteWorkers.Size())
}

func (sq *SplitQueue) CompleteTaskStatus(m *metrics.TaskStatus) {
	m.NumQueued = sq.telemetry.numQueued.Load()
	m.NumRunning = sq.telemetry.numRunning.Load()
	m.NumDelegated = sq.telemetry.numDelegated.Load()
}

func (sq *SplitQueue) CompleteLocalTasksCompleted(m *metrics.LocalTasksCompleted) {
	m.Total = int64(sq.telemetry.numCompletedLocal.Load())
}

func (sq *SplitQueue) CompleteDelegatedTasksCompleted(m *metrics.DelegatedTasksCompleted) {
	m.Total = int64(sq.telemetry.numCompletedRemote.Load())
}

func (sq *SplitQueue) Telemetry() *Telemetry {
	return sq.telemetry
}
