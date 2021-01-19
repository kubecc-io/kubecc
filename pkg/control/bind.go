package control

import (
	"context"
	"reflect"

	"github.com/cobalt77/pkg/rec"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type ResolveFunc func(context.Context, client.Client, client.Object) (ctrl.Result, error)
type Comparator func(client.Object, client.Object) (ctrl.Result, error)

type Binder struct {
	nodes    []*Binder
	name     types.NamespacedName
	objType  reflect.Type
	resolver ResolveFunc
}

func (b *Binder) Resolve(ctx context.Context, r client.Client, crd client.Object) (ctrl.Result, error) {
	result, err := b.resolveSelf(ctx, r)
	if rec.Done(result, err) {
		return result, err
	}
	for _, binder := range b.nodes {
		r, err := binder.Resolve(ctx, r, crd)
		if rec.Done(result, err) {
			return r, err
		}
	}
	return ctrl.Result{}, nil
}

func (b *Binder) Bind(fieldJson string, creator CreatorFunc) *Binder {
	binder := &Binder{
		nodes: []*Binder{},
	}
}

func NewRootBinder(name types.NamespacedName, obj interface{}) *Binder {
	return &Binder{
		nodes:   []*Binder{},
		name:    name,
		objType: reflect.TypeOf(obj),
		creator: func() (client.Object, error) {
			return nil, errors.NewBadRequest("Resource not found (may already be deleted)")
		},
	}
}

func (b *Binder) resolveSelf(ctx context.Context, r client.Client) (ctrl.Result, error) {
	i := reflect.New(b.objType).Interface().(client.Object)
	err := r.Get(ctx, b.name, i)
	if err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}
