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
	"context"
	"os"
	"strings"

	"github.com/cobalt77/kubecc/internal/logkc"
	"github.com/cobalt77/kubecc/pkg/identity"
	"github.com/cobalt77/kubecc/pkg/meta"
	"github.com/cobalt77/kubecc/pkg/types"
	. "github.com/onsi/ginkgo"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap/zapcore"
)

var ctx context.Context

func init() {
	ctx = meta.NewContext(
		meta.WithProvider(identity.Component, meta.WithValue(types.TestComponent)),
		meta.WithProvider(identity.UUID),
		meta.WithProvider(logkc.Logger, meta.WithValue(logkc.New(types.TestComponent,
			logkc.WithLogLevel(zapcore.WarnLevel)))),
	)
}

var _ = Describe("CC Arg Parser", func() {
	Measure("basic command parsing", func(b Benchmarker) {
		os.Args = strings.Split(`gcc -Werror -g -O2 -MD -W -Wall -o src/test.o -c src/test.c`, " ")
		info := NewArgParserFromOS(ctx)
		b.Time("Parse", func() {
			info.Parse()
		})
		assert.Equal(GinkgoT(), Compile, info.ActionOpt())
		assert.Equal(GinkgoT(), 9, info.InputArgIndex)
		assert.Equal(GinkgoT(), 7, info.OutputArgIndex)
		assert.Equal(GinkgoT(), 6, info.FlagIndexMap["-o"])
		assert.Equal(GinkgoT(), 8, info.FlagIndexMap["-c"])
	}, 1000)
	Measure("changing the action opt", func(b Benchmarker) {
		os.Args = strings.Split(`gcc -Werror -g -O2 -MD -W -Wall -o src/test.o -c src/test.c`, " ")
		info := NewArgParserFromOS(ctx)
		info.Parse()
		assert.Equal(GinkgoT(), Compile, info.ActionOpt())
		b.Time("Set action opt", func() {
			info.SetActionOpt(GenAssembly)
		})
		assert.Equal(GinkgoT(), GenAssembly, info.ActionOpt())
		b.Time("Set action opt", func() {
			info.SetActionOpt(Preprocess)
		})
		assert.Equal(GinkgoT(), Preprocess, info.ActionOpt())
	}, 1000)
	Measure("Substituting preprocessor options", func(b Benchmarker) {
		oldArgs := os.Args
		defer func() {
			os.Args = oldArgs
		}()
		os.Args = strings.Split(`gcc -Werror -g -O2 -MD -W -Wall -o src/test.o -c src/test.c`, " ")

		info := NewArgParserFromOS(ctx)
		info.Parse()
		b.Time("Configure Preprocessor Options", func() {
			info.ConfigurePreprocessorOptions()
		})
		assert.Equal(GinkgoT(), append(os.Args[1:], "-fdirectives-only"),
			info.Args,
		)

		os.Args = strings.Split(`gcc -Werror -g -O2 -MD -Wp,-X -Wp,-Y -Wp,-MD,path -o src/test.o -c src/test.c`, " ")

		info = NewArgParserFromOS(ctx)
		info.Parse()
		b.Time("Configure Preprocessor Options", func() {
			info.ConfigurePreprocessorOptions()
		})
		assert.Equal(GinkgoT(), strings.Split(`-Werror -g -O2 -MD -X -Y -MD -MF path -o src/test.o -c src/test.c -fdirectives-only`, " "),
			info.Args,
		)

		os.Args = strings.Split(`gcc -Werror -g -O2 -MD -Wp,-X -Wp,-Y,-YY -Wp,-MD,path,-Z,-ZZ -o src/test.o -c src/test.c`, " ")

		info = NewArgParserFromOS(ctx)
		info.Parse()
		b.Time("Configure Preprocessor Options", func() {
			info.ConfigurePreprocessorOptions()
		})
		assert.Equal(GinkgoT(), strings.Split(`-Werror -g -O2 -MD -X -Y -YY -MD -MF path -Z -ZZ -o src/test.o -c src/test.c -fdirectives-only`, " "),
			info.Args,
		)

		os.Args = strings.Split(`gcc -Werror -g -O2 -MD -Wp,-X -Wp,-Y,-YY -Wp,-MD,path,-MMD,path2 -o src/test.o -c src/test.c`, " ")

		info = NewArgParserFromOS(ctx)
		info.Parse()
		b.Time("Configure Preprocessor Options", func() {
			info.ConfigurePreprocessorOptions()
		})
		assert.Equal(GinkgoT(), strings.Split(`-Werror -g -O2 -MD -X -Y -YY -MD -MF path -MMD -MF path2 -o src/test.o -c src/test.c -fdirectives-only`, " "),
			info.Args,
		)
	}, 1000)
	Measure("Replacing output paths", func(b Benchmarker) {
		oldArgs := os.Args
		defer func() {
			os.Args = oldArgs
		}()
		os.Args = strings.Split(`gcc -Werror -g -O2 -MD -W -Wall -o src/test.o -c src/test.c`, " ")

		info := NewArgParserFromOS(ctx)
		info.Parse()
		b.Time("Replace output path", func() {
			info.ReplaceOutputPath("-")
		})
		assert.Equal(GinkgoT(), strings.Split(`-Werror -g -O2 -MD -W -Wall -o - -c src/test.c`, " "),
			info.Args,
		)
		b.Time("Replace output path", func() {
			info.ReplaceOutputPath("-")
		})
		assert.Equal(GinkgoT(), strings.Split(`-Werror -g -O2 -MD -W -Wall -o - -c src/test.c`, " "),
			info.Args,
		)
		b.Time("Replace output path", func() {
			info.ReplaceOutputPath("src/test.o")
		})
		assert.Equal(GinkgoT(), strings.Split(`-Werror -g -O2 -MD -W -Wall -o src/test.o -c src/test.c`, " "),
			info.Args,
		)
		b.Time("Replace output path", func() {
			info.ReplaceOutputPath("src/test.o")
		})
		assert.Equal(GinkgoT(), strings.Split(`-Werror -g -O2 -MD -W -Wall -o src/test.o -c src/test.c`, " "),
			info.Args,
		)
	}, 1000)
	Measure("Replacing input paths", func(b Benchmarker) {
		oldArgs := os.Args
		defer func() {
			os.Args = oldArgs
		}()
		os.Args = strings.Split(`gcc -Werror -g -O2 -MD -W -Wall -o src/test.o -c src/test.c`, " ")

		info := NewArgParserFromOS(ctx)
		info.Parse()
		b.Time("Replace input path", func() {
			info.ReplaceInputPath("-")
		})
		assert.Equal(GinkgoT(), strings.Split(`-x c -Werror -g -O2 -MD -W -Wall -o src/test.o -c -`, " "),
			info.Args,
		)
		b.Time("Replace input path", func() {
			info.ReplaceInputPath("-")
		})
		assert.Equal(GinkgoT(), strings.Split(`-x c -Werror -g -O2 -MD -W -Wall -o src/test.o -c -`, " "),
			info.Args,
		)
		b.Time("Replace input path", func() {
			info.ReplaceInputPath("src/test.c")
		})
		assert.Equal(GinkgoT(), strings.Split(`-x c -Werror -g -O2 -MD -W -Wall -o src/test.o -c src/test.c`, " "),
			info.Args,
		)
		b.Time("Replace input path", func() {
			info.ReplaceInputPath("src/test.c")
		})
		assert.Equal(GinkgoT(), strings.Split(`-x c -Werror -g -O2 -MD -W -Wall -o src/test.o -c src/test.c`, " "),
			info.Args,
		)
	}, 1000)
	Measure("Removing local-only arguments", func(b Benchmarker) {
		oldArgs := os.Args
		defer func() {
			os.Args = oldArgs
		}()
		os.Args = strings.Split(`gcc -Wp,a,b -MD -L test -Ltest -l test -ltest -Da=b -I. -I test -D a=b -o src/test.o -c src/test.c`, " ")

		info := NewArgParserFromOS(ctx)
		info.Parse()
		b.Time("Remove local arguments", func() {
			info.RemoveLocalArgs()
		})
		assert.Equal(GinkgoT(), strings.Split(`-o src/test.o -c src/test.c -fpreprocessed`, " "),
			info.Args,
		)
	}, 1000)
	Measure("Prepending the language flag", func(b Benchmarker) {
		oldArgs := os.Args
		defer func() {
			os.Args = oldArgs
		}()
		os.Args = strings.Split(`gcc -o src/test.o -c src/test.c`, " ")

		info := NewArgParserFromOS(ctx)
		info.Parse()
		b.Time("Prepend language flag", func() {
			info.ReplaceInputPath("-")
		})
		assert.Equal(GinkgoT(), strings.Split(`-x c -o src/test.o -c -`, " "),
			info.Args,
		)

		os.Args = strings.Split(`gcc -o src/test.o -c src/test.cpp`, " ")

		info = NewArgParserFromOS(ctx)
		info.Parse()
		b.Time("Prepend language flag", func() {
			info.ReplaceInputPath("-")
		})
		assert.Equal(GinkgoT(), strings.Split(`-x c++ -o src/test.o -c -`, " "),
			info.Args,
		)
	}, 1000)
})
