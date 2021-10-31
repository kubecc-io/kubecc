package buildcluster

import (
	"context"
	"fmt"
	"os"

	"github.com/banzaicloud/operator-tools/pkg/reconciler"
	"github.com/kubecc-io/kubecc/api/v1alpha1"
	"github.com/kubecc-io/kubecc/pkg/meta"
	"github.com/kubecc-io/kubecc/pkg/resources"
	"github.com/kubecc-io/kubecc/pkg/util"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type Reconciler struct {
	reconciler.ResourceReconciler
	ctx          context.Context
	client       client.Client
	buildCluster *v1alpha1.BuildCluster
}

func NewReconciler(
	ctx context.Context,
	client client.Client,
	buildCluster *v1alpha1.BuildCluster,
	opts ...func(*reconciler.ReconcilerOpts),
) *Reconciler {
	lg := meta.Log(ctx)
	return &Reconciler{
		ResourceReconciler: reconciler.NewReconcilerWith(client,
			append(opts, reconciler.WithLog(&util.ZapfLogShim{
				ZapLogger: lg,
			}))...),
		ctx:          ctx,
		client:       client,
		buildCluster: buildCluster,
	}
}

func (r *Reconciler) Reconcile() (*reconcile.Result, error) {
	lg := meta.Log(r.ctx)
	items := []resources.Resource{}

	if modified, err := r.configureDefaultImage(); err != nil {
		lg.Error(err)
		return nil, err
	} else if modified {
		return &reconcile.Result{Requeue: true}, nil
	}

	cm, err := r.configMap()
	if err != nil {
		lg.Error(err)
		return nil, err
	}
	items = append(items, cm...)

	monitor, err := r.monitor()
	if err != nil {
		lg.Error(err)
		return nil, err
	}
	items = append(items, monitor...)

	scheduler, err := r.scheduler()
	if err != nil {
		lg.Error(err)
		return nil, err
	}
	items = append(items, scheduler...)

	cacheSrv, err := r.cacheSrv()
	if err != nil {
		lg.Error(err)
		return nil, err
	}
	items = append(items, cacheSrv...)

	toolbox, err := r.toolbox()
	if err != nil {
		lg.Error(err)
		return nil, err
	}
	items = append(items, toolbox...)

	for _, tcSpec := range r.buildCluster.Spec.Toolchains {
		tc, err := r.toolchain(tcSpec)
		if err != nil {
			lg.Error(err)
			return nil, err
		}
		items = append(items, tc...)
	}

	for _, resourceBuilder := range items {
		o, state, err := resourceBuilder()
		if err != nil {
			lg.Error(err)
			return nil, err
		}
		if o == nil {
			panic(fmt.Sprintf("resource builder %#v created a nil object", resourceBuilder))
		}
		result, err := r.ReconcileResource(o, state)
		if err != nil {
			lg.Error(err)
			return result, err
		}
		if result != nil && (result.Requeue || result.RequeueAfter > 0) {
			return result, err
		}
	}

	return &reconcile.Result{}, nil
}

func (r *Reconciler) configureDefaultImage() (bool, error) {
	lg := meta.Log(r.ctx)
	// Look up our own deployment to get the image name. This will be the default
	// image to copy to all our deployments
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "kubecc-operator",
			Namespace: os.Getenv("KUBECC_NAMESPACE"),
		},
	}
	if err := r.client.Get(r.ctx, client.ObjectKeyFromObject(deployment), deployment); err != nil {
		return false, err
	}
	var imageName string
	for _, container := range deployment.Spec.Template.Spec.Containers {
		if container.Name == "manager" {
			imageName = container.Image
		}
	}
	if imageName == "" {
		panic("could not find image name in kubecc-operator deployment")
	}

	if r.buildCluster.Spec.Image != "" {
		imageName = r.buildCluster.Spec.Image
	}

	if r.buildCluster.Status.DefaultImageName != imageName {
		lg.Info("Found image name: " + imageName)
		lg.Info("Updating buildcluster status")
		err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
			err := r.client.Get(r.ctx, client.ObjectKeyFromObject(r.buildCluster), r.buildCluster)
			if err != nil {
				lg.Error(err)
				return err
			}
			r.buildCluster.Status.DefaultImageName = imageName
			return r.client.Status().Update(r.ctx, r.buildCluster)
		})
		if err != nil {
			lg.Error(err)
			return false, err
		}
		return true, nil
	}
	return false, nil
}
