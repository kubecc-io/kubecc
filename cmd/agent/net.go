package main

import (
	"fmt"
	"time"

	"github.com/cobalt77/kubecc/pkg/cluster"
	"github.com/cobalt77/kubecc/pkg/types"
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

func connectToScheduler() {
	ctx := cluster.NewAgentContext()
	go func() {
		cc, err := grpc.Dial(
			fmt.Sprintf("kubecc-scheduler.%s.svc.cluster.local:9090",
				cluster.GetNamespace()),
			grpc.WithInsecure())
		if err != nil {
			log.With(zap.Error(err)).Fatal("Error dialing scheduler")
		}
		client := types.NewSchedulerClient(cc)
		for {
			log.Info("Starting connection to the scheduler")
			stream, err := client.Connect(ctx, grpc.WaitForReady(true))
			if err != nil {
				log.With(zap.Error(err)).Error("Error connecting to scheduler. Reconnecting in 5 seconds")
				time.Sleep(5 * time.Second)
			} else {
				log.Info("Connected to the scheduler")
				<-stream.Context().Done()
				log.Info("Connection lost, reconnecting...")
			}
		}
	}()
}