package rec

import (
	"context"

	"github.com/cobalt77/kubecc/pkg/meta"
	"go.uber.org/zap"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Resolver interface {
	Resolve(rc ResolveContext) (ctrl.Result, error)
	Find(root client.Object) interface{}
}

type ResolveContext struct {
	Context    context.Context
	Log        *zap.SugaredLogger
	Client     client.Client
	RootObject client.Object
	Object     interface{}
}

type ResolverTree struct {
	ctrlCtx  context.Context
	client   client.Client
	Resolver Resolver
	Find     func() interface{}
	Nodes    []*ResolverTree
}

func BuildRootResolver(ctrlCtx context.Context, client client.Client, tree *ResolverTree) *ResolverTree {
	tree.injectClient(ctrlCtx, client)
	return tree
}

func (t *ResolverTree) injectClient(ctrlCtx context.Context, client client.Client) {
	t.client = client
	t.ctrlCtx = ctrlCtx
	for _, node := range t.Nodes {
		node.injectClient(ctrlCtx, client)
	}
}

func (t *ResolverTree) Walk(ctx context.Context, root client.Object) (ctrl.Result, error) {
	for _, node := range t.Nodes {
		if res, err := node.Resolver.Resolve(ResolveContext{
			Log:        meta.Log(t.ctrlCtx),
			Context:    ctx,
			Client:     t.client,
			RootObject: root,
			Object:     node.Resolver.Find(root),
		}); ShouldRequeue(res, err) {
			return RequeueWith(res, err)
		}
		if res, err := node.Walk(ctx, root); ShouldRequeue(res, err) {
			return RequeueWith(res, err)
		}
	}
	return DoNotRequeue()
}
