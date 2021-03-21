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
