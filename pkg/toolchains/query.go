package toolchains

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/cobalt77/kubecc/pkg/types"
)

type Querier interface {
	Version(compiler string) (string, error)
	TargetArch(compiler string) (string, error)
	IsPicDefault(compiler string) (bool, error)
	Kind(compiler string) (types.ToolchainKind, error)
	Lang(compiler string) (types.ToolchainLang, error)
	ModTime(compiler string) (time.Time, error)
}

type ExecQuerier struct{}

var picCheck = `
#if defined __PIC__ || defined __pic__ || defined PIC || defined pic            
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
	case strings.Contains(base, "g++"):
		return types.Gnu, nil
	case strings.Contains(base, "gcc"):
		return types.Gnu, nil
	}
	return 0, errors.New("Unknown compiler")
}

func (q ExecQuerier) Lang(compiler string) (types.ToolchainLang, error) {
	switch base := filepath.Base(compiler); {
	case strings.Contains(base, "clang"):
		return types.Multi, nil
	case strings.Contains(base, "g++"):
		return types.CXX, nil
	case strings.Contains(base, "gcc"):
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
