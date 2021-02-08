package toolchains_test

import (
	"context"
	_ "embed"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/cobalt77/kubecc/internal/logkc"
	"github.com/cobalt77/kubecc/pkg/toolchains"
	"github.com/cobalt77/kubecc/pkg/types"
	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/testing/protocmp"
)

func sym(to string) *fstest.MapFile {
	return &fstest.MapFile{
		Data:    []byte(to),
		Mode:    fs.ModeSymlink,
		ModTime: time.Now(),
	}
}

func elf() *fstest.MapFile {
	return &fstest.MapFile{
		Data: []byte{'\x7F', '\x45', '\x4C', '\x46'},
		Mode: 0755,
	}
}

type mockQuerier struct{}

func (q mockQuerier) IsPicDefault(compiler string) (bool, error) {
	switch filepath.Base(os.Args[0]) {
	case "x86_64-linux-gnu-gcc-10", "x86_64-linux-gnu-g++-10":
		return true, nil
	case "x86_64-linux-gnu-gcc-9", "x86_64-linux-gnu-g++-9":
		return true, nil
	case "clang":
		return false, nil
	}
	return false, errors.New("Unknown compiler")
}

func (q mockQuerier) TargetArch(compiler string) (string, error) {
	switch filepath.Base(os.Args[0]) {
	case "x86_64-linux-gnu-gcc-10", "x86_64-linux-gnu-g++-10":
		return "x86_64-linux-gnu", nil
	case "x86_64-linux-gnu-gcc-9", "x86_64-linux-gnu-g++-9":
		return "x86_64-linux-gnu", nil
	case "clang":
		return "x86_64-pc-linux-gnu", nil
	}
	return "", errors.New("Unknown compiler")
}

func (q mockQuerier) Version(compiler string) (string, error) {
	switch filepath.Base(os.Args[0]) {
	case "x86_64-linux-gnu-gcc-10", "x86_64-linux-gnu-g++-10":
		return "10", nil
	case "x86_64-linux-gnu-gcc-9", "x86_64-linux-gnu-g++-9":
		return "9", nil
	case "clang":
		return "11.0.0", nil
	}
	return "", errors.New("Unknown compiler")
}

func (q mockQuerier) Kind(compiler string) (types.ToolchainKind, error) {
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

func (q mockQuerier) Lang(compiler string) (types.ToolchainLang, error) {
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

func TestFindToolchains(t *testing.T) {
	ctx := logkc.NewFromContext(context.Background(), types.Test)
	fs := fstest.MapFS{
		"/bin":                             sym("usr/bin"),
		"/usr/bin/gcc":                     sym("gcc-10"),
		"/usr/bin/g++":                     sym("g++-10"),
		"/usr/bin/gcc-10":                  sym("x86_64-linux-gnu-gcc-10"),
		"/usr/bin/gcc-9":                   sym("x86_64-linux-gnu-gcc-9"),
		"/usr/bin/g++-10":                  sym("x86_64-linux-gnu-g++-10"),
		"/usr/bin/g++-9":                   sym("x86_64-linux-gnu-g++-9"),
		"/usr/bin/x86_64-linux-gnu-gcc":    sym("gcc-10"),
		"/usr/bin/x86_64-linux-gnu-g++":    sym("g++-10"),
		"/usr/bin/x86_64-linux-gnu-gcc-10": elf(),
		"/usr/bin/x86_64-linux-gnu-g++-10": elf(),
		"/usr/bin/x86_64-linux-gnu-gcc-9":  elf(),
		"/usr/bin/x86_64-linux-gnu-g++-9":  elf(),
		"/usr/bin/clang":                   sym("../lib/llvm-11/bin/clang"),
		"/usr/bin/clang++":                 sym("../lib/llvm-11/bin/clang++"),
		"/usr/bin/clang++-11":              sym("../lib/llvm-11/bin/clang++"),
		"/usr/bin/clang-11":                sym("../lib/llvm-11/bin/clang"),
		"/usr/lib/llvm-11/bin/clang++":     sym("clang"),
		"/usr/lib/llvm-11/bin/clang-11":    sym("clang"),
		"/usr/lib/llvm-11/bin/clang":       elf(),
	}
	expected := map[string]*types.Toolchain{
		"/usr/bin/x86_64-linux-gnu-gcc-10": {
			Kind:       types.Gnu,
			Lang:       types.C,
			Executable: "/usr/bin/x86_64-linux-gnu-gcc-10",
			TargetArch: "x86_64",
			Version:    "10",
			PicDefault: true,
		},
		"/usr/bin/x86_64-linux-gnu-g++-10": {
			Kind:       types.Gnu,
			Lang:       types.CXX,
			Executable: "/usr/bin/x86_64-linux-gnu-g++-10",
			TargetArch: "x86_64",
			Version:    "10",
			PicDefault: true,
		},
		"/usr/bin/x86_64-linux-gnu-gcc-9": {
			Kind:       types.Gnu,
			Lang:       types.C,
			Executable: "/usr/bin/x86_64-linux-gnu-gcc-9",
			TargetArch: "x86_64",
			Version:    "9",
			PicDefault: true,
		},
		"/usr/bin/x86_64-linux-gnu-g++-9": {
			Kind:       types.Gnu,
			Lang:       types.CXX,
			Executable: "/usr/bin/x86_64-linux-gnu-g++-9",
			TargetArch: "x86_64",
			Version:    "9",
			PicDefault: true,
		},
		"/usr/lib/llvm-11/bin/clang": {
			Kind:       types.Clang,
			Lang:       types.Multi,
			Executable: "/usr/lib/llvm-11/bin/clang",
			TargetArch: "x86_64",
			Version:    "11.0.0",
			PicDefault: false,
		},
	}

	tcs := toolchains.FindToolchains(ctx,
		toolchains.WithFS(fs),
		toolchains.WithQuerier(mockQuerier{}),
	)
	tcsMap := make(map[string]*types.Toolchain)
	for _, tc := range tcs {
		tcsMap[tc.Executable] = tc
	}
	assert.Empty(t, cmp.Diff(expected, tcsMap, protocmp.Transform()))
}
