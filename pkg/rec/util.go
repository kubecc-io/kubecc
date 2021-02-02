package rec

import (
	"time"

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

func RequeueWith(res ctrl.Result, err error) (ctrl.Result, error) {
	if !ShouldRequeue(res, err) {
		panic("Logic error in reconciliation loop")
	}
	return res, err
}

func Requeue() (ctrl.Result, error) {
	return ctrl.Result{Requeue: true}, nil
}
