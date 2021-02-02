package scheduler

import "google.golang.org/grpc"

type AgentResolver interface {
	Dial() (*grpc.ClientConn, error)
}
