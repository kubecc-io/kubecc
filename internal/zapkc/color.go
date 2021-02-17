// Package zapkc contains internal code copied from zap
// that we need to adjust slightly but can't import
package zapkc

import (
	"fmt"
	"os"
	"strconv"

	"go.uber.org/zap/zapcore"
	"golang.org/x/crypto/ssh/terminal"
)

var UseColor = true

// Foreground colors.
const (
	Black termColor = iota + 30
	Red
	Green
	Yellow
	Blue
	Magenta
	Cyan
	White
)

var NoColor noColor

// Color represents a text color.
type Color interface {
	Add(string) string
}

type termColor uint8

// Add adds the coloring to the given string.
func (c termColor) Add(s string) string {
	if !UseColor {
		return s
	}
	return fmt.Sprintf("\x1b[%dm%s\x1b[0m", uint8(c), s)
}

type noColor struct{}

func (c noColor) Add(s string) string {
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
)

func init() {
	if value, ok := os.LookupEnv("KUBECC_LOG_COLOR"); ok {
		b, err := strconv.ParseBool(value)
		if err != nil {
			fmt.Printf("Invalid value for KUBECC_LOG_COLOR: %s\n", value)
		} else {
			UseColor = b
		}
	} else {
		UseColor = terminal.IsTerminal(int(os.Stdout.Fd()))
	}
	for level, color := range _levelToColor {
		_levelToLowercaseColorString[level] = color.Add(level.String())
		_levelToCapitalColorString[level] = color.Add(level.CapitalString()[:4])
	}
}
