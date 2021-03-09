// Package logkc contains project-wide logging configuration and tools
package logkc

import (
	"context"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/cobalt77/kubecc/internal/zapkc"
	"github.com/cobalt77/kubecc/pkg/meta"
	"github.com/cobalt77/kubecc/pkg/meta/mdkeys"
	"github.com/cobalt77/kubecc/pkg/types"
	"go.uber.org/atomic"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var (
	BigAsciiText = `    __         __                  
   / /____  __/ /_  ___  __________
  / //_/ / / / __ \/ _ \/ ___/ ___/
 / ,< / /_/ / /_/ /  __/ /__/ /__  
/_/|_|\__,_/_.___/\___/\___/\___/  `
	BigAsciiTextColored = strings.Join([]string{
		"\x1b[33m    __         __\x1b[0m                  ",
		"\x1b[33m   / /____  __/ /_  ___  \x1b[0m\x1b[34m__________\x1b[0m",
		"\x1b[33m  / //_/ / / / __ \\/ _ \\\x1b[0m\x1b[34m/ ___/ ___/\x1b[0m",
		"\x1b[33m / ,< / /_/ / /_/ /  __/ \x1b[0m\x1b[34m/__/ /__  \x1b[0m",
		"\x1b[33m/_/|_|\\__,_/_.___/\\___/\x1b[0m\x1b[34m\\___/\\___/  \x1b[0m\n",
	}, "\n")

	startTime = atomic.NewInt64(time.Now().Unix())
)

func formatTime(t time.Time, enc zapcore.PrimitiveArrayEncoder) {
	now := t.Unix()
	var timeBuf []byte
	elapsed := now - startTime.Load()
	number := strconv.AppendInt([]byte{}, elapsed, 10)
	switch len(number) {
	case 1:
		timeBuf = []byte{'[', '0', '0', number[0], ']'}
	case 2:
		timeBuf = []byte{'[', '0', number[0], number[1], ']'}
	case 3:
		timeBuf = []byte{'[', number[0], number[1], number[2], ']'}
	default:
		timeBuf = append([]byte{'['}, number...)
	}
	enc.AppendByteString(timeBuf)
}

type LogOptions struct {
	outputPaths      []string
	errorOutputPaths []string
	logLevel         zapcore.Level
	name             string
	writer           io.Writer
}
type logOption func(*LogOptions)

func (o *LogOptions) Apply(opts ...logOption) {
	for _, op := range opts {
		op(o)
	}
}

func WithOutputPaths(paths []string) logOption {
	return func(opts *LogOptions) {
		opts.outputPaths = paths
	}
}

func WithErrorOutputPaths(paths []string) logOption {
	return func(opts *LogOptions) {
		opts.errorOutputPaths = paths
	}
}

func WithLogLevel(level zapcore.Level) logOption {
	return func(opts *LogOptions) {
		opts.logLevel = level
	}
}

func WithName(name string) logOption {
	return func(opts *LogOptions) {
		opts.name = name
	}
}

func WithWriter(writer io.Writer) logOption {
	return func(opts *LogOptions) {
		opts.writer = writer
	}
}

func New(component types.Component, ops ...logOption) *zap.SugaredLogger {
	options := LogOptions{
		outputPaths:      []string{"stdout"},
		errorOutputPaths: []string{"stderr"},
		logLevel:         zapcore.DebugLevel,
	}
	options.Apply(ops...)
	color := component.Color()

	encoderConfig := zapcore.EncoderConfig{
		MessageKey:       "M",
		LevelKey:         "L",
		TimeKey:          "T",
		NameKey:          "N",
		CallerKey:        "C",
		FunctionKey:      "",
		StacktraceKey:    "S",
		LineEnding:       "\n",
		EncodeLevel:      zapkc.CapitalColorLevelEncoder,
		EncodeTime:       formatTime,
		EncodeCaller:     zapkc.ShortCallerEncoder,
		EncodeName:       zapkc.NameEncoder(color),
		EncodeDuration:   zapcore.SecondsDurationEncoder,
		ConsoleSeparator: " ",
	}

	var logger *zap.Logger
	if options.writer != nil {
		ws := zapcore.Lock(zapcore.AddSync(options.writer))
		encoder := zapcore.NewConsoleEncoder(encoderConfig)
		core := zapcore.NewCore(encoder, ws,
			zap.NewAtomicLevelAt(options.logLevel))
		logger = zap.New(core)
	} else {
		conf := zap.Config{
			Level:             zap.NewAtomicLevelAt(options.logLevel),
			Development:       false,
			DisableCaller:     false,
			DisableStacktrace: true,
			Sampling:          nil,
			Encoding:          "console",
			OutputPaths:       options.outputPaths,
			ErrorOutputPaths:  options.errorOutputPaths,
			EncoderConfig:     encoderConfig,
		}
		l, err := conf.Build()
		if err != nil {
			panic(err)
		}
		logger = l
	}
	s := logger.Sugar().Named(component.ShortName())
	if options.name != "" {
		s = s.Named(options.name)
	}
	s.Infof(color.Add("Starting %s"), component.Name())
	return s
}

func PrintHeader() {
	if zapkc.UseColor {
		fmt.Println(BigAsciiTextColored)
	} else {
		fmt.Println(BigAsciiText)
	}
}

type logProvider struct{}

var Logger logProvider

func (logProvider) Key() meta.MetadataKey {
	return mdkeys.LogKey
}

func (logProvider) InitialValue(ctx context.Context) interface{} {
	return New(meta.Component(ctx))
}

func (logProvider) Marshal(i interface{}) string {
	return ""
}

func (logProvider) Unmarshal(s string) (interface{}, error) {
	return nil, nil
}
