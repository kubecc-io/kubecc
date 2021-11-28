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
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/kubecc-io/kubecc/internal/logkc"
	"github.com/kubecc-io/kubecc/pkg/identity"
	"github.com/kubecc-io/kubecc/pkg/meta"
	"github.com/kubecc-io/kubecc/pkg/types"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gmeasure"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap/zapcore"
)

var ctx context.Context

func init() {
	ctx = meta.NewContext(
		meta.WithProvider(identity.Component, meta.WithValue(types.TestComponent)),
		meta.WithProvider(identity.UUID),
		meta.WithProvider(logkc.Logger, meta.WithValue(logkc.New(types.TestComponent,
			logkc.WithLogLevel(zapcore.ErrorLevel)))),
	)
}

var _ = Describe("CC Arg Parser", func() {
	Specify("basic command parsing", func() {
		experiment := gmeasure.NewExperiment("basic command parsing")
		for i := 0; i < 1000; i++ {
			os.Args = strings.Split(`gcc -Werror -g -O2 -MD -W -Wall -o src/test.o -c src/test.c`, " ")
			info := NewArgParserFromOS(ctx)
			experiment.MeasureDuration("Parse", func() {
				info.Parse()
			}, gmeasure.Precision(time.Microsecond))
			assert.Equal(GinkgoT(), Compile, info.ActionOpt())
			assert.Equal(GinkgoT(), 9, info.InputArgIndex)
			assert.Equal(GinkgoT(), 7, info.OutputArgIndex)
			assert.Equal(GinkgoT(), 6, info.FlagIndexMap["-o"])
			assert.Equal(GinkgoT(), 8, info.FlagIndexMap["-c"])
		}
		AddReportEntry(experiment.Name, experiment)
	})
	Specify("changing the action opt", func() {
		experiment := gmeasure.NewExperiment("changing the action opt")
		for i := 0; i < 1000; i++ {
			os.Args = strings.Split(`gcc -Werror -g -O2 -MD -W -Wall -o src/test.o -c src/test.c`, " ")
			info := NewArgParserFromOS(ctx)
			info.Parse()
			assert.Equal(GinkgoT(), Compile, info.ActionOpt())
			experiment.MeasureDuration("Set action opt", func() {
				info.SetActionOpt(GenAssembly)
			}, gmeasure.Precision(time.Microsecond))
			assert.Equal(GinkgoT(), GenAssembly, info.ActionOpt())
			experiment.MeasureDuration("Set action opt", func() {
				info.SetActionOpt(Preprocess)
			}, gmeasure.Precision(time.Microsecond))
			assert.Equal(GinkgoT(), Preprocess, info.ActionOpt())
		}
		AddReportEntry(experiment.Name, experiment)
	})
	Specify("Substituting preprocessor options", func() {
		experiment := gmeasure.NewExperiment("Substituting preprocessor options")
		for i := 0; i < 1000; i++ {
			oldArgs := os.Args
			defer func() {
				os.Args = oldArgs
			}()
			os.Args = strings.Split(`gcc -Werror -g -O2 -MD -W -Wall -o src/test.o -c src/test.c`, " ")

			info := NewArgParserFromOS(ctx)
			info.Parse()
			experiment.MeasureDuration("Configure Preprocessor Options", func() {
				info.ConfigurePreprocessorOptions()
			}, gmeasure.Precision(time.Microsecond))
			assert.Equal(GinkgoT(), os.Args[1:],
				info.Args,
			)

			os.Args = strings.Split(`gcc -Werror -g -O2 -MD -Wp,-X -Wp,-Y -Wp,-MD,path -o src/test.o -c src/test.c`, " ")

			info = NewArgParserFromOS(ctx)
			info.Parse()
			experiment.MeasureDuration("Configure Preprocessor Options", func() {
				info.ConfigurePreprocessorOptions()
			}, gmeasure.Precision(time.Microsecond))
			assert.Equal(GinkgoT(), strings.Split(`-Werror -g -O2 -MD -X -Y -MD -MF path -o src/test.o -c src/test.c`, " "),
				info.Args,
			)

			os.Args = strings.Split(`gcc -Werror -g -O2 -MD -Wp,-X -Wp,-Y,-YY -Wp,-MD,path,-Z,-ZZ -o src/test.o -c src/test.c`, " ")

			info = NewArgParserFromOS(ctx)
			info.Parse()
			experiment.MeasureDuration("Configure Preprocessor Options", func() {
				info.ConfigurePreprocessorOptions()
			}, gmeasure.Precision(time.Microsecond))
			assert.Equal(GinkgoT(), strings.Split(`-Werror -g -O2 -MD -X -Y -YY -MD -MF path -Z -ZZ -o src/test.o -c src/test.c`, " "),
				info.Args,
			)

			os.Args = strings.Split(`gcc -Werror -g -O2 -MD -Wp,-X -Wp,-Y,-YY -Wp,-MD,path,-MMD,path2 -o src/test.o -c src/test.c`, " ")

			info = NewArgParserFromOS(ctx)
			info.Parse()
			experiment.MeasureDuration("Configure Preprocessor Options", func() {
				info.ConfigurePreprocessorOptions()
			}, gmeasure.Precision(time.Microsecond))
			assert.Equal(GinkgoT(), strings.Split(`-Werror -g -O2 -MD -X -Y -YY -MD -MF path -MMD -MF path2 -o src/test.o -c src/test.c`, " "),
				info.Args,
			)
		}
		AddReportEntry(experiment.Name, experiment)
	})
	Specify("Replacing output paths", func() {
		experiment := gmeasure.NewExperiment("Replacing output paths")
		for i := 0; i < 1000; i++ {
			oldArgs := os.Args
			defer func() {
				os.Args = oldArgs
			}()
			os.Args = strings.Split(`gcc -Werror -g -O2 -MD -W -Wall -o src/test.o -c src/test.c`, " ")

			info := NewArgParserFromOS(ctx)
			info.Parse()
			experiment.MeasureDuration("Replace output path", func() {
				info.ReplaceOutputPath("-")
			}, gmeasure.Precision(time.Microsecond))
			assert.Equal(GinkgoT(), strings.Split(`-Werror -g -O2 -MD -W -Wall -o - -c src/test.c`, " "),
				info.Args,
			)
			experiment.MeasureDuration("Replace output path", func() {
				info.ReplaceOutputPath("-")
			}, gmeasure.Precision(time.Microsecond))
			assert.Equal(GinkgoT(), strings.Split(`-Werror -g -O2 -MD -W -Wall -o - -c src/test.c`, " "),
				info.Args,
			)
			experiment.MeasureDuration("Replace output path", func() {
				info.ReplaceOutputPath("src/test.o")
			}, gmeasure.Precision(time.Microsecond))
			assert.Equal(GinkgoT(), strings.Split(`-Werror -g -O2 -MD -W -Wall -o src/test.o -c src/test.c`, " "),
				info.Args,
			)
			experiment.MeasureDuration("Replace output path", func() {
				info.ReplaceOutputPath("src/test.o")
			}, gmeasure.Precision(time.Microsecond))
			assert.Equal(GinkgoT(), strings.Split(`-Werror -g -O2 -MD -W -Wall -o src/test.o -c src/test.c`, " "),
				info.Args,
			)
		}
		AddReportEntry(experiment.Name, experiment)
	})
	Specify("Replacing input paths", func() {
		experiment := gmeasure.NewExperiment("Replacing input paths")
		for i := 0; i < 1000; i++ {
			oldArgs := os.Args
			defer func() {
				os.Args = oldArgs
			}()
			os.Args = strings.Split(`gcc -Werror -g -O2 -MD -W -Wall -o src/test.o -c src/test.c`, " ")

			info := NewArgParserFromOS(ctx)
			info.Parse()
			experiment.MeasureDuration("Replace input path", func() {
				info.ReplaceInputPath("src2/test.c")
			}, gmeasure.Precision(time.Microsecond))
			assert.Equal(GinkgoT(), strings.Split(`-Werror -g -O2 -MD -W -Wall -o src/test.o -c src2/test.c -ffile-prefix-map=src2=src`, " "),
				info.Args,
			)
			experiment.MeasureDuration("Replace input path", func() {
				info.ReplaceInputPath("/test.c")
			}, gmeasure.Precision(time.Microsecond))
			// this wouldn't be a valid use of ReplaceInputPath but it shouldn't break
			assert.Equal(GinkgoT(), strings.Split(`-Werror -g -O2 -MD -W -Wall -o src/test.o -c /test.c -ffile-prefix-map=src2=src -ffile-prefix-map=/=src2`, " "),
				info.Args,
			)
			experiment.MeasureDuration("Replace input path", func() {
				info.ReplaceInputPath("test.c")
			}, gmeasure.Precision(time.Microsecond))
			// this wouldn't be a valid use of ReplaceInputPath but it shouldn't break
			assert.Equal(GinkgoT(), strings.Split(`-Werror -g -O2 -MD -W -Wall -o src/test.o -c test.c -ffile-prefix-map=src2=src -ffile-prefix-map=/=src2 -ffile-prefix-map=.=/`, " "),
				info.Args,
			)
			experiment.MeasureDuration("Replace input path", func() {
				info.ReplaceInputPath("src/test.c")
			}, gmeasure.Precision(time.Microsecond))
			// this wouldn't be a valid use of ReplaceInputPath but it shouldn't break
			assert.Equal(GinkgoT(), strings.Split(`-Werror -g -O2 -MD -W -Wall -o src/test.o -c src/test.c -ffile-prefix-map=src2=src -ffile-prefix-map=/=src2 -ffile-prefix-map=.=/ -ffile-prefix-map=src=.`, " "),
				info.Args,
			)
		}
		AddReportEntry(experiment.Name, experiment)
	})
	Specify("Removing local-only arguments", func() {
		experiment := gmeasure.NewExperiment("Removing local-only arguments")
		for i := 0; i < 1000; i++ {
			oldArgs := os.Args
			defer func() {
				os.Args = oldArgs
			}()
			os.Args = strings.Split(`gcc -Wp,a,b -MD -L test -Ltest -l test -ltest -Da=b -I. -I test -D a=b -o src/test.o -c src/test.c`, " ")

			info := NewArgParserFromOS(ctx)
			info.Parse()
			experiment.MeasureDuration("Remove local arguments", func() {
				info.RemoveLocalArgs()
			}, gmeasure.Precision(time.Microsecond))
			assert.Equal(GinkgoT(), strings.Split(`-o src/test.o -c src/test.c`, " "),
				info.Args,
			)
		}
		AddReportEntry(experiment.Name, experiment)
	})
	Specify("Prepending the language flag", func() {
		experiment := gmeasure.NewExperiment("Prepending the language flag")
		for i := 0; i < 1000; i++ {
			oldArgs := os.Args
			defer func() {
				os.Args = oldArgs
			}()
			os.Args = strings.Split(`gcc -o src/test.o -c src/test.c`, " ")

			info := NewArgParserFromOS(ctx)
			info.Parse()
			experiment.MeasureDuration("Prepend language flag", func() {
				info.ReplaceInputPath("/path/to/test.c")
			}, gmeasure.Precision(time.Microsecond))
			assert.Equal(GinkgoT(), strings.Split(`-o src/test.o -c /path/to/test.c -ffile-prefix-map=/path/to=src`, " "),
				info.Args,
			)

			os.Args = strings.Split(`gcc -o src/test.o -c src/test.cpp`, " ")

			info = NewArgParserFromOS(ctx)
			info.Parse()
			experiment.MeasureDuration("Prepend language flag", func() {
				info.ReplaceInputPath("relative/path/test.cpp")
			}, gmeasure.Precision(time.Microsecond))
			assert.Equal(GinkgoT(), strings.Split(`-o src/test.o -c relative/path/test.cpp -ffile-prefix-map=relative/path=src`, " "),
				info.Args,
			)
		}
		AddReportEntry(experiment.Name, experiment)
	})
	It("should standardize arguments where necessary", func() {
		os.Args = strings.Split(`gcc -o/path/to/test.o -c src/test.cpp`, " ")

		info := NewArgParserFromOS(ctx)
		info.Parse()
		assert.Equal(GinkgoT(), strings.Split(`-o /path/to/test.o -c src/test.cpp`, " "),
			info.Args,
		)
	})
})

//go:embed testdata
var testdata embed.FS

var _ = Describe("Test Data", func() {
	Context("Test cases should allow running remotely", func() {
		entries, err := testdata.ReadDir("testdata/should_run_remote")
		if err != nil {
			panic(err)
		}
		for i, f := range entries {
			name := filepath.Join("testdata/should_run_remote", f.Name())
			Specify(fmt.Sprintf("Test %d", i), func() {
				lines, err := fs.ReadFile(testdata, name)
				if err != nil {
					panic(err)
				}
				ap := NewArgParser(ctx, strings.Split(string(lines), "\n"))
				ap.Parse()
				Expect(ap.CanRunRemote()).To(BeTrue())
			})
		}
	})
})
