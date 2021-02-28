package toolchains

import (
	"github.com/cobalt77/kubecc/pkg/meta"
	"github.com/cobalt77/kubecc/pkg/tools"
)

type Finder interface {
	FindToolchains(ctx meta.Context, opts ...FindOption) *Store
}

type FindOptions struct {
	FS          tools.ReadDirStatFS
	Querier     Querier
	Path        bool
	SearchPaths []string
}
type FindOption func(*FindOptions)

func (o *FindOptions) Apply(opts ...FindOption) {
	for _, op := range opts {
		op(o)
	}
}

func WithFS(fs tools.ReadDirStatFS) FindOption {
	return func(opts *FindOptions) {
		opts.FS = fs
	}
}

func WithQuerier(q Querier) FindOption {
	return func(opts *FindOptions) {
		opts.Querier = q
	}
}

func WithSearchPaths(paths []string) FindOption {
	return func(opts *FindOptions) {
		opts.SearchPaths = paths
	}
}

func SearchPathEnv(search bool) FindOption {
	return func(opts *FindOptions) {
		opts.Path = search
	}
}
