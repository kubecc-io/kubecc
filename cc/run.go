package cc

import (
	"bytes"
	"compress/gzip"
	"os"
	"os/exec"

	log "github.com/sirupsen/logrus"
)

type runOptions struct {
	compress bool
}

var defaultSchedulerOptions = runOptions{
	compress: false,
}

type RunOption interface {
	apply(*runOptions)
}

type funcRunOption struct {
	f func(*runOptions)
}

func (fso *funcRunOption) apply(ops *runOptions) {
	fso.f(ops)
}

func WithCompressOutput() RunOption {
	return &funcRunOption{
		func(ro *runOptions) {
			ro.compress = true
		},
	}
}

// Run the compiler with the current args.
func Run(info *ArgsInfo, opts ...RunOption) ([]byte, error) {
	options := defaultSchedulerOptions
	for _, op := range opts {
		op.apply(&options)
	}

	info.ReplaceOutputPath("-")
	info.SetCompiler("/usr/bin/gcc")
	cmd := exec.Command(info.Compiler, info.Args...)
	log.Trace(cmd.Args)
	buf := new(bytes.Buffer)
	var gz *gzip.Writer
	if options.compress {
		gz := gzip.NewWriter(buf)
		cmd.Stdout = gz
	} else {
		cmd.Stdout = buf
	}
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	if err != nil {
		return nil, err
	}
	gz.Close()
	return buf.Bytes(), nil
}
