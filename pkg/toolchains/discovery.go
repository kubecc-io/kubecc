package toolchains

import "github.com/cobalt77/kubecc/pkg/meta"

type FinderWithOptions struct {
	Finder
	Opts []FindOption
}

func Aggregate(
	ctx meta.Context,
	finders ...FinderWithOptions,
) *Store {
	store := NewStore()
	for _, f := range finders {
		store.Merge(f.FindToolchains(ctx, f.Opts...))
	}
	return store
}
