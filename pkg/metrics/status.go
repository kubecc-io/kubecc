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

package metrics

import (
	"context"
	"sync"
	"time"

	"github.com/kubecc-io/kubecc/pkg/meta"
	"go.uber.org/zap"
)

type activeCondition struct {
	condition StatusConditions
	msgs      []string
}

type StatusController struct {
	srvCtx        context.Context
	lg            *zap.SugaredLogger
	conditions    map[context.Context]*activeCondition
	statusLock    sync.Mutex
	changed       *sync.Cond
	cancelPending context.CancelFunc
}

func (sm *StatusController) BeginInitialize(srvCtx context.Context) {
	sm.srvCtx = srvCtx
	sm.lg = meta.Log(srvCtx)
	sm.changed = sync.NewCond(&sm.statusLock)
	sm.conditions = make(map[context.Context]*activeCondition)

	// Not parented to srvCtx, that logic is handled in ApplyCondition
	ctx, ca := context.WithCancel(context.Background())
	sm.ApplyCondition(ctx, StatusConditions_Pending, "Component is initializing")
	sm.cancelPending = ca
}

func (sm *StatusController) EndInitialize() {
	sm.cancelPending()
}

func (sm *StatusController) WaitForStatusChanged() {
	sm.statusLock.Lock()
	sm.changed.Wait()
	sm.statusLock.Unlock()
}

func (sm *StatusController) ApplyCondition(
	while context.Context,
	cond StatusConditions,
	msgs ...string,
) {
	sm.lg.With(
		"cond", cond.String(),
	).Debug("Applying status condition")
	sm.statusLock.Lock()
	sm.conditions[while] = &activeCondition{
		condition: cond,
		msgs:      msgs,
	}
	sm.statusLock.Unlock()
	sm.changed.Broadcast()

	go func() {
		// Display status updates for long-running conditions
		go func() {
			tick := time.NewTicker(30 * time.Second)
			defer tick.Stop()
			for {
				select {
				case <-while.Done():
					return
				case <-sm.srvCtx.Done():
					return
				case <-tick.C:
					sm.lg.With(
						"cond", cond.String(),
						"msgs", msgs,
					).Warn("Still waiting on status condition")
				}
			}
		}()
		select {
		case <-while.Done():
		case <-sm.srvCtx.Done(): // cancel all conditions when server is done
		}
		sm.lg.With(
			"cond", cond.String(),
		).Debug("Clearing status condition")
		sm.statusLock.Lock()
		delete(sm.conditions, while)
		sm.statusLock.Unlock()
		sm.changed.Broadcast()
	}()
}

// requires sm.statusLock to be locked
func (sm *StatusController) health() *Health {
	overall := OverallStatus_Ready
	if sm.srvCtx.Err() != nil {
		overall = OverallStatus_Unavailable
	}
	msgs := []string{}

	for _, v := range sm.conditions {
		msgs = append(msgs, v.condition.FormatAll(v.msgs...)...)
		switch v.condition {
		case StatusConditions_Pending:
			// we don't necessarily care about other status conditions while
			// the component is in the pending state
			overall = OverallStatus_Initializing
			msgs = v.msgs
			goto done
		case StatusConditions_MissingOptionalComponent:
			if overall < OverallStatus_Degraded {
				overall = OverallStatus_Degraded
			}
		case StatusConditions_MissingCriticalComponent,
			StatusConditions_InvalidConfiguration,
			StatusConditions_InternalError:
			if overall < OverallStatus_Unavailable {
				overall = OverallStatus_Unavailable
			}
		}
	}
done:
	return &Health{
		Status:   overall,
		Messages: msgs,
	}
}

func (sm *StatusController) GetHealth() *Health {
	sm.statusLock.Lock()
	defer sm.statusLock.Unlock()
	return sm.health()
}

func (sm *StatusController) StreamHealthUpdates() chan *Health {
	ch := make(chan *Health)
	go func() {
		defer sm.lg.Debug("Status controller is done")
		defer close(ch)
		for {
			if sm.srvCtx.Err() != nil {
				return
			}
			sm.statusLock.Lock()
			h := sm.health()
			ch <- h
			sm.lg.With(
				zap.Object("health", h),
			).Debug("Health changed")
			sm.changed.Wait()
			sm.statusLock.Unlock()
		}
	}()
	return ch
}
