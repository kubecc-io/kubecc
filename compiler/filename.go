package main

import (
	"path/filepath"
	"strings"
)

var sourceExtensions *StringSet = NewStringSet(
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

func isSourceFile(f string) bool {
	return sourceExtensions.Contains(filepath.Ext(f))
}

func shouldRunLocal(f string) bool {
	basename := filepath.Base(f)
	if strings.HasPrefix(basename, "conftest.") ||
		strings.HasPrefix(basename, "tmp.conftest.") {
		return true
	}
	return false
}

func replaceExtension(f string, newExt string) string {
	ext := filepath.Ext(f)
	return f[:-len(ext)] + newExt
}
