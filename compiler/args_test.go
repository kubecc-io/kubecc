package main

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSubstitutePreprocessorOptions(t *testing.T) {
	oldArgs := os.Args
	defer func() {
		os.Args = oldArgs
	}()
	os.Args = strings.Split(`gcc -Werror -g -O2 -MD -W -Wall -o src/test.o -c src/test.c`, " ")

	info := NewArgsInfo()
	info.Parse()
	info.SubstitutePreprocessorOptions()
	assert.Equal(t,
		os.Args[1:],
		info.Args,
	)

	os.Args = strings.Split(`gcc -Werror -g -O2 -MD -Wp,-X -Wp,-Y -Wp,-MD,path -o src/test.o -c src/test.c`, " ")

	info = NewArgsInfo()
	info.Parse()
	info.SubstitutePreprocessorOptions()
	assert.Equal(t,
		strings.Split(`-Werror -g -O2 -MD -X -Y -MF path -o src/test.o -c src/test.c`, " "),
		info.Args,
	)

	os.Args = strings.Split(`gcc -Werror -g -O2 -MD -Wp,-X -Wp,-Y,-YY -Wp,-MD,path,-Z,-ZZ -o src/test.o -c src/test.c`, " ")

	info = NewArgsInfo()
	info.Parse()
	info.SubstitutePreprocessorOptions()
	assert.Equal(t,
		strings.Split(`-Werror -g -O2 -MD -X -Y -YY -MF path -Z -ZZ -o src/test.o -c src/test.c`, " "),
		info.Args,
	)

	os.Args = strings.Split(`gcc -Werror -g -O2 -MD -Wp,-X -Wp,-Y,-YY -Wp,-MD,path,-MMD,path2 -o src/test.o -c src/test.c`, " ")

	info = NewArgsInfo()
	info.Parse()
	info.SubstitutePreprocessorOptions()
	assert.Equal(t,
		strings.Split(`-Werror -g -O2 -MD -X -Y -YY -MF path -MF path2 -o src/test.o -c src/test.c`, " "),
		info.Args,
	)
}
