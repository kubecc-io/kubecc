package toolchains

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/cobalt77/kubecc/internal/logkc"
	"github.com/cobalt77/kubecc/pkg/types"
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
	fs      readDirStatFs
	querier Querier
}
type findOption func(*FindOptions)

func (o *FindOptions) Apply(opts ...findOption) {
	for _, op := range opts {
		op(o)
	}
}

func WithFS(fs readDirStatFs) findOption {
	return func(opts *FindOptions) {
		opts.fs = fs
	}
}

func WithQuerier(q Querier) findOption {
	return func(opts *FindOptions) {
		opts.querier = q
	}
}

func FindToolchains(ctx context.Context, opts ...findOption) (tcs []*types.Toolchain) {
	options := FindOptions{
		fs:      osFS{},
		querier: ExecQuerier{},
	}
	options.Apply(opts...)

	lg := logkc.LogFromContext(ctx)
	tcs = []*types.Toolchain{}
	searchPaths := mapset.NewSet()
	addPath := func(set mapset.Set, path string) {
		if _, err := options.fs.Stat(path); os.IsNotExist(err) {
			return
		}
		realPath, err := filepath.EvalSymlinks(path)
		if err != nil {
			lg.With("path", path).Debug("Symlink eval failed")
			return
		}
		set.Add(realPath)
	}
	addPath(searchPaths, "/usr/bin")
	addPath(searchPaths, "/usr/local/bin")
	addPath(searchPaths, "/bin")

	if paths, ok := os.LookupEnv("PATH"); ok {
		for _, path := range strings.Split(paths, ":") {
			addPath(searchPaths, path)
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

	for c := range compilers.Iter() {
		compiler := c.(string)
		arch, err := options.querier.TargetArch(compiler)
		if err != nil {
			lg.With(
				"compiler", compiler,
				zap.Error(err),
			).Warn("Could not determine target arch")
			continue
		}
		version, err := options.querier.Version(compiler)
		if err != nil {
			lg.With(
				"compiler", compiler,
				zap.Error(err),
			).Warn("Could not determine target version")
			continue
		}
		pic, err := options.querier.IsPicDefault(compiler)
		if err != nil {
			lg.With(
				"compiler", compiler,
				zap.Error(err),
			).Warn("Could not determine compiler PIC defaults")
			continue
		}
		kind, err := options.querier.Kind(compiler)
		if err != nil {
			lg.With(
				"compiler", compiler,
				zap.Error(err),
			).Warn("Could not determine compiler kind (gcc/clang)")
			continue
		}
		lang, err := options.querier.Lang(compiler)
		if err != nil {
			lg.With(
				"compiler", compiler,
				zap.Error(err),
			).Warn("Could not determine compiler language (c/cxx/multi)")
			continue
		}
		tcs = append(tcs, &types.Toolchain{
			Kind:       kind,
			Lang:       lang,
			Executable: compiler,
			TargetArch: arch,
			PicDefault: pic,
			Version:    version,
		})
	}

	return
}
