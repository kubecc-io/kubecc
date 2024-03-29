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

package toolchains

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/kubecc-io/kubecc/pkg/types"
)

// A Querier can query a compiler to determine its characteristics.
type Querier interface {
	Version(compiler string) (string, error)
	TargetArch(compiler string) (string, error)
	IsPicDefault(compiler string) (bool, error)
	IsPieDefault(compiler string) (bool, error)
	Kind(compiler string) (types.ToolchainKind, error)
	Lang(compiler string) (types.ToolchainLang, error)
	ModTime(compiler string) (time.Time, error)
}

type ExecQuerier struct{}

var picCheck = `
#if defined __PIC__ || defined __pic__       
# error                                                    
#endif                                                                          
`

var pieCheck = `
#if defined __PIE__ || defined __pie__  
# error                                                    
#endif                                                                          
`

func (q ExecQuerier) IsPicDefault(compiler string) (bool, error) {
	cmd := exec.Command(compiler, "-E", "-o", "/dev/null", "-")
	stderrBuf := new(bytes.Buffer)
	cmd.Stdin = strings.NewReader(picCheck)
	cmd.Stdout = nil
	cmd.Stderr = stderrBuf
	cmd.Env = []string{}
	err := cmd.Run()
	if err == nil {
		return false, nil
	}
	return strings.Contains(stderrBuf.String(), "#error"), nil
}

func (q ExecQuerier) IsPieDefault(compiler string) (bool, error) {
	cmd := exec.Command(compiler, "-E", "-o", "/dev/null", "-")
	stderrBuf := new(bytes.Buffer)
	cmd.Stdin = strings.NewReader(pieCheck)
	cmd.Stdout = nil
	cmd.Stderr = stderrBuf
	cmd.Env = []string{}
	err := cmd.Run()
	if err == nil {
		return false, nil
	}
	return strings.Contains(stderrBuf.String(), "#error"), nil
}

func (q ExecQuerier) TargetArch(compiler string) (string, error) {
	cmd := exec.Command(compiler, "-dumpmachine")
	stdoutBuf := new(bytes.Buffer)
	cmd.Stdin = nil
	cmd.Stdout = stdoutBuf
	cmd.Stderr = nil
	cmd.Env = []string{}
	err := cmd.Run()
	if err != nil {
		return "", err
	}
	triple := strings.Split(stdoutBuf.String(), "-")
	if len(triple) <= 2 {
		return "", errors.New("GCC returned an invalid target triple with -dumpmachine")
	}
	return strings.TrimSpace(triple[0]), nil
}

func (q ExecQuerier) Version(compiler string) (string, error) {
	cmd := exec.Command(compiler, "-dumpversion")
	stdoutBuf := new(bytes.Buffer)
	cmd.Stdin = nil
	cmd.Stdout = stdoutBuf
	cmd.Stderr = nil
	cmd.Env = []string{}
	err := cmd.Run()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(stdoutBuf.String()), nil
}

func (q ExecQuerier) Kind(compiler string) (types.ToolchainKind, error) {
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

func (q ExecQuerier) Lang(compiler string) (types.ToolchainLang, error) {
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

func (q ExecQuerier) ModTime(compiler string) (time.Time, error) {
	info, err := os.Stat(compiler)
	if err != nil {
		return time.Time{}, err
	}
	return info.ModTime(), nil
}
