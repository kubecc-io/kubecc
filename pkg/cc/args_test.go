package cc

import (
	"os"
	"strings"
	"testing"

	"github.com/cobalt77/kubecc/internal/lll"
	"github.com/stretchr/testify/assert"
)

func init() {
	lll.Setup("TEST")
}

func BenchmarkParse(b *testing.B) {
	os.Args = strings.Split(`gcc -Werror -g -O2 -MD -W -Wall -o src/test.o -c src/test.c`, " ")
	for i := 0; i < b.N; i++ {
		info := NewArgsInfoFromOS()
		info.Parse()
	}
}

func TestParse(t *testing.T) {
	os.Args = strings.Split(`gcc -Werror -g -O2 -MD -W -Wall -o src/test.o -c src/test.c`, " ")
	info := NewArgsInfoFromOS()
	info.Parse()
	assert.Equal(t, Compile, info.ActionOpt())
	assert.Equal(t, 9, info.InputArgIndex)
	assert.Equal(t, 7, info.OutputArgIndex)
	assert.Equal(t, 6, info.FlagIndexMap["-o"])
	assert.Equal(t, 8, info.FlagIndexMap["-c"])
}

func TestSetActionOpt(t *testing.T) {
	os.Args = strings.Split(`gcc -Werror -g -O2 -MD -W -Wall -o src/test.o -c src/test.c`, " ")
	info := NewArgsInfoFromOS()
	info.Parse()
	assert.Equal(t, Compile, info.ActionOpt())
	info.SetActionOpt(GenAssembly)
	assert.Equal(t, GenAssembly, info.ActionOpt())
	info.SetActionOpt(Preprocess)
	assert.Equal(t, Preprocess, info.ActionOpt())

}

func TestSubstitutePreprocessorOptions(t *testing.T) {
	oldArgs := os.Args
	defer func() {
		os.Args = oldArgs
	}()
	os.Args = strings.Split(`gcc -Werror -g -O2 -MD -W -Wall -o src/test.o -c src/test.c`, " ")

	info := NewArgsInfoFromOS()
	info.Parse()
	info.ConfigurePreprocessorOptions()
	assert.Equal(t,
		append(os.Args[1:], "-fdirectives-only"),
		info.Args,
	)

	os.Args = strings.Split(`gcc -Werror -g -O2 -MD -Wp,-X -Wp,-Y -Wp,-MD,path -o src/test.o -c src/test.c`, " ")

	info = NewArgsInfoFromOS()
	info.Parse()
	info.ConfigurePreprocessorOptions()
	assert.Equal(t,
		strings.Split(`-Werror -g -O2 -MD -X -Y -MD -MF path -o src/test.o -c src/test.c -fdirectives-only`, " "),
		info.Args,
	)

	os.Args = strings.Split(`gcc -Werror -g -O2 -MD -Wp,-X -Wp,-Y,-YY -Wp,-MD,path,-Z,-ZZ -o src/test.o -c src/test.c`, " ")

	info = NewArgsInfoFromOS()
	info.Parse()
	info.ConfigurePreprocessorOptions()
	assert.Equal(t,
		strings.Split(`-Werror -g -O2 -MD -X -Y -YY -MD -MF path -Z -ZZ -o src/test.o -c src/test.c -fdirectives-only`, " "),
		info.Args,
	)

	os.Args = strings.Split(`gcc -Werror -g -O2 -MD -Wp,-X -Wp,-Y,-YY -Wp,-MD,path,-MMD,path2 -o src/test.o -c src/test.c`, " ")

	info = NewArgsInfoFromOS()
	info.Parse()
	info.ConfigurePreprocessorOptions()
	assert.Equal(t,
		strings.Split(`-Werror -g -O2 -MD -X -Y -YY -MD -MF path -MMD -MF path2 -o src/test.o -c src/test.c -fdirectives-only`, " "),
		info.Args,
	)
}

func TestReplaceOutputPath(t *testing.T) {
	oldArgs := os.Args
	defer func() {
		os.Args = oldArgs
	}()
	os.Args = strings.Split(`gcc -Werror -g -O2 -MD -W -Wall -o src/test.o -c src/test.c`, " ")

	info := NewArgsInfoFromOS()
	info.Parse()
	info.ReplaceOutputPath("-")
	assert.Equal(t,
		strings.Split(`-Werror -g -O2 -MD -W -Wall -o - -c src/test.c`, " "),
		info.Args,
	)
	info.ReplaceOutputPath("-")
	assert.Equal(t,
		strings.Split(`-Werror -g -O2 -MD -W -Wall -o - -c src/test.c`, " "),
		info.Args,
	)
	info.ReplaceOutputPath("src/test.o")
	assert.Equal(t,
		strings.Split(`-Werror -g -O2 -MD -W -Wall -o src/test.o -c src/test.c`, " "),
		info.Args,
	)
	info.ReplaceOutputPath("src/test.o")
	assert.Equal(t,
		strings.Split(`-Werror -g -O2 -MD -W -Wall -o src/test.o -c src/test.c`, " "),
		info.Args,
	)
}

func TestReplaceInputPath(t *testing.T) {
	oldArgs := os.Args
	defer func() {
		os.Args = oldArgs
	}()
	os.Args = strings.Split(`gcc -Werror -g -O2 -MD -W -Wall -o src/test.o -c src/test.c`, " ")

	info := NewArgsInfoFromOS()
	info.Parse()
	info.ReplaceInputPath("-")
	assert.Equal(t,
		strings.Split(`-x c -Werror -g -O2 -MD -W -Wall -o src/test.o -c -`, " "),
		info.Args,
	)
	info.ReplaceInputPath("-")
	assert.Equal(t,
		strings.Split(`-x c -Werror -g -O2 -MD -W -Wall -o src/test.o -c -`, " "),
		info.Args,
	)
	info.ReplaceInputPath("src/test.c")
	assert.Equal(t,
		strings.Split(`-x c -Werror -g -O2 -MD -W -Wall -o src/test.o -c src/test.c`, " "),
		info.Args,
	)
	info.ReplaceInputPath("src/test.c")
	assert.Equal(t,
		strings.Split(`-x c -Werror -g -O2 -MD -W -Wall -o src/test.o -c src/test.c`, " "),
		info.Args,
	)
}

func TestRemoveLocalArgs(t *testing.T) {
	oldArgs := os.Args
	defer func() {
		os.Args = oldArgs
	}()
	os.Args = strings.Split(`gcc -Wp,a,b -MD -L test -Ltest -l test -ltest -Da=b -I. -I test -D a=b -o src/test.o -c src/test.c`, " ")

	info := NewArgsInfoFromOS()
	info.Parse()
	info.RemoveLocalArgs()
	assert.Equal(t,
		strings.Split(`-o src/test.o -c src/test.c -fpreprocessed`, " "),
		info.Args,
	)

}

func TestPrependLanguageFlag(t *testing.T) {
	oldArgs := os.Args
	defer func() {
		os.Args = oldArgs
	}()
	os.Args = strings.Split(`gcc -o src/test.o -c src/test.c`, " ")

	info := NewArgsInfoFromOS()
	info.Parse()
	info.ReplaceInputPath("-")
	assert.Equal(t,
		strings.Split(`-x c -o src/test.o -c -`, " "),
		info.Args,
	)

	os.Args = strings.Split(`gcc -o src/test.o -c src/test.cpp`, " ")

	info = NewArgsInfoFromOS()
	info.Parse()
	info.ReplaceInputPath("-")
	assert.Equal(t,
		strings.Split(`-x c++ -o src/test.o -c -`, " "),
		info.Args,
	)
}
