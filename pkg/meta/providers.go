package meta

import (
	"context"

	"github.com/cobalt77/kubecc/pkg/meta/mdkeys"
	"github.com/cobalt77/kubecc/pkg/types"
	"google.golang.org/grpc/metadata"
)

type metadataProvider struct{}

func (mp *metadataProvider) Key() MetadataKey {
	return mdkeys.ComponentKey
}

func (mp *metadataProvider) ExportToOutgoing(ctx context.Context) {
	ctx = metadata.AppendToOutgoingContext(ctx,
		mp.Key().String(),
		ctx.Value(mp.Key()).(types.Component).String(),
	)
}

func (mp *metadataProvider) ImportFromIncoming(ctx context.Context, md metadata.MD) {
	values := md.Get(mp.Key().String())
	if len(values) == 0 {
		return
	}
	ctx = context.WithValue(ctx, mp.Key().String(), values[0])
}
