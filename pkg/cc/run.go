package cc

import (
	"bytes"
	"compress/gzip"
	"io"
	"io/ioutil"
	"os"
	"os/exec"

	log "github.com/sirupsen/logrus"
)

type runOptions struct {
	compress  bool
	logOutput io.Writer
	env       []string
	workDir   string
}

var defaultSchedulerOptions = runOptions{
	compress:  false,
	logOutput: ioutil.Discard,
	env:       nil,
	workDir:   "",
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

func WithLogOutput(w io.Writer) RunOption {
	return &funcRunOption{
		func(ro *runOptions) {
			ro.logOutput = w
		},
	}
}

func WithEnv(env []string) RunOption {
	return &funcRunOption{
		func(ro *runOptions) {
			ro.env = env
		},
	}
}

func WithWorkDir(dir string) RunOption {
	return &funcRunOption{
		func(ro *runOptions) {
			ro.workDir = dir
		},
	}
}

// Run the compiler with the current args.
func Run(info *ArgsInfo, opts ...RunOption) ([]byte, error) {
	options := defaultSchedulerOptions
	for _, op := range opts {
		op.apply(&options)
	}
	info.SetCompiler("/bin/gcc")

	// If preprocessing we can write to '-', but otherwise
	// we need to write to a file. The assembler needs a
	// seekable file to write to.
	if info.ActionOpt() == Preprocess {
		if info.OutputArgIndex != -1 {
			log.Trace("Replacing output path with '-'")
			info.ReplaceOutputPath("-")
		}
		cmd := exec.Command(info.Compiler, info.Args...)
		if options.env != nil {
			cmd.Env = options.env
		}
		if options.workDir != "" {
			cmd.Dir = options.workDir
		}
		log.Trace(cmd.Args)
		buf := new(bytes.Buffer)
		var gz *gzip.Writer
		if options.compress {
			gz := gzip.NewWriter(buf)
			cmd.Stdout = gz
		} else {
			cmd.Stdout = buf
		}
		cmd.Stderr = options.logOutput
		err := cmd.Run()
		if err != nil {
			return nil, err
		}
		gz.Close()
		return buf.Bytes(), nil
	}

	tmp, err := ioutil.TempFile("", "kubecc")
	defer os.Remove(tmp.Name())
	if err != nil {
		log.WithError(err).Fatal("Can't create temporary files")
	}
	if info.OutputArgIndex != -1 {
		log.Trace("Replacing output path")
		err = info.ReplaceOutputPath(tmp.Name())
		if err != nil {
			log.WithError(err).Error("Error replacing output path")
			return nil, err
		}
	}
	cmd := exec.Command(info.Compiler, info.Args...)
	if options.env != nil {
		cmd.Env = options.env
	}
	if options.workDir != "" {
		cmd.Dir = options.workDir
	}
	cmd.Stdout = options.logOutput
	cmd.Stderr = options.logOutput
	err = cmd.Run()
	if err != nil {
		log.WithError(err).Error("Compiler error")
		return nil, err
	}
	buf := new(bytes.Buffer)
	reader := io.Reader(tmp)
	if options.compress {
		reader, err = gzip.NewReader(tmp)
		if err != nil {
			log.WithError(err).Error("Error creating gzip reader")
			return nil, err
		}
	}
	_, err = io.Copy(buf, reader)
	if err != nil {
		log.WithError(err).Error("Copy error")
		return nil, err
	}
	return buf.Bytes(), nil
}
