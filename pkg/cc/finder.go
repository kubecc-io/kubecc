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
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	mapset "github.com/deckarep/golang-set"
	"github.com/kubecc-io/kubecc/pkg/meta"
	"github.com/kubecc-io/kubecc/pkg/toolchains"
	"github.com/kubecc-io/kubecc/pkg/util"
	"go.uber.org/zap"
)

type CCFinder struct{}

func (f CCFinder) FindToolchains(ctx context.Context, opts ...toolchains.FindOption) *toolchains.Store {
	options := toolchains.FindOptions{
		FS:      util.OSFS,
		Querier: toolchains.ExecQuerier{},
		SearchPaths: []string{
			"/usr/bin",
			"/usr/local/bin",
			"/bin",
		},
	}
	options.Apply(opts...)

	lg := meta.Log(ctx)
	searchPaths := mapset.NewSet()
	addPath := func(set mapset.Set, path string) {
		f, err := options.FS.Stat(path)
		if os.IsNotExist(err) {
			return
		}
		if f.Mode()&fs.ModeSymlink != 0 {
			path, err = filepath.EvalSymlinks(path)
			if err != nil {
				lg.With("path", path).Debug("Symlink eval failed")
				return
			}
		}
		set.Add(path)
	}

	for _, path := range options.SearchPaths {
		addPath(searchPaths, path)
	}
	if options.Path {
		if paths, ok := os.LookupEnv("PATH"); ok {
			for _, path := range strings.Split(paths, ":") {
				addPath(searchPaths, path)
			}
		}
	}

	// Matches the following:
	// (beginning of line)                 followed by
	// (a host triple) or (empty)          followed by
	// (one of: gcc, g++, clang, clang++)  followed by
	// ('-' and a number) or (empty)       followed by
	// (end of line)
	pattern := `^(?:\w+\-\w+\-\w+\-)?(?:(?:gcc)|(?:g\+\+)|(?:clang(?:\+{2})?))(?:-[\d.]+)?$`
	re := regexp.MustCompile(pattern)

	compilers := mapset.NewSet()
	for p := range searchPaths.Iter() {
		dirname := p.(string)
		infos, err := options.FS.ReadDir(dirname)
		if err != nil {
			lg.With(zap.Error(err)).Debug("Error listing directory contents")
			continue
		}
		for _, info := range infos {
			if re.Match([]byte(filepath.Base(info.Name()))) {
				addPath(compilers, filepath.Join(dirname, info.Name()))
			}
		}
	}

	store := toolchains.NewStore()
	for c := range compilers.Iter() {
		compiler := c.(string)
		if store.Contains(compiler) {
			continue
		}
		_, err := store.Add(compiler, options.Querier)
		if err != nil {
			lg.With(
				zap.String("compiler", compiler),
				zap.Error(err),
			).Warn("Error adding toolchain")
		}
	}

	return store
}
