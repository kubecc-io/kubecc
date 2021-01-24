package rec

import (
	"time"

	"github.com/cobalt77/kubecc/internal/lll"
	ctrl "sigs.k8s.io/controller-runtime"
)

func DoNotRequeue() (ctrl.Result, error) {
	return ctrl.Result{}, nil
}

func RequeueAfter(d time.Duration) (ctrl.Result, error) {
	return ctrl.Result{RequeueAfter: d}, nil
}

func RequeueWithErr(err error) (ctrl.Result, error) {
	return ctrl.Result{}, err
}

func Requeue(res ctrl.Result, err error) (ctrl.Result, error) {
	if !ShouldRequeue(res, err) {
		lll.DPanic("Logic error in reconciliation loop")
	}
	return res, err
}
