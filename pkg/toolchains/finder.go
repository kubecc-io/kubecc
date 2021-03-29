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

package toolchains

import (
	"context"

	"github.com/kubecc-io/kubecc/pkg/util"
)

// A Finder can locate toolchains in a filesystem.
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
