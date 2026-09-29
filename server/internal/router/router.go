// Package router 集中注册 HTTP 路由。
// 只绑定 URL、方法、中间件与 Handler，不写业务状态规则。
package router

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"citywall/server/internal/api"
	"citywall/server/internal/middleware"
)

// Handlers 聚合各模块 Handler，供 Router 注册。
type Handlers struct {
	Task        *api.TaskHandler
	Observation *api.ObservationHandler
}

// Router 构建 gin 引擎：全局中间件 + 健康检查 + /api/v1 分组。
func Router(h Handlers, reqIDHeader string) *gin.Engine {
	engine := gin.New()
	engine.Use(middleware.Recovery(), middleware.RequestID(reqIDHeader))

	engine.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	// /readyz 由 bootstrap 注入数据库就绪状态。

	v1 := engine.Group("/api/v1")
	RegisterTaskRoutes(v1, h.Task)
	RegisterObservationRoutes(v1, h.Observation)
	return engine
}
