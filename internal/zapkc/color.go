/*
Copyright 2021 The Kubecc Authors.

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU General Public License for more details.

You should have received a copy of the GNU General Public License
along with this program.  If not, see <http://www.gnu.org/licenses/>.
*/

// Package zapkc contains internal code copied from zap
// that we need to adjust slightly but can't import
package zapkc

import (
	"fmt"
	"os"
	"strconv"

	"go.uber.org/zap/zapcore"
	"golang.org/x/term"
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
		UseColor = term.IsTerminal(int(os.Stdout.Fd()))
	}
	for level, color := range _levelToColor {
		_levelToLowercaseColorString[level] = color.Add(level.String())
		_levelToCapitalColorString[level] = color.Add(level.CapitalString()[:4])
	}
}
