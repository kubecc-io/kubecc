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
	assert.Equal(t, info.ActionOpt(), Compile)
	assert.Equal(t, info.InputArgIndex, 9)
	assert.Equal(t, info.OutputArgIndex, 7)
	assert.Equal(t, info.FlagIndexMap["-o"], 6)
	assert.Equal(t, info.FlagIndexMap["-c"], 8)
}

func TestSetActionOpt(t *testing.T) {
	os.Args = strings.Split(`gcc -Werror -g -O2 -MD -W -Wall -o src/test.o -c src/test.c`, " ")
	info := NewArgsInfoFromOS(log)
	info.Parse()
	assert.Equal(t, info.ActionOpt(), Compile)
	info.SetActionOpt(GenAssembly)
	assert.Equal(t, info.ActionOpt(), GenAssembly)
	info.SetActionOpt(Preprocess)
	assert.Equal(t, info.ActionOpt(), Preprocess)

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
