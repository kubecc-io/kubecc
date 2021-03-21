package consumerd

import (
	"container/ring"
	"context"
	"time"

	"github.com/cobalt77/kubecc/pkg/clients"
	"github.com/cobalt77/kubecc/pkg/meta"
	"github.com/cobalt77/kubecc/pkg/metrics"
	"github.com/cobalt77/kubecc/pkg/run"
	"github.com/cobalt77/kubecc/pkg/types"
	"github.com/cobalt77/kubecc/pkg/util"
	"go.uber.org/atomic"
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

	numRunning   *atomic.Int32
	numQueued    *atomic.Int32
	numDelegated *atomic.Int32
	numCompleted *atomic.Int32
	telemetry    Telemetry
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

	queue := make(chan run.Task)
	sq := &SplitQueue{
		PauseController: util.NewPauseController(),
		ctx:             ctx,
		lg:              meta.Log(ctx),
		inputQueue:      queue,
		avc: clients.NewAvailabilityChecker(
			clients.ComponentFilter(types.Scheduler),
		),
		numRunning:   atomic.NewInt32(0),
		numQueued:    atomic.NewInt32(0),
		numDelegated: atomic.NewInt32(0),
		numCompleted: atomic.NewInt32(0),
		telemetry: Telemetry{
			recording: atomic.NewBool(false),
			history:   ring.New(int(options.telemetryCfg.HistoryLen)),
			conf:      options.telemetryCfg,
		},
	}
	if sq.telemetry.conf.Enabled {
		sq.telemetry.history = ring.New(int(sq.telemetry.conf.HistoryLen))
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
	sq.incRunning()
	defer sq.decRunning()
	t.(*SplitTask).Local.Run()
}

func (sq *SplitQueue) remoteRunner(t run.Task) {
	sq.incDelegated()
	defer sq.decDelegated()
	t.(*SplitTask).Remote.Run()
}

func (sq *SplitQueue) Exec(task run.Task) error {
	if _, ok := task.(*SplitTask); !ok {
		return run.ErrUnsupportedTask
	}
	sq.incQueued()
	defer sq.decQueued()
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
	m.NumQueued = sq.numQueued.Load()
	m.NumRunning = sq.numRunning.Load()
	m.NumDelegated = sq.numDelegated.Load()
}

func (sq *SplitQueue) incQueued() {
	sq.telemetry.RecordEntry(Entry{
		X:    time.Now(),
		Y:    float64(sq.numQueued.Inc()),
		Kind: QueuedTasks,
	})
}

func (sq *SplitQueue) decQueued() {
	sq.telemetry.RecordEntry(Entry{
		X:    time.Now(),
		Y:    float64(sq.numQueued.Dec()),
		Kind: QueuedTasks,
	})
}

func (sq *SplitQueue) incRunning() {
	sq.telemetry.RecordEntry(Entry{
		X:    time.Now(),
		Y:    float64(sq.numRunning.Inc()),
		Kind: RunningTasks,
	})
}

func (sq *SplitQueue) decRunning() {
	sq.telemetry.RecordEntry(Entry{
		X:    time.Now(),
		Y:    float64(sq.numRunning.Dec()),
		Kind: RunningTasks,
	})
	sq.telemetry.RecordEntry(Entry{
		X:    time.Now(),
		Y:    float64(sq.numCompleted.Inc()),
		Kind: CompletedTasks,
		Loc:  Local,
	})
}

func (sq *SplitQueue) incDelegated() {
	sq.telemetry.RecordEntry(Entry{
		X:    time.Now(),
		Y:    float64(sq.numDelegated.Inc()),
		Kind: DelegatedTasks,
	})
}

func (sq *SplitQueue) decDelegated() {
	sq.telemetry.RecordEntry(Entry{
		X:    time.Now(),
		Y:    float64(sq.numDelegated.Dec()),
		Kind: DelegatedTasks,
	})
	sq.telemetry.RecordEntry(Entry{
		X:    time.Now(),
		Y:    float64(sq.numCompleted.Inc()),
		Kind: CompletedTasks,
		Loc:  Remote,
	})
}

func (sq *SplitQueue) Telemetry() *Telemetry {
	return &sq.telemetry
}
