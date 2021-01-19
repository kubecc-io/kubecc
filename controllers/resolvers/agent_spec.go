package resolvers

import (
	"github.com/cobalt77/kubecc/api/v1alpha1"
	"github.com/cobalt77/kubecc/pkg/rec"
	ctrl "sigs.k8s.io/controller-runtime"
)

type agentResolver struct {
	rec.Resolver
}

func Resolve(current interface{}, desired interface{}) (ctrl.Result, error) {
	cur := current.(v1alpha1.AgentSpec)
	des := desired.(v1alpha1.AgentSpec)

	if rec.Equal(cur, des) {
		return
	}
}

func BuildDesired() interface{} {

}
