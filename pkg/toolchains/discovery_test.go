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

package toolchains_test

import (
	_ "embed"
	"io/fs"
	"testing/fstest"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/google/go-cmp/cmp"
	"github.com/kubecc-io/kubecc/internal/logkc"
	"github.com/kubecc-io/kubecc/internal/testutil"
	"github.com/kubecc-io/kubecc/pkg/cc"
	"github.com/kubecc-io/kubecc/pkg/identity"
	"github.com/kubecc-io/kubecc/pkg/meta"
	"github.com/kubecc-io/kubecc/pkg/toolchains"
	"github.com/kubecc-io/kubecc/pkg/types"
	"google.golang.org/protobuf/testing/protocmp"
)

var elfHeader = []byte{'\x7F', '\x45', '\x4C', '\x46'}

func sym(to string) *fstest.MapFile {
	return &fstest.MapFile{
		Data:    []byte(to),
		Mode:    fs.ModeSymlink,
		ModTime: time.Now(),
	}
}

func elf() *fstest.MapFile {
	return &fstest.MapFile{
		Data:    elfHeader,
		Mode:    0755,
		ModTime: time.Now(),
	}
}

func dir() *fstest.MapFile {
	return &fstest.MapFile{
		Mode:    fs.ModeDir,
		ModTime: time.Now(),
	}
}

// todo: test with symlinks, commented code below didn't work

// // mapFS is a replacement for fstest.mapFS which auto-dereferences symlinks.
// type mapFS struct {
// 	fstest.MapFS
// }

// func (fsys mapFS) Open(name string) (fs.File, error) {
// 	info, err := fsys.MapFS.Stat(name)
// 	if err == nil {
// 		if info.Mode()&fs.ModeSymlink != 0 {
// 			data, err := fsys.MapFS.ReadFile(name)
// 			if err == nil {
// 				dest := filepath.Join(filepath.Dir(name), string(data))
// 				return fsys.Open(dest)
// 			}
// 		}
// 	}
// 	f, err := fsys.MapFS.Open(name)
// 	if os.IsNotExist(err) {
// 		// One of the parent directories could be a symlink
// 		for path := name; path != "" && path != "."; {
// 			path = filepath.Dir(path)
// 			p, err := fsys.MapFS.Stat(path)
// 			if err == nil && p.Mode()&fs.ModeSymlink != 0 {
// 				resolved, err := fsys.MapFS.ReadFile(path)
// 				if err == nil {
// 					return fsys.Open(filepath.Join(
// 						string(resolved), filepath.Base(name)))
// 				}
// 			}
// 		}
// 	}
// 	return f, err
// }

// type fsOnly struct{ fs.FS }

// func (fsys mapFS) ReadFile(name string) ([]byte, error) {
// 	return fs.ReadFile(fsOnly{fsys}, name)
// }

// func (fsys mapFS) Stat(name string) (fs.FileInfo, error) {
// 	return fs.Stat(fsOnly{fsys}, name)
// }

// func (fsys mapFS) ReadDir(name string) ([]fs.DirEntry, error) {
// 	return fs.ReadDir(fsOnly{fsys}, name)
// }

// func (fsys mapFS) Glob(pattern string) ([]string, error) {
// 	return fs.Glob(fsOnly{fsys}, pattern)
// }

// func TestMapFS(t *testing.T) {
// 	fs := mapFS{fstest.MapFS{
// 		"bin":                             sym("usr/bin"),
// 		"lib":                             sym("usr/lib"),
// 		"usr/bin/gcc":                     sym("gcc-10"),
// 		"usr/bin/g++":                     sym("g++-10"),
// 		"usr/bin/gcc-10":                  sym("x86_64-linux-gnu-gcc-10"),
// 		"usr/bin/gcc-9":                   sym("x86_64-linux-gnu-gcc-9"),
// 		"usr/bin/g++-10":                  sym("x86_64-linux-gnu-g++-10"),
// 		"usr/bin/g++-9":                   sym("x86_64-linux-gnu-g++-9"),
// 		"usr/bin/x86_64-linux-gnu-gcc":    sym("gcc-10"),
// 		"usr/bin/x86_64-linux-gnu-g++":    sym("g++-10"),
// 		"usr/bin/x86_64-linux-gnu-gcc-10": elf(),
// 		"usr/bin/x86_64-linux-gnu-g++-10": elf(),
// 		"usr/bin/x86_64-linux-gnu-gcc-9":  elf(),
// 		"usr/bin/x86_64-linux-gnu-g++-9":  elf(),
// 		"bin2":                            sym("bin3"),
// 		"bin3":                            sym("bin"),
// 	}}
// 	for _, test := range []string{
// 		"usr/bin/gcc",
// 		"bin/gcc",
// 		"usr/bin/g++",
// 		"bin/g++",
// 		"usr/bin/gcc-10",
// 		"usr/bin/gcc-9",
// 		"usr/bin/x86_64-linux-gnu-g++",
// 		"usr/bin/x86_64-linux-gnu-g++",
// 		"usr/bin/x86_64-linux-gnu-gcc-9",
// 		"bin2/gcc",
// 		"bin3/gcc",
// 	} {
// 		data, err := fs.ReadFile(test)
// 		if assert.NoError(t, err) {
// 			assert.Equal(t, data, elfHeader)
// 		}
// 	}
// }

var _ = Describe("Find Toolchains", func() {
	ctx := meta.NewContext(
		meta.WithProvider(identity.Component, meta.WithValue(types.TestComponent)),
		meta.WithProvider(identity.UUID),
		meta.WithProvider(logkc.Logger),
	)

	fs := fstest.MapFS{
		// "usr/bin/gcc":                     sym("gcc-10"),
		// "usr/bin/g++":                     sym("g++-10"),
		// "usr/bin/gcc-10":                  sym("x86_64-linux-gnu-gcc-10"),
		// "usr/bin/gcc-9":                   sym("x86_64-linux-gnu-gcc-9"),
		// "usr/bin/g++-10":                  sym("x86_64-linux-gnu-g++-10"),
		// "usr/bin/g++-9":                   sym("x86_64-linux-gnu-g++-9"),
		// "usr/bin/x86_64-linux-gnu-gcc":    sym("gcc-10"),
		// "usr/bin/x86_64-linux-gnu-g++":    sym("g++-10"),
		"usr/bin/x86_64-linux-gnu-gcc-10": elf(),
		"usr/bin/x86_64-linux-gnu-g++-10": elf(),
		"usr/bin/x86_64-linux-gnu-gcc-9":  elf(),
		"usr/bin/x86_64-linux-gnu-g++-9":  elf(),
		// "usr/bin/clang":                   sym("../lib/llvm-11/bin/clang"),
		// "usr/bin/clang++":                 sym("../lib/llvm-11/bin/clang++"),
		// "usr/bin/clang++-11":              sym("../lib/llvm-11/bin/clang++"),
		// "usr/bin/clang-11":                sym("../lib/llvm-11/bin/clang"),
		// "usr/lib/llvm-11/bin/clang++":     sym("clang"),
		// "usr/lib/llvm-11/bin/clang-11":    sym("clang"),
		"usr/lib/llvm-11/bin/clang": elf(),
	}
	expected := map[string]*types.Toolchain{
		"usr/bin/x86_64-linux-gnu-gcc-10": {
			Kind:       types.Gnu,
			Lang:       types.C,
			Executable: "usr/bin/x86_64-linux-gnu-gcc-10",
			TargetArch: "x86_64",
			Version:    "10",
			PicDefault: true,
		},
		"usr/bin/x86_64-linux-gnu-g++-10": {
			Kind:       types.Gnu,
			Lang:       types.CXX,
			Executable: "usr/bin/x86_64-linux-gnu-g++-10",
			TargetArch: "x86_64",
			Version:    "10",
			PicDefault: true,
		},
		"usr/bin/x86_64-linux-gnu-gcc-9": {
			Kind:       types.Gnu,
			Lang:       types.C,
			Executable: "usr/bin/x86_64-linux-gnu-gcc-9",
			TargetArch: "x86_64",
			Version:    "9",
			PicDefault: true,
		},
		"usr/bin/x86_64-linux-gnu-g++-9": {
			Kind:       types.Gnu,
			Lang:       types.CXX,
			Executable: "usr/bin/x86_64-linux-gnu-g++-9",
			TargetArch: "x86_64",
			Version:    "9",
			PicDefault: true,
		},
		"usr/lib/llvm-11/bin/clang": {
			Kind:       types.Clang,
			Lang:       types.Multi,
			Executable: "usr/lib/llvm-11/bin/clang",
			TargetArch: "x86_64",
			Version:    "11.0.0",
			PicDefault: false,
		},
	}

	When("Searching for toolchains", func() {
		It("Should find them", func() {
			store := toolchains.Aggregate(ctx,
				toolchains.FinderWithOptions{
					Finder: cc.CCFinder{},
					Opts: []toolchains.FindOption{
						toolchains.WithFS(fs),
						toolchains.WithQuerier(testutil.MockQuerier{}),
						toolchains.SearchPathEnv(false),
						toolchains.WithSearchPaths([]string{
							"usr/bin",
							"usr/lib/llvm-11/bin",
						}),
					},
				})
			tcsMap := make(map[string]*types.Toolchain)
			for tc := range store.Items() {
				tcsMap[tc.Executable] = tc
			}
			Expect(cmp.Diff(expected, tcsMap, protocmp.Transform())).To(BeEmpty())
		})
	})
})
