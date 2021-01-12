package run

import (
	"io"

	"github.com/cobalt77/kubecc/pkg/cc"
	"go.uber.org/zap"
)

type Runner interface {
	Run(info *cc.ArgsInfo) error
}

type OutputType int

type ProcessOptions struct {
	Stderr  io.Writer
	Stdin   io.Reader
	Env     []string
	WorkDir string
	UID     uint32
	GID     uint32
}

type ResultOptions struct {
	OutputWriter io.Writer
	NoTempFile   bool
}

type RunnerOptions struct {
	ProcessOptions
	ResultOptions

	Logger *zap.Logger
}

func (r *RunnerOptions) Apply(opts ...RunOption) {
	for _, opt := range opts {
		opt.apply(r)
	}
}

type RunOption interface {
	apply(*RunnerOptions)
}

type funcRunOption struct {
	f func(*RunnerOptions)
}

func (fso *funcRunOption) apply(ops *RunnerOptions) {
	fso.f(ops)
}

func WithLogger(logger *zap.Logger) RunOption {
	return &funcRunOption{
		func(ro *RunnerOptions) {
			ro.Logger = logger
		},
	}
}

func WithEnv(env []string) RunOption {
	return &funcRunOption{
		func(ro *RunnerOptions) {
			ro.Env = env
		},
	}
}

func WithWorkDir(dir string) RunOption {
	return &funcRunOption{
		func(ro *RunnerOptions) {
			ro.WorkDir = dir
		},
	}
}

func WithStderr(stderr io.Writer) RunOption {
	return &funcRunOption{
		func(ro *RunnerOptions) {
			ro.Stderr = stderr
		},
	}
}

func WithStdin(stdin io.Reader) RunOption {
	return &funcRunOption{
		func(ro *RunnerOptions) {
			ro.Stdin = stdin
		},
	}
}

func WithUidGid(uid, gid uint32) RunOption {
	return &funcRunOption{
		func(ro *RunnerOptions) {
			ro.UID = uid
			ro.GID = gid
		},
	}
}

func WithOutputWriter(w io.Writer) RunOption {
	return &funcRunOption{
		func(ro *RunnerOptions) {
			ro.OutputWriter = w
		},
	}
}

func InPlace(inPlace bool) RunOption {
	return &funcRunOption{
		func(ro *RunnerOptions) {
			ro.NoTempFile = inPlace
		},
	}
}
