package cc

import (
	"path/filepath"
	"strings"

	types "github.com/cobalt77/kube-cc/types"
)

var sourceExtensions *types.StringSet = types.NewStringSet(
	".i",
	".ii",
	".c",
	".cc",
	".cpp",
	".cxx",
	".cp",
	".c++",
	".C",
	".m",
	".mm",
	".mi",
	".mii",
	".M",
	".s",
	".S",
)

// IsSourceFile returns true if the given file is a
// source file recognized by GCC, otherwise false.
func IsSourceFile(f string) bool {
	return sourceExtensions.Contains(filepath.Ext(f))
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
	return path[:-len(ext)] + newExt
}
