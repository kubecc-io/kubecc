package types

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/google/uuid"
	"google.golang.org/grpc/metadata"
)

func NewIdentity(component Component) *Identity {
	return &Identity{
		Component: component,
		UUID:      uuid.NewString(),
	}
}

func (id *Identity) Equal(other *Identity) bool {
	return id.UUID == other.UUID
}

func ContextWithIdentity(ctx context.Context, id *Identity) context.Context {
	data, err := json.Marshal(id)
	if err != nil {
		panic(err)
	}
	return metadata.AppendToOutgoingContext(ctx, "identity", string(data))
}

func IdentityFromContext(ctx context.Context) (*Identity, error) {
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		values := md.Get("identity")
		if len(values) == 1 {
			id := &Identity{}
			err := json.Unmarshal([]byte(values[0]), id)
			return id, err
		}
	}
	return nil, errors.New("Could not find identity in context")
}
