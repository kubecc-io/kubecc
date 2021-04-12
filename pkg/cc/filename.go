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
	".h":   "c-header",
	".H":   "c++-header",
	".hpp": "c++-header",
	".hp":  "c++-header",
	".hxx": "c++-header",
	".h++": "c++-header",
	".HPP": "c++-header",
	".tcc": "c++-header",
	".hh":  "c++-header",
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
