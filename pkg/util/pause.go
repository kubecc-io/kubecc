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

func DefaultPaused(paused bool) PauseControllerOption {
	return func(o *PauseControllerOptions) {
		o.defaultPaused = paused
	}
}

func NewPauseController(opts ...PauseControllerOption) *PauseController {
	options := PauseControllerOptions{}
	options.Apply(opts...)

	return &PauseController{
		paused: options.defaultPaused,
		pause:  sync.NewCond(&sync.Mutex{}),
	}
}

func (p *PauseController) Pause() {
	p.pause.L.Lock()
	p.paused = true
	p.pause.L.Unlock()
}

func (p *PauseController) Resume() {
	p.pause.L.Lock()
	p.paused = false
	p.pause.L.Unlock()
	p.pause.Signal()
}

func (p *PauseController) CheckPaused() {
	p.pause.L.Lock()
	for p.paused {
		p.pause.Wait()
	}
	p.pause.L.Unlock()
}
