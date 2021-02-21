package main

import (
	"context"
	"net/http"

	"github.com/cobalt77/kubecc/internal/logkc"
	"github.com/cobalt77/kubecc/pkg/types"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

func main() {
	ctx := logkc.NewWithContext(context.Background(), types.Dashboard)
	lg := logkc.LogFromContext(ctx)
	cc, err := grpc.Dial("localhost:9090", grpc.WithInsecure())
	if err != nil {
		lg.With(zap.Error(err)).Fatal("Error dialing scheduler")
	}
	client := types.NewSchedulerClient(cc)

	r := gin.Default()
	r.Use(cors.New(cors.Config{
		AllowOrigins:  []string{"*"},
		AllowMethods:  []string{"GET"},
		AllowHeaders:  []string{"Origin"},
		ExposeHeaders: []string{"Content-Length"},
	}))
	r.GET("/api/status", func(c *gin.Context) {
		response, err := client.SystemStatus(ctx, &types.Empty{})
		if err != nil {
			c.AbortWithError(http.StatusServiceUnavailable, err)
		} else {
			c.JSON(http.StatusOK, response)
		}
	})
	r.Run(":9091")
}
