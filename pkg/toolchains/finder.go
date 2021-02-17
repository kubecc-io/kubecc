package toolchains

import (
	"context"

	"github.com/cobalt77/kubecc/pkg/tools"
)

type Finder interface {
	FindToolchains(ctx context.Context, opts ...FindOption) *Store
}

type FindOptions struct {
	fs          tools.ReadDirStatFS
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

func WithFS(fs tools.ReadDirStatFS) FindOption {
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
