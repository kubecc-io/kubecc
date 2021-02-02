// Package lll contains project-wide logging configuration and tools
package lll

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"go.uber.org/atomic"
	"go.uber.org/zap"
	"go.uber.org/zap/buffer"
	"go.uber.org/zap/zapcore"
	"golang.org/x/crypto/ssh/terminal"
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
	colorTable = [10]colorer{
		NoColor{},
		Blue,
		Blue,
		Green,
		Green,
		Yellow,
		Yellow,
		Red,
		Red,
		Red,
	}

	globalLog *zap.SugaredLogger
	useColor  bool
)

func formatTime(t time.Time, enc zapcore.PrimitiveArrayEncoder) {
	now := t.Unix()
	since := int(now - startTime.Load())
	startTime.Store(now)
	if since > 99 {
		since = 99
	}
	if !useColor {
		enc.AppendString(string([]rune{
			'[', '+',
			numTable[since/10],
			numTable[since%10],
			']',
		}))
	} else {
		enc.AppendString(fmt.Sprintf("[+%s]", colorTable[since/10].Add(
			string([]rune{
				numTable[since/10],
				numTable[since%10],
			}))))
	}
}

func formatLevel(l zapcore.Level, enc zapcore.PrimitiveArrayEncoder) {
	enc.AppendString(l.CapitalString()[:4])
}

type LogOptions struct {
	color            bool
	outputPaths      []string
	errorOutputPaths []string
	logLevel         zapcore.Level
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

func WithColor(color bool) logOption {
	return func(opts *LogOptions) {
		opts.color = color
	}
}

func NewFromContext(
	ctx context.Context,
	component Component,
	ops ...logOption,
) context.Context {
	options := LogOptions{
		color:            useColor,
		outputPaths:      []string{"stdout"},
		errorOutputPaths: []string{"stderr"},
		logLevel:         zapcore.DebugLevel,
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
			MessageKey:    "M",
			LevelKey:      "L",
			TimeKey:       "T",
			NameKey:       "N",
			CallerKey:     "C",
			FunctionKey:   "",
			StacktraceKey: "S",
			LineEnding:    "\n",
			EncodeLevel:   CapitalColorLevelEncoder,
			EncodeTime:    formatTime,
			EncodeCaller:  ShortCallerEncoder,
			EncodeName: func(loggerName string, enc zapcore.PrimitiveArrayEncoder) {
				enc.AppendString(fmt.Sprintf("[%s]", color.Add(loggerName)))
			},
			EncodeDuration:   zapcore.SecondsDurationEncoder,
			ConsoleSeparator: " ",
		},
	}
	l, err := conf.Build()
	if err != nil {
		panic(err)
	}
	s := l.Sugar().Named(component.ShortName())
	s.Infof(color.Add("Starting %s"), component.String())
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
	if useColor {
		fmt.Println(BigAsciiTextColored)
	} else {
		fmt.Println(BigAsciiText)
	}
}

// internal code copied from zap below

// Foreground colors.
const (
	Black Color = iota + 30
	Red
	Green
	Yellow
	Blue
	Magenta
	Cyan
	White
)

type colorer interface {
	Add(string) string
}

// Color represents a text color.
type Color uint8

// Add adds the coloring to the given string.
func (c Color) Add(s string) string {
	if !useColor {
		return s
	}
	return fmt.Sprintf("\x1b[%dm%s\x1b[0m", uint8(c), s)
}

type NoColor struct{}

func (c NoColor) Add(s string) string {
	return s
}

var (
	_levelToColor = map[zapcore.Level]Color{
		zapcore.DebugLevel:  Magenta,
		zapcore.InfoLevel:   Blue,
		zapcore.WarnLevel:   Yellow,
		zapcore.ErrorLevel:  Red,
		zapcore.DPanicLevel: Red,
		zapcore.PanicLevel:  Red,
		zapcore.FatalLevel:  Red,
	}
	_unknownLevelColor = Red

	_levelToLowercaseColorString = make(map[zapcore.Level]string, len(_levelToColor))
	_levelToCapitalColorString   = make(map[zapcore.Level]string, len(_levelToColor))
	_pool                        = buffer.NewPool()
)

func init() {
	useColor = terminal.IsTerminal(int(os.Stdout.Fd()))
	for level, color := range _levelToColor {
		_levelToLowercaseColorString[level] = color.Add(level.String())
		_levelToCapitalColorString[level] = color.Add(level.CapitalString()[:4])
	}
}

// CapitalColorLevelEncoder serializes a Level to an all-caps string and adds color.
// For example, InfoLevel is serialized to "INFO" and colored blue.
func CapitalColorLevelEncoder(l zapcore.Level, enc zapcore.PrimitiveArrayEncoder) {
	s, ok := _levelToCapitalColorString[l]
	if !ok {
		s = _unknownLevelColor.Add(l.CapitalString()[:4])
	}
	enc.AppendString(s)
}

func ShortCallerEncoder(caller zapcore.EntryCaller, enc zapcore.PrimitiveArrayEncoder) {
	// name := [16]byte{}

	// idx := strings.LastIndex(caller.File, "/")
	// substr := []byte(caller.File[idx+1 : len(caller.File)-3])
	// lineNum := strconv.Itoa(caller.Line)
	// dest := name[:]
	// if len(substr) < len(name)-len(lineNum) {
	// 	for i := 0; i < len(name)-len(substr); i++ {
	// 		name[i] = ' '
	// 	}
	// 	dest = name[:len(name)-len(substr)]
	// }
	// copy(dest, substr)
	// copy(name[len(name)-len(lineNum)+1:], lineNum)

	name := []byte(fmt.Sprintf("%s:%d", filepath.Base(caller.File), caller.Line))
	if len(name) > 16 {
		name[0] = '+'
	} else if len(name) <= 16 {
		spaces := make([]byte, 16-len(name)+1)
		for i := range spaces {
			spaces[i] = ' '
		}
		enc.AppendByteString(spaces)
	}
	enc.AppendByteString(name[:])
}
