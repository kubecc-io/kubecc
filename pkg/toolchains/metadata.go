package toolchains

import (
	"context"
	"encoding/base64"
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
	// Important: can't add raw proto wire data to grpc metadata.
	// It will result in a protocol error from grpc.
	return metadata.New(map[string]string{
		toolchainsKey: base64.StdEncoding.EncodeToString(data),
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
	wire, err := base64.StdEncoding.DecodeString(data[0])
	if err != nil {
		return nil, ErrInvalidData
	}
	toolchains := &metrics.Toolchains{}
	err = proto.Unmarshal(wire, toolchains)
	if err != nil {
		return nil, ErrInvalidData
	}
	return toolchains, nil
}
