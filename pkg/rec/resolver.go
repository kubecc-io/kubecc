package rec

import (
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Resolver interface {
	Resolve(current client.Object, desired client.Object) (ctrl.Result, error)
	BuildDesired() client.Object
}
