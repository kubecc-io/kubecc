//+build context_tracking

// Package ctxutil provides methods that add instrumentation and debugging
// utilities to context objects.
package ctxutil

import (
	"context"
	"fmt"
	"runtime/debug"
)

func WithCancel(parent context.Context) (ctx context.Context, cancel context.CancelFunc) {
	fmt.Printf("\n====== NEW TRACKED CONTEXT ======\n%s=================================\n\n",
		debug.Stack())
	c, ca := context.WithCancel(parent)
	return c, func() {
		fmt.Printf("\n====== TRACKED CONTEXT CANCELED ======\n%s======================================\n\n",
			debug.Stack())
		ca()
	}
}
