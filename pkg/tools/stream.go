package tools

import (
	"github.com/cobalt77/kubecc/pkg/types"
	"google.golang.org/grpc"
)

type EmptyServerStream interface {
	grpc.ClientStream
	Recv() (*types.Empty, error)
}

func StreamClosed(stream EmptyServerStream) <-chan error {
	ch := make(chan error)
	go func() {
		_, err := stream.Recv()
		ch <- err
		close(ch)
	}()
	return ch
}
