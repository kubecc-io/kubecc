package run

import (
	"context"
	"io"

	"github.com/cobalt77/kubecc/internal/logkc"
	"go.uber.org/zap"
)

type Runner interface {
	Run(compiler string) error
}

type OutputType int

type ProcessOptions struct {
	Stdout  io.Writer
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

	Context context.Context
	Log     *zap.SugaredLogger
}

func (r *RunnerOptions) Apply(opts ...RunOption) {
	for _, opt := range opts {
		opt(r)
	}
}

type RunOption func(*RunnerOptions)

func WithEnv(env []string) RunOption {
	return func(ro *RunnerOptions) {
		ro.Env = env
	}
}

func WithWorkDir(dir string) RunOption {
	return func(ro *RunnerOptions) {
		ro.WorkDir = dir
	}
}

func WithOutputStreams(stdout, stderr io.Writer) RunOption {
	return func(ro *RunnerOptions) {
		ro.Stdout = stdout
		ro.Stderr = stderr
	}
}

func WithStdin(stdin io.Reader) RunOption {
	return func(ro *RunnerOptions) {
		ro.Stdin = stdin
	}
}

func WithUidGid(uid, gid uint32) RunOption {
	return func(ro *RunnerOptions) {
		ro.UID = uid
		ro.GID = gid
	}
}

func WithOutputWriter(w io.Writer) RunOption {
	return func(ro *RunnerOptions) {
		ro.OutputWriter = w
	}
}

func InPlace(inPlace bool) RunOption {
	return func(ro *RunnerOptions) {
		ro.NoTempFile = inPlace
	}
}

func WithContext(ctx context.Context) RunOption {
	return func(ro *RunnerOptions) {
		ro.Context = ctx
		ro.Log = logkc.LogFromContext(ctx)
	}
}
