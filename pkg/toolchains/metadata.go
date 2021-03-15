package toolchains

import (
	"context"
	"errors"

	"github.com/cobalt77/kubecc/pkg/metrics"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/proto"
)

var toolchainsKey = "kubecc-toolchains-metadata-key"

var (
	ErrNoMetadata   = errors.New("No metadata in incoming context")
	ErrNoToolchains = errors.New("No toolchains in context")
	ErrInvalidData  = errors.New("Could not unmarshal proto data")
)

func CreateMetadata(tcs *metrics.Toolchains) metadata.MD {
	data, err := proto.Marshal(tcs)
	if err != nil {
		panic("Could not marshal proto data")
	}
	return metadata.New(map[string]string{
		toolchainsKey: string(data),
	})
}

func FromIncomingContext(ctx context.Context) (*metrics.Toolchains, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil, ErrNoMetadata
	}
	data := md.Get(toolchainsKey)
	if len(data) == 0 {
		return nil, ErrNoToolchains
	}
	toolchains := &metrics.Toolchains{}
	err := proto.Unmarshal([]byte(data[1]), toolchains)
	if err != nil {
		return nil, ErrInvalidData
	}
	return toolchains, nil
}
