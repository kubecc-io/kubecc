package main

import (
	"fmt"
	"os"

	"github.com/cobalt77/kubecc/pkg/cluster"
	"github.com/cobalt77/kubecc/pkg/types"
	"github.com/spf13/viper"
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

func runFollower() {
	// Connect to the leader
	c, err := grpc.Dial(
		fmt.Sprintf("127.0.0.1:%d", viper.GetInt("port")),
		grpc.WithInsecure())
	if err != nil {
		log.With(zap.Error(err)).Fatal("Error connecting to leader")
	}
	dispatchAndWait(c)
}

func dispatchAndWait(cc *grpc.ClientConn) {
	log.Info("Dispatching to leader")
	leaderClient := types.NewConsumerClient(cc)
	wd, err := os.Getwd()
	if err != nil {
		log.Fatal(err.Error())
	}
	ctx, _ := cluster.NewAgentContext()
	_, err = leaderClient.Run(ctx, &types.DispatchRequest{
		WorkDir: wd,
		Command: os.Args[0],
		Args:    os.Args[1:],
		Env:     os.Environ(),
	}, grpc.WaitForReady(true))
	if err != nil {
		log.Error("Dispatch error")
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	log.Debug("Dispatch Success")
	os.Exit(0)
}
