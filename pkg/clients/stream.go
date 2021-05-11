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

package clients

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/kubecc-io/kubecc/internal/zapkc"
	"github.com/kubecc-io/kubecc/pkg/meta"
	"github.com/kubecc-io/kubecc/pkg/metrics"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/apimachinery/pkg/util/clock"
	"k8s.io/apimachinery/pkg/util/wait"
)

type StreamHandler interface {
	TryConnect() (grpc.ClientStream, error)
	HandleStream(grpc.ClientStream) error
	Target() string
}

type OnConnectedEventHandler interface {
	OnConnected()
}

type OnLostConnectionEventHandler interface {
	OnLostConnection()
}

type OnConnectFailedEventHandler interface {
	OnConnectFailed()
}

// StreamManager is used to manage automatic reconnect and backoff logic
// for gRPC streams, as well as providing a means to handle connection
// events.
type StreamManager struct {
	StreamManagerOptions
	ctx        context.Context
	handler    StreamHandler
	backoffMgr wait.BackoffManager
	immediate  chan struct{}
	immLock    *sync.Mutex
}

type EventKind uint

const (
	LogConnected EventKind = 1 << iota
	LogConnectionFailed
	LogConnectionLost
	LogStreamFinished

	LogNone     EventKind = 0
	LogDefaults EventKind = LogConnected | LogConnectionFailed | LogConnectionLost
)

type StreamManagerOptions struct {
	logEvents  EventKind
	statusCtrl *metrics.StatusController
	statusKind StatusCtrlKind
}
type StreamManagerOption func(*StreamManagerOptions)

func (o *StreamManagerOptions) Apply(opts ...StreamManagerOption) {
	for _, op := range opts {
		op(o)
	}
}

func WithLogEvents(events EventKind) StreamManagerOption {
	return func(opts *StreamManagerOptions) {
		opts.logEvents = events
	}
}

type StatusCtrlKind int

const (
	Optional StatusCtrlKind = iota
	Required
)

func WithStatusCtrl(stat *metrics.StatusController, kind StatusCtrlKind) StreamManagerOption {
	return func(o *StreamManagerOptions) {
		o.statusCtrl = stat
		o.statusKind = kind
	}
}

func makeBackoffMgr() wait.BackoffManager {
	return wait.NewExponentialBackoffManager(
		500*time.Millisecond, // Initial
		5*time.Second,        // Max
		10*time.Second,       // Reset
		math.Sqrt2,           // Backoff factor
		0.25,                 // Jitter factor
		clock.RealClock{},
	)
}

// todo: unit tests here

func NewStreamManager(
	ctx context.Context,
	handler StreamHandler,
	opts ...StreamManagerOption,
) *StreamManager {
	options := StreamManagerOptions{
		logEvents: LogDefaults,
	}
	options.Apply(opts...)

	return &StreamManager{
		StreamManagerOptions: options,
		ctx:                  ctx,
		handler:              handler,
		backoffMgr:           makeBackoffMgr(),
		immediate:            make(chan struct{}),
		immLock:              &sync.Mutex{},
	}
}

// TryImmediately will immediately invoke TryConnect.
// This should only be used when you are reasonably certain the connection
// to the server will succeed, but you may be stuck in a long backoff timer.
// This function has the side effect of resetting the backoff manager to its
// defaults, but only if a backoff timer is currently active. If the
// backoff timer is not currently active, this function will do nothing.
func (sm *StreamManager) TryImmediately() {
	sm.immLock.Lock()
	defer sm.immLock.Unlock()
	if sm.immediate != nil {
		close(sm.immediate)
	}
	sm.immediate = nil
}

// this must only be called by Run() which should be running in a separate
// goroutine.
func (sm *StreamManager) waitBackoff() {
	lg := meta.Log(sm.ctx)
	lg.Debug("Backing off")
	sm.immLock.Lock()
	sm.immediate = make(chan struct{})
	sm.immLock.Unlock()

	defer func() {
		sm.immLock.Lock()
		defer sm.immLock.Unlock()
		if sm.immediate != nil {
			close(sm.immediate)
		}
		sm.immediate = nil
	}()

	select {
	case <-sm.ctx.Done():
	case <-sm.backoffMgr.Backoff().C():
		lg.Debug("Backoff timer completed")
	case <-sm.immediate:
		lg.Debug(zapkc.Yellow.Add("Requested to try connecting immediately"))
		// We need to reset the backoff manager since its timer is most likely
		// still waiting.
		sm.backoffMgr = makeBackoffMgr()
	}
}

func (sm *StreamManager) applyCondition() (context.Context, context.CancelFunc) {
	condCtx, condCancel := context.WithCancel(context.Background())
	if sm.statusCtrl != nil {
		var cond metrics.StatusConditions
		switch sm.statusKind {
		case Optional:
			cond = metrics.StatusConditions_MissingOptionalComponent
		case Required:
			cond = metrics.StatusConditions_MissingCriticalComponent
		}
		sm.statusCtrl.ApplyCondition(condCtx, cond,
			fmt.Sprintf("Attempting to connect to %s", sm.handler.Target()))
	}
	return condCtx, condCancel
}

func (sm *StreamManager) Run() {
	lg := meta.Log(sm.ctx)

	condCtx, cancelCond := sm.applyCondition()
	defer func() {
		cancelCond()
	}()

	for {
		if stream, err := sm.handler.TryConnect(); err != nil {
			if e, ok := sm.handler.(OnConnectFailedEventHandler); ok {
				e.OnConnectFailed()
			}

			if sm.logEvents&LogConnectionFailed != 0 {
				lg.With(
					zap.String("err", status.Convert(err).Message()),
					zap.String("target", sm.handler.Target()),
				).Warn(zapkc.Red.Add("Failed to connect"))
			}
			if errors.Is(err, context.Canceled) || status.Code(err) == codes.Canceled {
				// If the context was canceled, we are done
				return
			}
			if condCtx.Err() != nil {
				condCtx, cancelCond = sm.applyCondition()
			}
			sm.waitBackoff()
		} else {
			cancelCond()
			if e, ok := sm.handler.(OnConnectedEventHandler); ok {
				e.OnConnected()
			}
			if sm.logEvents&LogConnected != 0 {
				lg.Infof(zapkc.Green.Add("Connected to %s"), sm.handler.Target())
			}
			err := sm.handler.HandleStream(stream)
			if condCtx.Err() != nil {
				condCtx, cancelCond = sm.applyCondition()
			}
			if e, ok := sm.handler.(OnLostConnectionEventHandler); ok {
				e.OnLostConnection()
			}
			if err != nil {
				if errors.Is(err, context.Canceled) || status.Code(err) == codes.Canceled {
					// If the context was canceled, we are done
					return
				}
				if sm.logEvents&LogConnectionLost != 0 {
					lg.With(
						zap.Error(err),
						zap.String("target", sm.handler.Target()),
					).Error(zapkc.Red.Add("Connection lost, Attempting to reconnect"))
				}
				sm.waitBackoff()
			} else {
				if sm.logEvents&LogStreamFinished != 0 {
					lg.Debug("Stream finished")
				}
				return
			}
		}
	}
}
