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
