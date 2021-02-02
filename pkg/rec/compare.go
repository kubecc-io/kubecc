package rec

import (
	"reflect"

	v1 "k8s.io/api/core/v1"
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
		eq := func() bool {
			return Equal(update.Actual, update.Desired)
		}
		if update.Equal != nil {
			eq = update.Equal
		}
		if !eq() {
			if update.Apply != nil {
				update.Apply()
			} else {
				update.Actual = update.Desired
			}
			err := rc.Client.Update(rc.Context, source)
			if err != nil {
				rc.Log.Error(err)
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

func CompareImage(a string, pod *v1.PodSpec, idx int) func() bool {
	return func() bool {
		if len(pod.Containers) <= idx {
			return true // Nothing to compare
		}
		return a == pod.Containers[idx].Image
	}
}

func ApplyImage(a string, pod *v1.PodSpec, idx int) func() {
	return func() {
		if len(pod.Containers) <= idx {
			return // Nothing to compare
		}
		ctr := pod.Containers[idx]
		ctr.Image = a
		pod.Containers[idx] = ctr
	}
}

func ImageUpdater(a string, pod *v1.PodSpec, idx int) Updater {
	return Updater{
		Equal: CompareImage(a, pod, idx),
		Apply: ApplyImage(a, pod, idx),
	}
}

func ComparePullPolicy(a v1.PullPolicy, pod *v1.PodSpec, idx int) func() bool {
	return func() bool {
		if len(pod.Containers) <= idx {
			return true // Nothing to compare
		}
		return a == pod.Containers[idx].ImagePullPolicy
	}
}

func ApplyPullPolicy(a v1.PullPolicy, pod *v1.PodSpec, idx int) func() {
	return func() {
		if len(pod.Containers) <= idx {
			return // Nothing to compare
		}
		ctr := pod.Containers[idx]
		ctr.ImagePullPolicy = a
		pod.Containers[idx] = ctr
	}
}

func PullPolicyUpdater(a v1.PullPolicy, pod *v1.PodSpec, idx int) Updater {
	return Updater{
		Equal: ComparePullPolicy(a, pod, idx),
		Apply: ApplyPullPolicy(a, pod, idx),
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
