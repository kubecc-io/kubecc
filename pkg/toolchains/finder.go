package toolchains

import (
	"context"

	"github.com/cobalt77/kubecc/pkg/util"
)

type Finder interface {
	FindToolchains(ctx context.Context, opts ...FindOption) *Store
}

type FindOptions struct {
	FS          util.ReadDirStatFS
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

func WithFS(fs util.ReadDirStatFS) FindOption {
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
