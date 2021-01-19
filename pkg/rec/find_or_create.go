package rec

import (
	"context"

	"github.com/cobalt77/kubecc/internal/lll"
	ctrl "sigs.k8s.io/controller-runtime"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type ObjectCreator interface {
	Create(rootCrd client.Object, name types.NamespacedName) error
}

func FindOrCreate(
	cli client.Client,
	ctx context.Context,
	rootCrd client.Object,
	name types.NamespacedName,
	obj client.Object,
	creator ObjectCreator,
) (ctrl.Result, error) {
	err := cli.Get(ctx, name, obj)
	if err != nil {
		if errors.IsNotFound(err) {
			err := creator.Create(rootCrd, name)
			if err != nil {
				lll.With(err).Error("Error creating object")
			}
			return ctrl.Result{Requeue: true}, err
		}
		lll.Error(err, "Failed to get resource")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func Done(res ctrl.Result, err error) bool {
	return res.Requeue == false && res.RequeueAfter == 0 && err == nil
}
