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

// Package rec contains utilities to help with reconciling desired state.
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
