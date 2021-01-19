package rec

import (
	"errors"

	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type neverCreate struct {
	ObjectCreator
}

func (nc neverCreate) Create(client.Object, types.NamespacedName) error {
	return errors.New("Object could not be created")
}

func NeverCreate() neverCreate {
	return neverCreate{}
}
