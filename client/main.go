package main

import (
	"context"
	"crypto/tls"

	log "github.com/sirupsen/logrus"

	"github.com/cobalt77/kube-distcc/mgr/api"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

func main() {
	var conn *grpc.ClientConn
	conn, err := grpc.Dial("---:443",
		grpc.WithTransportCredentials(credentials.NewTLS(
			&tls.Config{
				InsecureSkipVerify: false,
			},
		)))
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()
	client := api.NewApiClient(conn)
	status, err := client.Status(context.Background(), &api.StatusRequest{})

	if err != nil {
		log.Fatal(err)
	}

	log.Info(status.Agents)
}
