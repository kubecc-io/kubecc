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

package util

import (
	"context"
	"time"

	"k8s.io/apimachinery/pkg/util/wait"
)

// RunPeriodic runs each function given in funcs (in parallel) every time
// the channel receives a value. It stops when either the channel is closed
// or the context is canceled.
func RunOnNotify(ctx context.Context, c <-chan struct{}, funcs ...func()) {
	go func() {
		for {
			select {
			case _, open := <-c:
				if !open {
					return
				}
				for _, fn := range funcs {
					go fn()
				}
			case <-ctx.Done():
				return
			}
		}
	}()
}

// RunPeriodic runs each function given in funcs (in parallel) every given
// duration until the context is done. A jitter factor can be provided, or
// set to a negative number to disable jitter. If now is true, the functions
// will be run once immediately before starting the timer.
func RunPeriodic(
	ctx context.Context,
	duration time.Duration,
	factor float64,
	now bool,
	funcs ...func(),
) {
	if now {
		for _, fn := range funcs {
			go fn()
		}
	}
	go wait.JitterUntil(func() {
		for _, fn := range funcs {
			go fn()
		}
	}, duration, factor, false, ctx.Done())
}
