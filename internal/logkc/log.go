// Package logkc contains project-wide logging configuration and tools
package logkc

import (
	"fmt"
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
		timeBuf = []byte{'[', '0', '0', '0', number[0]}
	case 2:
		timeBuf = []byte{'[', '0', '0', number[0], number[1]}
	case 3:
		timeBuf = []byte{'[', '0', number[0], number[1], number[2]}
	case 4:
		timeBuf = []byte{'[', number[0], number[1], number[2], number[3]}
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

func New(component types.Component, ops ...logOption) *zap.SugaredLogger {
	options := LogOptions{
		outputPaths:      []string{"stdout"},
		errorOutputPaths: []string{"stderr"},
		logLevel:         zapcore.InfoLevel,
	}
	options.Apply(ops...)
	color := component.Color()
	conf := zap.Config{
		Level:             zap.NewAtomicLevelAt(options.logLevel),
		Development:       false,
		DisableCaller:     false,
		DisableStacktrace: true,
		Sampling:          nil,
		Encoding:          "console",
		OutputPaths:       options.outputPaths,
		ErrorOutputPaths:  options.errorOutputPaths,
		EncoderConfig: zapcore.EncoderConfig{
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
		},
	}
	l, err := conf.Build()
	if err != nil {
		panic(err)
	}
	s := l.Sugar().Named(component.ShortName())
	if options.name != "" {
		s = s.Named(options.name)
	}
	s.Infof(color.Add("Starting %s"), component.Name())
	return s
}

// type logContextKeyType struct{}

// var logContextKey logContextKeyType

// func ContextWithLog(
// 	ctx context.Context,
// 	log *zap.SugaredLogger,
// ) context.Context {
// 	return context.WithValue(ctx, logContextKey, log)
// }

// func LogFromContext(ctx context.Context) *zap.SugaredLogger {
// 	if log, ok := ctx.Value(logContextKey).(*zap.SugaredLogger); ok {
// 		return log
// 	}
// 	panic("No logger stored in the given context")
// }

func PrintHeader() {
	if zapkc.UseColor {
		fmt.Println(BigAsciiTextColored)
	} else {
		fmt.Println(BigAsciiText)
	}
}

type logProvider struct{}

var MetadataProvider logProvider

func (logProvider) Key() meta.MetadataKey {
	return mdkeys.LogKey
}

func (logProvider) InitialValue(ctx meta.Context) interface{} {
	return New(ctx.Component())
}

func (logProvider) Marshal(i interface{}) string {
	return ""
}

func (logProvider) Unmarshal(s string) interface{} {
	return nil
}
