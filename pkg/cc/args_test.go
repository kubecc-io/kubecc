package cc

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

var (
	log *zap.Logger
)

func init() {
	log = zap.NewNop()
}

func BenchmarkParse(b *testing.B) {
	os.Args = strings.Split(`gcc -Werror -g -O2 -MD -W -Wall -o src/test.o -c src/test.c`, " ")
	for i := 0; i < b.N; i++ {
		info := NewArgsInfoFromOS(log)
		info.Parse()
	}
}

func TestParse(t *testing.T) {
	os.Args = strings.Split(`gcc -Werror -g -O2 -MD -W -Wall -o src/test.o -c src/test.c`, " ")
	info := NewArgsInfoFromOS(log)
	info.Parse()
	assert.Equal(t, Compile, info.ActionOpt())
	assert.Equal(t, 9, info.InputArgIndex)
	assert.Equal(t, 7, info.OutputArgIndex)
	assert.Equal(t, 6, info.FlagIndexMap["-o"])
	assert.Equal(t, 8, info.FlagIndexMap["-c"])
}

func TestSetActionOpt(t *testing.T) {
	os.Args = strings.Split(`gcc -Werror -g -O2 -MD -W -Wall -o src/test.o -c src/test.c`, " ")
	info := NewArgsInfoFromOS(log)
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

	info := NewArgsInfoFromOS(log)
	info.Parse()
	info.SubstitutePreprocessorOptions()
	assert.Equal(t,
		os.Args[1:],
		info.Args,
	)

	os.Args = strings.Split(`gcc -Werror -g -O2 -MD -Wp,-X -Wp,-Y -Wp,-MD,path -o src/test.o -c src/test.c`, " ")

	info = NewArgsInfoFromOS(log)
	info.Parse()
	info.SubstitutePreprocessorOptions()
	assert.Equal(t,
		strings.Split(`-Werror -g -O2 -MD -X -Y -MF path -o src/test.o -c src/test.c`, " "),
		info.Args,
	)

	os.Args = strings.Split(`gcc -Werror -g -O2 -MD -Wp,-X -Wp,-Y,-YY -Wp,-MD,path,-Z,-ZZ -o src/test.o -c src/test.c`, " ")

	info = NewArgsInfoFromOS(log)
	info.Parse()
	info.SubstitutePreprocessorOptions()
	assert.Equal(t,
		strings.Split(`-Werror -g -O2 -MD -X -Y -YY -MF path -Z -ZZ -o src/test.o -c src/test.c`, " "),
		info.Args,
	)

	os.Args = strings.Split(`gcc -Werror -g -O2 -MD -Wp,-X -Wp,-Y,-YY -Wp,-MD,path,-MMD,path2 -o src/test.o -c src/test.c`, " ")

	info = NewArgsInfoFromOS(log)
	info.Parse()
	info.SubstitutePreprocessorOptions()
	assert.Equal(t,
		strings.Split(`-Werror -g -O2 -MD -X -Y -YY -MF path -MF path2 -o src/test.o -c src/test.c`, " "),
		info.Args,
	)
}

func TestReplaceOutputPath(t *testing.T) {
	oldArgs := os.Args
	defer func() {
		os.Args = oldArgs
	}()
	os.Args = strings.Split(`gcc -Werror -g -O2 -MD -W -Wall -o src/test.o -c src/test.c`, " ")

	info := NewArgsInfoFromOS(log)
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

	info := NewArgsInfoFromOS(log)
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

	info := NewArgsInfoFromOS(log)
	info.Parse()
	info.RemoveLocalArgs()
	assert.Equal(t,
		strings.Split(`-o src/test.o -c src/test.c`, " "),
		info.Args,
	)

}

func TestPrependLanguageFlag(t *testing.T) {
	oldArgs := os.Args
	defer func() {
		os.Args = oldArgs
	}()
	os.Args = strings.Split(`gcc -o src/test.o -c src/test.c`, " ")

	info := NewArgsInfoFromOS(log)
	info.Parse()
	info.ReplaceInputPath("-")
	assert.Equal(t,
		strings.Split(`-x c -o src/test.o -c -`, " "),
		info.Args,
	)

	os.Args = strings.Split(`gcc -o src/test.o -c src/test.cpp`, " ")

	info = NewArgsInfoFromOS(log)
	info.Parse()
	info.ReplaceInputPath("-")
	assert.Equal(t,
		strings.Split(`-x c++ -o src/test.o -c -`, " "),
		info.Args,
	)
}
