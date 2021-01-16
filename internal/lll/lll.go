// Package lll contains project-wide logging configuration and tools
package lll

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"go.uber.org/atomic"
	"go.uber.org/zap"
	"go.uber.org/zap/buffer"
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
		"\x1b[33m/_/|_|\\__,_/_.___/\\___/\x1b[0m\x1b[34m\\___/\\___/  \x1b[0m",
	}, "\n")

	startTime *atomic.Int64

	hextable = [...]rune{
		'0', '1', '2', '3', '4', '5', '6', '7', '8', '9',
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
	enc.AppendString(
		string([]rune{
			'[', '+',
			hextable[since/10],
			hextable[since%10],
			']',
		}),
	)
}

func formatLevel(l zapcore.Level, enc zapcore.PrimitiveArrayEncoder) {
	enc.AppendString(l.CapitalString()[:4])
}

type LogOptions struct {
	outputPaths      []string
	errorOutputPaths []string
}

var (
	defaultLogOptions = LogOptions{
		outputPaths:      []string{"stdout"},
		errorOutputPaths: []string{"stderr"},
	}
)

type LogOption interface {
	apply(*LogOptions)
}

type funcLogOption struct {
	f func(*LogOptions)
}

func (fso *funcLogOption) apply(ops *LogOptions) {
	fso.f(ops)
}

func WithOutputPaths(paths []string) LogOption {
	return &funcLogOption{
		func(lo *LogOptions) {
			lo.outputPaths = paths
		},
	}
}

func WithErrorOutputPaths(paths []string) LogOption {
	return &funcLogOption{
		func(lo *LogOptions) {
			lo.errorOutputPaths = paths
		},
	}
}

func Setup(component string, ops ...LogOption) {
	options := defaultLogOptions
	for _, op := range ops {
		op.apply(&options)
	}

	startTime = atomic.NewInt64(time.Now().Unix())
	conf := zap.Config{
		Level:             zap.NewAtomicLevelAt(zapcore.DebugLevel),
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
			EncodeLevel:      CapitalColorLevelEncoder,
			EncodeTime:       formatTime,
			EncodeCaller:     ShortCallerEncoder,
			EncodeName:       FullNameEncoder,
			EncodeDuration:   zapcore.SecondsDurationEncoder,
			ConsoleSeparator: " ",
		},
	}
	l, err := conf.Build()
	if err != nil {
		panic(err)
	}
	globalLog = l.Named(component).Sugar()
	loadFunctions()
}

func PrintHeader() {
	fmt.Println(BigAsciiText)
	globalLog.Info(Green.Add("Starting component"))
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

// Color represents a text color.
type Color uint8

// Add adds the coloring to the given string.
func (c Color) Add(s string) string {
	return fmt.Sprintf("\x1b[%dm%s\x1b[0m", uint8(c), s)
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
	} else if len(name) < 16 {
		spaces := make([]byte, 16-len(name))
		for i := range spaces {
			spaces[i] = ' '
		}
		enc.AppendByteString(spaces)
	}
	enc.AppendByteString(name[:])
}

// FullNameEncoder serializes the logger name as-is.
func FullNameEncoder(loggerName string, enc zapcore.PrimitiveArrayEncoder) {
	enc.AppendString(Green.Add(loggerName))
}
