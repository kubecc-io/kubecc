package main

import (
	"context"
	"fmt"
	"time"

	"github.com/cobalt77/kubecc/pkg/cluster"
	"github.com/cobalt77/kubecc/pkg/types"
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

func connectToScheduler(ctx context.Context) {
	go func() {
		cc, err := grpc.Dial(
			fmt.Sprintf("kubecc-scheduler.%s.svc.cluster.local:9090",
				cluster.GetNamespace()),
			grpc.WithInsecure())
		if err != nil {
			lg.With(zap.Error(err)).Fatal("Error dialing scheduler")
		}
		client := types.NewSchedulerClient(cc)
		for {
			lg.Info("Starting connection to the scheduler")
			stream, err := client.Connect(ctx, grpc.WaitForReady(true))
			if err != nil {
				lg.With(zap.Error(err)).Error("Error connecting to scheduler. Reconnecting in 5 seconds")
				time.Sleep(5 * time.Second)
			}
			lg.Info("Connected to the scheduler")
			for {
				_, err := stream.Recv()
				if err != nil {
					lg.With(zap.Error(err)).Error("Connection lost, reconnecting...")
				}
				break
			}
		}
	}()
}
