package tools

import (
	"context"
	"sync"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type ReconcileFn = func(context.Context, client.Object) (ctrl.Result, error)

func ReconcileAndAggregate(
	ctx context.Context,
	obj client.Object,
	fns ...ReconcileFn,
) (ctrl.Result, error) {
	wg := &sync.WaitGroup{}
	wg.Add(len(fns))
	resultList := make([]ctrl.Result, len(fns))
	errList := make([]error, len(fns))
	for i, fn := range fns {
		go func(i int, fn ReconcileFn) {
			defer wg.Done()
			resultList[i], errList[i] = fn(ctx, obj)
		}(i, fn)
	}
	wg.Wait()
	return ctrl.Result{
			Requeue: func() bool {
				for _, result := range resultList {
					if result.Requeue {
						return true
					}
				}
				return false
			}(),
		}, func() error {
			for _, err := range errList {
				if err != nil {
					return err
				}
			}
			return nil
		}()
}