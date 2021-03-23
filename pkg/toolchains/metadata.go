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
	"errors"

	"github.com/cobalt77/kubecc/pkg/metrics"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/proto"
)

// -bin suffix is important here, it tells grpc to base64-encode the contents.
// See https://github.com/grpc/grpc/blob/master/doc/PROTOCOL-HTTP2.md#requests
var toolchainsKey = "kubecc-toolchains-metadata-key-bin"

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
	err := proto.Unmarshal([]byte(data[0]), toolchains)
	if err != nil {
		return nil, ErrInvalidData
	}
	return toolchains, nil
}
