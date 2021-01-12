package cc

import (
	"errors"
	"path/filepath"
	"strings"
)

var sourceExtensions map[string]string = map[string]string{
	".i":   "c",
	".ii":  "c++",
	".c":   "c",
	".cc":  "c++",
	".cpp": "c++",
	".CPP": "c++",
	".cxx": "c++",
	".cp":  "c++",
	".c++": "c++",
	".C":   "c++",
	".m":   "objective-c",
	".mm":  "objective-c++",
	".mi":  "objective-c",
	".mii": "objective-c++",
	".M":   "objective-c++",
	".s":   "assembler",
	".S":   "assembler",
	".go":  "go",
}

// IsSourceFile returns true if the given file is a
// source file recognized by GCC, otherwise false.
func IsSourceFile(f string) bool {
	_, ok := sourceExtensions[filepath.Ext(f)]
	return ok
}

func SourceFileLanguage(f string) (string, error) {
	if lang, ok := sourceExtensions[filepath.Ext(f)]; ok {
		return lang, nil
	}
	return "", errors.New("Unknown source file extension")
}

// ShouldRunLocal returns true if the given file
// should be compiled locally as a special case.
func ShouldRunLocal(f string) bool {
	basename := filepath.Base(f)
	if strings.HasPrefix(basename, "conftest.") ||
		strings.HasPrefix(basename, "tmp.conftest.") {
		return true
	}
	return false
}

// ReplaceExtension replaces the file extension of the given path.
func ReplaceExtension(path string, newExt string) string {
	ext := filepath.Ext(path)
	return path[:len(path)-len(ext)] + newExt
}
