package zapkc

import (
	"bytes"
	"fmt"
	"strings"

	"go.uber.org/zap/zapcore"
)

const (
	// 16 char buffer for filenames
	filenameBufferLen = 16
)

var spaceBuffer = bytes.Repeat([]byte{' '}, filenameBufferLen)

// CapitalColorLevelEncoder serializes a Level to an all-caps string and adds color.
// For example, InfoLevel is serialized to "INFO" and colored blue.
func CapitalColorLevelEncoder(l zapcore.Level, enc zapcore.PrimitiveArrayEncoder) {
	s, ok := _levelToCapitalColorString[l]
	if !ok {
		s = _unknownLevelColor.Add(l.CapitalString()[:4])
	}
	enc.AppendString(s)
}

// FormatEntryCaller formats a zapcore.EntryCaller to be displayed
// optimally given a maximum length. See test cases for examples.
//
// Finally, my bachelor's degree in CS is put to use. Knew it would
// come in handy some day.
func FormatEntryCaller(path string, maxLen int) []byte {
	if maxLen <= 0 {
		panic(fmt.Sprintf("Invalid maxLen: %d", maxLen))
	}
	bytes := make([]byte, maxLen)
	slashIdx := strings.LastIndexByte(path, '/')
	dotIdx := strings.LastIndexByte(path, '.')
	var basenameLen int
	if slashIdx == -1 {
		basenameLen = len(path)
	} else {
		basenameLen = len(path) - slashIdx - 1
	}
	switch {
	case len(path) <= maxLen:
		// len(path/filename.go:line) is <= max
		copy(bytes, spaceBuffer[:maxLen-len(path)])
		copy(bytes[maxLen-len(path):], path)
	case slashIdx != -1 && basenameLen+2 <= maxLen:
		// len(p/filename.go:line) is <= max
		start := maxLen - basenameLen - 2
		copy(bytes, spaceBuffer[:start])
		bytes[start] = path[0]
		bytes[start+1] = '/'
		copy(bytes[start+2:], path[slashIdx+1:])
	case basenameLen <= maxLen:
		// len(filename.go:line) is <= max
		copy(bytes, spaceBuffer[:maxLen-basenameLen])
		copy(bytes[maxLen-basenameLen:], path[slashIdx+1:])
	case basenameLen > maxLen && dotIdx > len(path)-basenameLen+1:
		// len(filename.go:line) is > max
		// contains ".", should match ".go:###"
		copy(bytes, path[len(path)-basenameLen:dotIdx-1])
		bytes[len(bytes)-(len(path)-dotIdx)-1] = '+'
		copy(bytes[len(bytes)-(len(path)-dotIdx):], path[dotIdx:])
	default:
		// len(path) > max; unexpected format
		copy(bytes, path)
		bytes[len(bytes)-1] = '+'
	}
	return bytes
}

func ShortCallerEncoder(caller zapcore.EntryCaller, enc zapcore.PrimitiveArrayEncoder) {
	enc.AppendByteString(FormatEntryCaller(caller.TrimmedPath(), filenameBufferLen))
	enc.AppendByteString([]byte{}) // Add an extra space before the log message
}

func NameEncoder(color Color) zapcore.NameEncoder {
	return func(loggerName string, enc zapcore.PrimitiveArrayEncoder) {
		text := color.Add(loggerName)
		buf := make([]byte, len(text)+2)
		buf[0] = '['
		buf[len(buf)-1] = ']'
		copy(buf[1:len(buf)-1], text)
		enc.AppendByteString(buf)
	}
}
