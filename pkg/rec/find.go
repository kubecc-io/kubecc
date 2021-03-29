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

package rec

import (
	"github.com/kubecc-io/kubecc/pkg/util"

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
	lg := rc.Log
	findOptions := &FindOptions{
		creator:           nil,
		recreateIfChanged: false,
	}
	findOptions.Apply(opts...)

	err := rc.Client.Get(rc.Context, name, out)
	if err != nil {
		if errors.IsNotFound(err) && findOptions.creator != nil {
			out, err = findOptions.creator(rc)
			if err != nil {
				lg.With(zap.Error(err)).Error("Error constructing object from creator")
			} else {
				if err := util.SetLastAppliedAnnotation(out); err != nil {
					lg.With(zap.Error(err)).Error("Error applying annotation")
				}
				err = ctrl.SetControllerReference(rc.RootObject, out, rc.Client.Scheme())
				if err != nil {
					lg.With(zap.Error(err)).Error("Error taking ownership of object")
				} else {
					err = rc.Client.Create(rc.Context, out)
					if err != nil {
						lg.With(zap.Error(err)).Error("Error creating object in cluster")
					}
				}
			}
			return ctrl.Result{Requeue: true}, err
		}
		lg.With(zap.Error(err)).Error("Failed to get resource")
		return ctrl.Result{}, err
	}

	if findOptions.recreateIfChanged {
		if findOptions.creator == nil {
			lg.DPanic("recreateIfChanged set but creator unset")
		}
		templateObj, err := findOptions.creator(rc)
		if err != nil {
			lg.With(zap.Error(err)).Error("Error constructing object from creator")
			return RequeueWithErr(err)
		}
		result, err := util.CalculatePatch(out, templateObj)
		if err != nil {
			lg.With(zap.Error(err)).Error("Error calculating patch, not updating object.")
			return RequeueWithErr(err)
		}
		if result.IsEmpty() {
			return DoNotRequeue()
		}
		if err := util.SetLastAppliedAnnotation(templateObj); err != nil {
			lg.With(zap.Error(err)).Error("Error applying annotation")
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
	return res.Requeue || res.RequeueAfter > 0 || err != nil
}
