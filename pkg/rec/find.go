package rec

import (
	"github.com/cobalt77/kubecc/internal/lll"
	"github.com/cobalt77/kubecc/pkg/tools"
	"go.uber.org/zap"
	ctrl "sigs.k8s.io/controller-runtime"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type ObjectCreator func(ResolveContext) (client.Object, error)

type FindOptions struct {
	creator           ObjectCreator
	recreateIfChanged bool
}
type findOption func(*FindOptions)

func (o *FindOptions) Apply(opts ...findOption) {
	for _, opt := range opts {
		opt(o)
	}
}

func WithCreator(cr ObjectCreator) findOption {
	return func(opts *FindOptions) {
		opts.creator = cr
	}
}

func RecreateIfChanged() findOption {
	return func(opts *FindOptions) {
		opts.recreateIfChanged = true
	}
}

func Find(
	rc ResolveContext,
	name types.NamespacedName,
	out client.Object,
	opts ...findOption,
) (ctrl.Result, error) {
	findOptions := &FindOptions{
		creator:           nil,
		recreateIfChanged: false,
	}
	findOptions.Apply(opts...)

	err := rc.Client.Get(rc.Context, name, out)
	if err != nil {
		if errors.IsNotFound(err) && findOptions.creator != nil {
			out, err = findOptions.creator(rc)
			if err := tools.SetLastAppliedAnnotation(out); err != nil {
				lll.With(zap.Error(err)).Error("Error applying annotation")
			}
			if err != nil {
				lll.With(zap.Error(err)).Error("Error constructing object from creator")
			} else {
				err = ctrl.SetControllerReference(rc.RootObject, out, rc.Client.Scheme())
				if err != nil {
					lll.With(zap.Error(err)).Error("Error taking ownership of object")
				} else {
					err = rc.Client.Create(rc.Context, out)
					if err != nil {
						lll.With(zap.Error(err)).Error("Error creating object in cluster")
					}
				}
			}
			return ctrl.Result{Requeue: true}, err
		}
		lll.With(zap.Error(err)).Error("Failed to get resource")
		return ctrl.Result{}, err
	}

	if findOptions.recreateIfChanged {
		if findOptions.creator == nil {
			lll.DPanic("recreateIfChanged set but creator unset")
		}
		templateObj, err := findOptions.creator(rc)
		if err != nil {
			lll.With(zap.Error(err)).Error("Error constructing object from creator")
			return RequeueWithErr(err)
		}
		result, err := tools.CalculatePatch(out, templateObj)
		if err != nil {
			lll.With(zap.Error(err)).Error("Error calculating patch, not updating object.")
			return RequeueWithErr(err)
		}
		if result.IsEmpty() {
			return DoNotRequeue()
		}
		if err := tools.SetLastAppliedAnnotation(templateObj); err != nil {
			lll.With(zap.Error(err)).Error("Error applying annotation")
		} else {
			err := rc.Client.Update(rc.Context, templateObj)
			if err != nil {
				return RequeueWithErr(err)
			}
		}
	}

	return DoNotRequeue()
}

func ShouldRequeue(res ctrl.Result, err error) bool {
	return res.Requeue == true || res.RequeueAfter > 0 || err != nil
}
