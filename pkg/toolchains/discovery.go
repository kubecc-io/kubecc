package toolchains

import "context"

type FinderWithOptions struct {
	Finder
	Opts []FindOption
}

func Aggregate(
	ctx context.Context,
	finders ...FinderWithOptions,
) *Store {
	store := NewStore()
	for _, f := range finders {
		store.Merge(f.FindToolchains(ctx, f.Opts...))
	}
	return store
}
