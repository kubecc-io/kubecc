package main

import (
	"errors"
	"net/http"

	"github.com/cobalt77/kubecc/internal/logkc"
	"github.com/cobalt77/kubecc/pkg/host"
	"github.com/cobalt77/kubecc/pkg/identity"
	"github.com/cobalt77/kubecc/pkg/meta"
	"github.com/cobalt77/kubecc/pkg/tracing"
	"github.com/cobalt77/kubecc/pkg/types"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

func main() {
	_ = meta.NewContext(
		meta.WithProvider(identity.Component, meta.WithValue(types.Dashboard)),
		meta.WithProvider(identity.UUID),
		meta.WithProvider(logkc.Logger),
		meta.WithProvider(tracing.Tracer),
		meta.WithProvider(host.SystemInfo),
	)

	r := gin.Default()
	r.Use(cors.New(cors.Config{
		AllowOrigins:  []string{"*"},
		AllowMethods:  []string{"GET"},
		AllowHeaders:  []string{"Origin"},
		ExposeHeaders: []string{"Content-Length"},
	}))
	r.GET("/api/status", func(c *gin.Context) {
		c.AbortWithError(http.StatusServiceUnavailable, errors.New("Unimplemented"))
	})
	r.Run(":9091")
}
