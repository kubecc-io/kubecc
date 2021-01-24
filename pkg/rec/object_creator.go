package rec

import (
	"github.com/cobalt77/kubecc/internal/lll"
	ctrl "sigs.k8s.io/controller-runtime"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type ObjectCreator func(ResolveContext) (client.Object, error)

func FindOrCreate(
	rc ResolveContext,
	name types.NamespacedName,
	out client.Object,
	creator ObjectCreator,
) (ctrl.Result, error) {
	err := rc.Client.Get(rc.Context, name, out)
	if err != nil {
		if errors.IsNotFound(err) {
			out, err = creator(rc)
			if err != nil {
				lll.With(err).Error("Error creating object")
			} else {
				err = ctrl.SetControllerReference(rc.RootObject, out, rc.Client.Scheme())
				if err != nil {
					lll.With(err).Error("Error taking ownership of object")
				}
			}
			return ctrl.Result{Requeue: true}, err
		}
		lll.Error(err, "Failed to get resource")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func ShouldRequeue(res ctrl.Result, err error) bool {
	return res.Requeue == false && res.RequeueAfter == 0 && err == nil
}
