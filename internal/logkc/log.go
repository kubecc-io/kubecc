// Package logkc contains project-wide logging configuration and tools
package logkc

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/cobalt77/kubecc/internal/zapkc"
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

	startTime *atomic.Int64

	numTable = [10]rune{
		'0', '1', '2', '3', '4', '5', '6', '7', '8', '9',
	}
	colorTable = [10]zapkc.Color{
		zapkc.NoColor,
		zapkc.Blue,
		zapkc.Blue,
		zapkc.Green,
		zapkc.Green,
		zapkc.Yellow,
		zapkc.Yellow,
		zapkc.Red,
		zapkc.Red,
		zapkc.Red,
	}

	globalLog *zap.SugaredLogger
)

func formatTime(t time.Time, enc zapcore.PrimitiveArrayEncoder) {
	now := t.Unix()
	since := int(now - startTime.Load())
	startTime.Store(now)
	if since > 99 {
		since = 99
	}
	if !zapkc.UseColor {
		enc.AppendString(string([]rune{
			'[', '+',
			numTable[since/10],
			numTable[since%10],
			']',
		}))
	} else {
		text := colorTable[since/10].Add(
			string([]rune{
				numTable[since/10],
				numTable[since%10],
			}))
		buf := make([]byte, len(text)+3)
		buf[0] = '['
		buf[1] = '+'
		buf[len(buf)-1] = ']'
		copy(buf[2:len(buf)-1], text)
		enc.AppendByteString(buf)
	}
}

func formatLevel(l zapcore.Level, enc zapcore.PrimitiveArrayEncoder) {
	enc.AppendString(l.CapitalString()[:4])
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

func NewFromContext(
	ctx context.Context,
	component types.Component,
	ops ...logOption,
) context.Context {
	options := LogOptions{
		outputPaths:      []string{"stdout"},
		errorOutputPaths: []string{"stderr"},
		logLevel:         zapcore.InfoLevel,
	}
	options.Apply(ops...)
	color := component.Color()
	startTime = atomic.NewInt64(time.Now().Unix())
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
	return ContextWithLog(ctx, s)
}

func ContextWithLog(
	ctx context.Context,
	log *zap.SugaredLogger,
) context.Context {
	return context.WithValue(ctx, "log", log)
}

func LogFromContext(ctx context.Context) *zap.SugaredLogger {
	if log, ok := ctx.Value("log").(*zap.SugaredLogger); ok {
		return log
	}
	panic("No logger stored in the given context")
}

func PrintHeader() {
	if zapkc.UseColor {
		fmt.Println(BigAsciiTextColored)
	} else {
		fmt.Println(BigAsciiText)
	}
}