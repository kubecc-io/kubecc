package types

import (
	"github.com/google/uuid"
)

func NewIdentity(component Component) *Identity {
	return &Identity{
		Component: component,
		UUID:      uuid.NewString(),
	}
}

func (id Identity) Equal(other *Identity) bool {
	return id.UUID == other.UUID
}
