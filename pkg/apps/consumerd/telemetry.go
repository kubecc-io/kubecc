package consumerd

import (
	"container/ring"
	"math/rand"
	"sort"
	"sync"
	"time"

	"go.uber.org/atomic"
	"gonum.org/v1/gonum/stat"
	"gonum.org/v1/plot/plotter"
)

// Telemetry stores sampled data from the consumerd split queue.
// Consumerd telemetry is of particular importance due to the performance
// impact of balancing local and remote tasks. It is important to be able
// to log and visualize the state of the consumerd queue to ensure it is
// performing correctly and to help identify slowdowns, contention, etc.
// Operations on the Telemetry object are thread-safe.
type Telemetry struct {
	conf TelemetryConfig

	recording *atomic.Bool
	history   *ring.Ring
	mu        sync.Mutex
}

func (t *Telemetry) RecordEntry(i Entry) {
	if !t.recording.Load() {
		return
	}
	if rand.Float64() <= t.conf.SampleRate {
		t.mu.Lock()
		t.history.Value = i
		t.history = t.history.Next()
		t.mu.Unlock()
	}
}

type Entry struct {
	X    time.Time
	Y    float64
	Kind EntryKind
	Loc  SplitTaskLocation
}

type EntryKind int

const (
	CompletedTasks EntryKind = iota
	RunningTasks
	QueuedTasks
	DelegatedTasks
)

func (t *Telemetry) StartRecording() {
	t.recording.Store(true)
}

func (t *Telemetry) StopRecording() {
	t.recording.Store(false)
}

func (t *Telemetry) Clear() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.history = ring.New(int(t.conf.HistoryLen))
}

type Entries []Entry

// Entries returns a slice of all the entries currently in the history buffer.
func (t *Telemetry) Entries() (entries Entries) {
	entries = make(Entries, 0, t.conf.HistoryLen)
	t.mu.Lock()
	defer t.mu.Unlock()
	t.history.Do(func(i interface{}) {
		if i == nil {
			return
		}
		entries = append(entries, i.(Entry))
	})
	return
}

type TelemetryConfig struct {
	Enabled    bool
	SampleRate float64
	HistoryLen int64
}

func (e Entries) Filter(include func(Entry) bool) (entries Entries) {
	entries = make(Entries, 0, len(e)/2)
	for _, entry := range e {
		if include(entry) {
			entries = append(entries, entry)
		}
	}
	return
}

// TimeRange returns a sub-slice of all entries where the timestamps of the
// returned entries are in the range [begin, end).
// If begin or end are the zero timestamp, they will be set to the timestamp
// of the first or last entry, respectively.
func (e Entries) TimeRange(begin, end time.Time) Entries {
	if end.Before(begin) {
		panic("end must be after begin")
	}
	var startIndex, endIndex int
	if begin.IsZero() {
		startIndex = 0
	} else {
		startIndex = sort.Search(len(e), func(i int) bool {
			t := e[i].X
			return t.Before(begin) || t.Equal(begin)
		})
	}
	if end.IsZero() {
		endIndex = len(e) - 1
	} else {
		endIndex = sort.Search(len(e), func(i int) bool {
			t := e[i].X
			return t.Before(end) || t.Equal(end)
		})
	}
	return e[startIndex:endIndex]
}

func (e Entries) LinearRegression() (alpha, beta float64) {
	times := make([]float64, 0, len(e))
	values := make([]float64, 0, len(e))
	for _, v := range e {
		times = append(times, float64(v.X.UnixNano()))
		values = append(values, v.Y)
	}
	return stat.LinearRegression(times, values, nil, false)
}

func (e Entries) ToXYs() (xys plotter.XYs) {
	if len(e) == 0 {
		return
	}
	startTime := e[0].X
	xys = make(plotter.XYs, len(e))
	for i, v := range e {
		xys[i] = plotter.XY{
			X: float64(v.X.Sub(startTime).Milliseconds()),
			Y: v.Y,
		}
	}
	return
}

func (e Entries) Deltas() (entries Entries) {
	entries = make(Entries, len(e))
	for i, v := range e {
		if i == 0 {
			entries[i] = Entry{
				X: v.X,
				Y: 0,
			}
			continue
		}
		entries[i] = Entry{
			X: v.X,
			Y: v.Y - e[i-1].Y,
		}
	}
	return
}
