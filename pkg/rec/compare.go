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
	"reflect"

	"go.uber.org/zap"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func Equal(current interface{}, desired interface{}) bool {
	return reflect.DeepEqual(current, desired)
}

type Updater struct {
	Desired interface{}
	Actual  interface{}
	Equal   func() bool
	Apply   func()
}

func UpdateIfNeeded(
	rc ResolveContext,
	source client.Object,
	updates []Updater,
) (ctrl.Result, error) {
	for _, update := range updates {
		eq := func(u Updater) func() bool {
			return func() bool { return Equal(update.Actual, update.Desired) }
		}(update)
		if update.Equal != nil {
			eq = update.Equal
		}
		if !eq() {
			rc.Log.With(
				zap.Any("have", update.Actual),
				zap.Any("want", update.Desired),
			).Infof("Applying update")
			if update.Apply != nil {
				update.Apply()
			} else {
				update.Actual = update.Desired
			}
			err := rc.Client.Update(rc.Context, source)
			if err != nil {
				if errors.IsConflict(err) {
					rc.Log.Debug(err)
				} else {
					rc.Log.Error(err)
				}
				return RequeueWithErr(err)
			}
			return Requeue()
		}
	}
	return DoNotRequeue()
}

func CompareAffinity(a *v1.NodeAffinity, pod *v1.PodSpec) func() bool {
	return func() bool {
		if a == nil || pod.Affinity == nil {
			return a == nil && pod.Affinity == nil
		}
		if pod.Affinity.NodeAffinity == nil {
			return false
		}
		return Equal(a, pod.Affinity.NodeAffinity)
	}
}

func ApplyAffinity(a *v1.NodeAffinity, pod *v1.PodSpec) func() {
	return func() {
		pod.Affinity = &v1.Affinity{
			NodeAffinity: a,
		}
	}
}

func AffinityUpdater(a *v1.NodeAffinity, pod *v1.PodSpec) Updater {
	return Updater{
		Equal: CompareAffinity(a, pod),
		Apply: ApplyAffinity(a, pod),
	}
}

func CompareResources(a v1.ResourceRequirements, pod *v1.PodSpec, idx int) func() bool {
	return func() bool {
		if len(pod.Containers) <= idx {
			return true // Nothing to compare
		}
		return Equal(a, pod.Containers[idx].Resources)
	}
}

func ApplyResources(a v1.ResourceRequirements, pod *v1.PodSpec, idx int) func() {
	return func() {
		if len(pod.Containers) <= idx {
			return // Nothing to compare
		}
		ctr := pod.Containers[idx]
		ctr.Resources = a
		pod.Containers[idx] = ctr
	}
}

func ResourceUpdater(a v1.ResourceRequirements, pod *v1.PodSpec, idx int) Updater {
	return Updater{
		Equal: CompareResources(a, pod, idx),
		Apply: ApplyResources(a, pod, idx),
	}
}

func CompareImage(a string, container *v1.Container) func() bool {
	return func() bool {
		return a == container.Image
	}
}

func ApplyImage(a string, container *v1.Container) func() {
	return func() {
		container.Image = a
	}
}

func PodImageUpdater(a string, pod *v1.PodSpec, idx int) Updater {
	return Updater{
		Equal: CompareImage(a, &pod.Containers[idx]),
		Apply: ApplyImage(a, &pod.Containers[idx]),
	}
}

func ImageUpdater(a string, container *v1.Container) Updater {
	return Updater{
		Equal: CompareImage(a, container),
		Apply: ApplyImage(a, container),
	}
}

func ComparePullPolicy(a v1.PullPolicy, container *v1.Container) func() bool {
	return func() bool {
		return a == container.ImagePullPolicy
	}
}

func ApplyPullPolicy(a v1.PullPolicy, container *v1.Container) func() {
	return func() {
		container.ImagePullPolicy = a
	}
}

func PodPullPolicyUpdater(a v1.PullPolicy, pod *v1.PodSpec, idx int) Updater {
	return Updater{
		Equal: ComparePullPolicy(a, &pod.Containers[idx]),
		Apply: ApplyPullPolicy(a, &pod.Containers[idx]),
	}
}

func PullPolicyUpdater(a v1.PullPolicy, container *v1.Container) Updater {
	return Updater{
		Equal: ComparePullPolicy(a, container),
		Apply: ApplyPullPolicy(a, container),
	}
}

func CompareLabels(
	a map[string]string,
	pod *v1.PodTemplateSpec,
	static map[string]string,
) func() bool {
	return func() bool {
		l := make(map[string]string)
		for k, v := range a {
			l[k] = v
		}
		for k, v := range static {
			l[k] = v
		}
		return Equal(l, pod.Labels)
	}
}

func ApplyLabels(
	a map[string]string,
	pod *v1.PodTemplateSpec,
	static map[string]string,
) func() {
	return func() {
		l := make(map[string]string)
		for k, v := range a {
			l[k] = v
		}
		for k, v := range static {
			l[k] = v
		}
		pod.Labels = l
	}
}

func LabelUpdater(
	a map[string]string,
	pod *v1.PodTemplateSpec,
	static map[string]string,
) Updater {
	return Updater{
		Equal: CompareLabels(a, pod, static),
		Apply: ApplyLabels(a, pod, static),
	}
}
