//go:build !context_tracking
// +build !context_tracking

package ctxutil

import (
	"context"
)

func WithCancel(parent context.Context) (ctx context.Context, cancel context.CancelFunc) {
	ctx, ca := context.WithCancel(parent)
	return ctx, func() {
		ca()
	}
}
