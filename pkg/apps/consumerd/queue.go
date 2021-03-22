package consumerd

import (
	"context"

	"github.com/cobalt77/kubecc/pkg/clients"
	"github.com/cobalt77/kubecc/pkg/meta"
	"github.com/cobalt77/kubecc/pkg/metrics"
	"github.com/cobalt77/kubecc/pkg/run"
	"github.com/cobalt77/kubecc/pkg/types"
	"github.com/cobalt77/kubecc/pkg/util"
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

	telemetry Telemetry
}

type SplitQueueOptions struct {
	telemetryCfg TelemetryConfig
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

func NewSplitQueue(
	ctx context.Context,
	monClient types.MonitorClient,
	opts ...SplitQueueOption,
) *SplitQueue {
	options := SplitQueueOptions{}
	options.Apply(opts...)
	capacity := int64(1000) // todo
	queue := make(chan run.Task, capacity)
	sq := &SplitQueue{
		PauseController: util.NewPauseController(),
		ctx:             ctx,
		lg:              meta.Log(ctx),
		inputQueue:      queue,
		avc: clients.NewAvailabilityChecker(
			clients.ComponentFilter(types.Scheduler),
		),
		telemetry: Telemetry{
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

	sq.localWorkers.SetWorkerCount(35)  // todo
	sq.remoteWorkers.SetWorkerCount(50) // todo
	clients.WatchAvailability(ctx, types.Scheduler, monClient, sq.avc)

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

}

func (sq *SplitQueue) CompleteTaskStatus(m *metrics.TaskStatus) {
	m.NumQueued = sq.telemetry.numQueued.Load()
	m.NumRunning = sq.telemetry.numRunning.Load()
	m.NumDelegated = sq.telemetry.numDelegated.Load()
}

func (sq *SplitQueue) Telemetry() *Telemetry {
	return &sq.telemetry
}
