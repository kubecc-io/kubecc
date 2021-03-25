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

import "sync"

// PauseController is an embeddable helper struct that allows an object to
// provide a means to "pause" and "resume" itself.
type PauseController struct {
	paused bool
	pause  *sync.Cond
}

type PauseControllerOptions struct {
	defaultPaused bool
}

type PauseControllerOption func(*PauseControllerOptions)

func (o *PauseControllerOptions) Apply(opts ...PauseControllerOption) {
	for _, op := range opts {
		op(o)
	}
}

// DefaultPaused starts the controller paused. This should be used place of
// simply calling Pause right away to prevent race conditions.
func DefaultPaused(paused bool) PauseControllerOption {
	return func(o *PauseControllerOptions) {
		o.defaultPaused = paused
	}
}

// NewPauseController creates and initializes a new pause controller.
func NewPauseController(opts ...PauseControllerOption) *PauseController {
	options := PauseControllerOptions{}
	options.Apply(opts...)

	return &PauseController{
		paused: options.defaultPaused,
		pause:  sync.NewCond(&sync.Mutex{}),
	}
}

// Pause triggers an implementation-defined state where the object should
// cease its normal operations until unpaused.
func (p *PauseController) Pause() {
	p.pause.L.Lock()
	p.paused = true
	p.pause.L.Unlock()
}

// Resume removes the paused state and unblocks any blocked calls to
// CheckPaused.
func (p *PauseController) Resume() {
	p.pause.L.Lock()
	p.paused = false
	p.pause.L.Unlock()
	p.pause.Broadcast()
}

// CheckPaused blocks while the paused state is active. The implementation
// should call CheckPaused before and/or after a "pausable" section of code
// to enable the pause functionality. When paused, all calls to CheckPaused
// will block until Resume is called, after which CheckPaused will not block
// until Pause is called again.
func (p *PauseController) CheckPaused() {
	p.pause.L.Lock()
	for p.paused {
		p.pause.Wait()
	}
	p.pause.L.Unlock()
}
