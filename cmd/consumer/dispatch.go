package main

import (
	"context"
	"fmt"
	"os"

	"github.com/cobalt77/kubecc/pkg/types"
	"google.golang.org/grpc"
)

func dispatchAndWait(cc *grpc.ClientConn) {
	log.Info("Dispatching to leader")
	leaderClient := types.NewConsumerdClient(cc)
	wd, err := os.Getwd()
	if err != nil {
		log.Fatal(err.Error())
	}
	_, err = leaderClient.Run(context.Background(), &types.RunRequest{
		WorkDir: wd,
		Args:    os.Args,
		Env:     os.Environ(),
		UID:     uint32(os.Getuid()),
		GID:     uint32(os.Getgid()),
	}, grpc.WaitForReady(true))
	if err != nil {
		log.Error("Dispatch error")
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	log.Debug("Dispatch Success")
	os.Exit(0)
}
