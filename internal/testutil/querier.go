package testutil

import (
	"errors"
	"path/filepath"
	"strings"
	"time"

	"github.com/cobalt77/kubecc/pkg/types"
)

type MockQuerier struct{}

func (q MockQuerier) IsPicDefault(compiler string) (bool, error) {
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

func (q MockQuerier) TargetArch(compiler string) (string, error) {
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

func (q MockQuerier) Version(compiler string) (string, error) {
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

func (q MockQuerier) Kind(compiler string) (types.ToolchainKind, error) {
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

func (q MockQuerier) Lang(compiler string) (types.ToolchainLang, error) {
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

var sampleTime = time.Now()

func (q MockQuerier) ModTime(compiler string) (time.Time, error) {
	return sampleTime, nil
}
