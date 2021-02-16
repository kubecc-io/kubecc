package toolchains

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/cobalt77/kubecc/internal/logkc"
	mapset "github.com/deckarep/golang-set"
	"go.uber.org/zap"
)

type readDirStatFs interface {
	fs.StatFS
	fs.ReadDirFS
}

type osFS struct {
	readDirStatFs
}

func (rfs osFS) Stat(name string) (os.FileInfo, error) {
	return os.Stat(name)
}
func (rfs osFS) ReadDir(name string) ([]os.DirEntry, error) {
	return os.ReadDir(name)
}

type FindOptions struct {
	fs          readDirStatFs
	querier     Querier
	path        bool
	searchPaths []string
}
type FindOption func(*FindOptions)

func (o *FindOptions) Apply(opts ...FindOption) {
	for _, op := range opts {
		op(o)
	}
}

func WithFS(fs readDirStatFs) FindOption {
	return func(opts *FindOptions) {
		opts.fs = fs
	}
}

func WithQuerier(q Querier) FindOption {
	return func(opts *FindOptions) {
		opts.querier = q
	}
}

func WithSearchPaths(paths []string) FindOption {
	return func(opts *FindOptions) {
		opts.searchPaths = paths
	}
}

func SearchPathEnv(search bool) FindOption {
	return func(opts *FindOptions) {
		opts.path = search
	}
}

func FindToolchains(ctx context.Context, opts ...FindOption) *Store {
	options := FindOptions{
		fs:      osFS{},
		querier: ExecQuerier{},
		searchPaths: []string{
			"/usr/bin",
			"/usr/local/bin",
			"/bin",
		},
	}
	options.Apply(opts...)

	lg := logkc.LogFromContext(ctx)
	searchPaths := mapset.NewSet()
	addPath := func(set mapset.Set, path string) {
		f, err := options.fs.Stat(path)
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

	for _, path := range options.searchPaths {
		addPath(searchPaths, path)
	}
	if options.path {
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
		infos, err := options.fs.ReadDir(dirname)
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

	store := NewStore()
	for c := range compilers.Iter() {
		compiler := c.(string)
		if store.Contains(compiler) {
			continue
		}
		_, err := store.Add(compiler, options.querier)
		if err != nil {
			lg.With(
				zap.String("compiler", compiler),
				zap.Error(err),
			).Warn("Error adding toolchain")
		}
	}

	return store
}
