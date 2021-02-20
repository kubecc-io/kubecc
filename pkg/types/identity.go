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

type identityKeyType struct{}

var identityKey identityKeyType

func ContextWithIdentity(ctx context.Context, id *Identity) context.Context {
	return context.WithValue(ctx, identityKey, id)
}

func IdentityFromContext(ctx context.Context) (*Identity, bool) {
	id, ok := ctx.Value(identityKey).(*Identity)
	return id, ok
}

func OutgoingContextWithIdentity(ctx context.Context, id *Identity) context.Context {
	data, err := json.Marshal(id)
	if err != nil {
		panic(err)
	}
	return metadata.AppendToOutgoingContext(ctx, "identity", string(data))
}

func IdentityFromIncomingContext(ctx context.Context) (*Identity, error) {
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		values := md.Get("identity")
		if len(values) > 0 {
			id := &Identity{}
			err := json.Unmarshal([]byte(values[0]), id)
			return id, err
		}
	}
	return nil, errors.New("Could not find identity in context")
}
