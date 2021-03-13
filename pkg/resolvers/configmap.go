package resolvers

import (
	"github.com/cobalt77/kubecc/api/v1alpha1"
	"github.com/cobalt77/kubecc/pkg/rec"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type ConfigMapResolver struct{}

const (
	configMapName = "kubecc"
)

func (r *ConfigMapResolver) Resolve(
	rc rec.ResolveContext,
) (ctrl.Result, error) {
	configMap := &v1.ConfigMap{}
	res, err := rec.Find(rc, types.NamespacedName{
		Namespace: rc.RootObject.GetNamespace(),
		Name:      configMapName,
	}, configMap,
		rec.WithCreator(rec.FromTemplate("objects/kubecc_configmap.yaml")),
	)
	if rec.ShouldRequeue(res, err) {
		return rec.RequeueWith(res, err)
	}

	return rec.DoNotRequeue()
}

func (r *ConfigMapResolver) Find(root client.Object) interface{} {
	return root.(*v1alpha1.BuildCluster).Spec.Components
}
