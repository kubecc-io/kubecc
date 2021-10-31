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

package test

import (
	"errors"
	"path/filepath"
	"strings"
	"time"

	"github.com/kubecc-io/kubecc/pkg/types"
)

// SampleQuerier is a querier that will return predefined values for some common
// compiler toolchains
type SampleQuerier struct{}

func (q SampleQuerier) IsPicDefault(compiler string) (bool, error) {
	switch filepath.Base(compiler) {
	case "x86_64-linux-gnu-gcc-10", "x86_64-linux-gnu-g++-10":
		return true, nil
	case "x86_64-linux-gnu-gcc-9", "x86_64-linux-gnu-g++-9":
		return true, nil
	case "clang":
		return false, nil
	}
	return false, errors.New("Unknown compiler")
}

func (q SampleQuerier) IsPieDefault(compiler string) (bool, error) {
	switch filepath.Base(compiler) {
	case "x86_64-linux-gnu-gcc-10", "x86_64-linux-gnu-g++-10":
		return true, nil
	case "x86_64-linux-gnu-gcc-9", "x86_64-linux-gnu-g++-9":
		return true, nil
	case "clang":
		return false, nil
	}
	return false, errors.New("Unknown compiler")
}

func (q SampleQuerier) TargetArch(compiler string) (string, error) {
	switch filepath.Base(compiler) {
	case "x86_64-linux-gnu-gcc-10", "x86_64-linux-gnu-g++-10":
		return "x86_64", nil
	case "x86_64-linux-gnu-gcc-9", "x86_64-linux-gnu-g++-9":
		return "x86_64", nil
	case "clang":
		return "x86_64", nil
	}
	return "", errors.New("Unknown compiler")
}

func (q SampleQuerier) Version(compiler string) (string, error) {
	switch filepath.Base(compiler) {
	case "x86_64-linux-gnu-gcc-10", "x86_64-linux-gnu-g++-10":
		return "10", nil
	case "x86_64-linux-gnu-gcc-9", "x86_64-linux-gnu-g++-9":
		return "9", nil
	case "clang":
		return "11.0.0", nil
	}
	return "", errors.New("Unknown compiler")
}

func (q SampleQuerier) Kind(compiler string) (types.ToolchainKind, error) {
	switch base := filepath.Base(compiler); {
	case strings.Contains(base, "clang"):
		return types.Clang, nil
	case strings.Contains(base, "++"):
		return types.Gnu, nil
	case strings.Contains(base, "cc"):
		return types.Gnu, nil
	}
	return 0, errors.New("Unknown compiler")
}

func (q SampleQuerier) Lang(compiler string) (types.ToolchainLang, error) {
	switch base := filepath.Base(compiler); {
	case strings.Contains(base, "clang"):
		return types.Multi, nil
	case strings.Contains(base, "++"):
		return types.CXX, nil
	case strings.Contains(base, "cc"):
		return types.C, nil
	}
	return 0, errors.New("Unknown compiler")
}

var sampleTime = time.Now()

func (q SampleQuerier) ModTime(compiler string) (time.Time, error) {
	return sampleTime, nil
}
