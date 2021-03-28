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
	"sort"
	"sync"
	"time"

	"github.com/cloudflare/golibs/ewma"
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
	history   Entries
	mu        sync.Mutex
	startTime time.Time

	numRunning         *atomic.Int32
	numQueued          *atomic.Int32
	numDelegated       *atomic.Int32
	numCompletedLocal  *atomic.Int32
	numCompletedRemote *atomic.Int32

	queueCapacity int64
	tickCancel    context.CancelFunc
}

func (t *Telemetry) init() {
	t.numRunning = atomic.NewInt32(0)
	t.numQueued = atomic.NewInt32(0)
	t.numDelegated = atomic.NewInt32(0)
	t.numCompletedLocal = atomic.NewInt32(0)
	t.numCompletedRemote = atomic.NewInt32(0)
	t.recording = atomic.NewBool(false)
	t.history = make(Entries, 0, t.conf.HistoryLen)
}

func (t *Telemetry) RecordEntry(i Entry) {
	if !t.recording.Load() {
		return
	}
	t.mu.Lock()
	i.X = time.Now()
	t.history = append(t.history, i)
	t.mu.Unlock()
}

type Entry struct {
	X    time.Time
	Y    float64
	Kind EntryKind
}

type EntryKind int

const (
	Invalid EntryKind = iota
	CompletedTasksLocal
	CompletedTasksRemote
	RunningTasks
	QueuedTasks
	DelegatedTasks
)

func (t *Telemetry) StartRecording() {
	t.startTime = time.Now()
	t.recording.Store(true)
	ticker := time.NewTicker(t.conf.RecordInterval)
	ctx, tickCancel := context.WithCancel(context.Background())
	t.tickCancel = tickCancel
	go func() {
		for {
			select {
			case <-ticker.C:
				t.recordValues()
			case <-ctx.Done():
				ticker.Stop()
				return
			}
		}
	}()
}

func (t *Telemetry) StopRecording() {
	t.recording.Store(false)
	t.tickCancel()
}

func (t *Telemetry) Clear() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.history = Entries{}
}

func (t *Telemetry) recordValues() {
	t.RecordEntry(Entry{
		Kind: CompletedTasksLocal,
		Y:    float64(t.numCompletedLocal.Load()),
	})
	t.RecordEntry(Entry{
		Kind: CompletedTasksRemote,
		Y:    float64(t.numCompletedRemote.Load()),
	})
	t.RecordEntry(Entry{
		Kind: DelegatedTasks,
		Y:    float64(t.numDelegated.Load()),
	})
	t.RecordEntry(Entry{
		Kind: RunningTasks,
		Y:    float64(t.numRunning.Load()),
	})
	t.RecordEntry(Entry{
		Kind: QueuedTasks,
		Y:    float64(t.numQueued.Load()) / float64(t.queueCapacity) * 10.0,
	})
}

func (t *Telemetry) incQueued() {
	t.numQueued.Inc()
}

func (t *Telemetry) decQueued() {
	t.numQueued.Dec()
}

func (t *Telemetry) incRunning() {
	t.numRunning.Inc()
}

func (t *Telemetry) decRunning() {
	t.numRunning.Dec()
	t.numCompletedLocal.Inc()
}

func (t *Telemetry) incDelegated() {
	t.numDelegated.Inc()
}

func (t *Telemetry) decDelegated() {
	t.numDelegated.Dec()
	t.numCompletedRemote.Inc()
}

type Entries []Entry

// Entries returns a slice of all the entries currently in the history buffer.
func (t *Telemetry) Entries() (entries Entries) {
	t.mu.Lock()
	defer t.mu.Unlock()
	return append(Entries{}, t.history...)
}

type TelemetryConfig struct {
	Enabled        bool
	RecordInterval time.Duration
	HistoryLen     int64
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
			return t.After(begin) || t.Equal(begin)
		})
	}
	if end.IsZero() {
		endIndex = len(e) - 1
	} else {
		endIndex = sort.Search(len(e), func(i int) bool {
			t := e[i].X
			return t.After(end) || t.Equal(end)
		})
	}
	return e[startIndex:endIndex]
}

func (e Entries) LinearRegression() (alpha, beta float64) {
	if len(e) < 2 {
		return 0, 0
	}
	times := make([]float64, 0, len(e))
	values := make([]float64, 0, len(e))

	for i, v := range e {
		// assumes evenly spaced samples
		times = append(times, float64(i))
		values = append(values, v.Y)
	}
	return stat.LinearRegression(times, values, nil, false)
}

func (e Entries) ToXYs() (xys plotter.XYs) {
	if len(e) == 0 {
		return
	}
	xys = make(plotter.XYs, len(e))
	for i, v := range e {
		xys[i] = plotter.XY{
			X: float64(time.Duration(v.X.UnixNano()).Milliseconds()),
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

func (e Entries) EWMA(halfLife time.Duration) (entries Entries) {
	entries = make(Entries, len(e))
	avg := ewma.NewEwma(halfLife)
	for i, entry := range e {
		avg.Update(entry.Y, entry.X)
		entries[i] = Entry{
			X: entry.X,
			Y: avg.Current,
		}
	}
	return
}
