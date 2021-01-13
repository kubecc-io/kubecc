package main

// func connectToScheduler() {
// 	ctx := cluster.NewAgentContext()
// 	go func() {
// 		cc, err := grpc.Dial(
// 			fmt.Sprintf("kubecc-scheduler.%s.svc.cluster.local:9090",
// 				cluster.GetNamespace()),
// 			grpc.WithInsecure())
// 		if err != nil {
// 			lll.With(zap.Error(err)).Fatal("Error dialing scheduler")
// 		}
// 		client := types.NewSchedulerClient(cc)
// 		for {
// 			lll.Info("Starting connection to the scheduler")
// 			stream, err := client.Connect(ctx, grpc.WaitForReady(true))
// 			if err != nil {
// 				lll.With(zap.Error(err)).Error("Error connecting to scheduler. Reconnecting in 5 seconds")
// 				time.Sleep(5 * time.Second)
// 			}
// 			lll.Info("Connected to the scheduler")
// 			for {
// 				_, err := stream.Recv()
// 				if err != nil {
// 					lll.With(zap.Error(err)).Error("Connection lost, reconnecting...")
// 				}
// 				break
// 			}
// 		}
// 	}()
// }
