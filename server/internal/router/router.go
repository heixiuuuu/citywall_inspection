// Package router 集中注册 HTTP 路由。
// 只绑定 URL、方法、中间件与 Handler，不写业务状态规则。
package router

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"citywall/server/internal/api"
	"citywall/server/internal/authn"
	"citywall/server/internal/middleware"
)

// Handlers 聚合各模块 Handler，供 Router 注册。
type Handlers struct {
	Identity    *api.IdentityHandler
	Site        *api.SiteHandler
	Task        *api.TaskHandler
	Observation *api.ObservationHandler
}

// Router 构建 gin 引擎：全局中间件 + 健康检查 + /api/v1 分组。
func Router(h Handlers, tm *authn.TokenManager, reqIDHeader string) *gin.Engine {
	engine := gin.New()
	engine.Use(middleware.Recovery(), middleware.RequestID(reqIDHeader))

	engine.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	authed := middleware.Auth(tm)

	v1 := engine.Group("/api/v1")
	{
		// 身份：登录无需认证，其余需 token。
		v1.POST("/auth/login", h.Identity.Login)
		v1.POST("/auth/wechat-login", h.Identity.WechatLogin)
		v1.GET("/me", authed, h.Identity.Me)

		// 巡检范围。
		v1.GET("/inspection-points", authed, h.Site.ListPoints)
		v1.GET("/wall-segments", authed, h.Site.ListSegments)
		v1.POST("/inspection-points", authed, h.Site.CreatePoint)

		// 任务。
		taskGroup := v1.Group("/tasks", authed)
		taskGroup.POST("", h.Task.Create)
		taskGroup.POST("/:id/dispatch", h.Task.Dispatch)
		taskGroup.GET("", h.Task.List)
		taskGroup.GET("/:id", h.Task.Get)
		taskGroup.POST("/:id/submit", h.Task.SubmitTask)
		taskGroup.POST("/:id/complete", h.Task.Complete)

		// 现场记录与图片。
		obsGroup := v1.Group("/observations", authed)
		obsGroup.POST("", h.Observation.Create)
		obsGroup.PATCH("/:id", h.Observation.Update)
		obsGroup.POST("/:id/images", h.Observation.AddImage)
		obsGroup.PUT("/:id/conclusion", h.Observation.SaveConclusion)
		obsGroup.POST("/:id/submit", h.Observation.Submit)
		obsGroup.POST("/:id/review", h.Observation.Review)

		imgGroup := v1.Group("/images", authed)
		imgGroup.GET("/:id/content", h.Observation.GetImageContent)
		imgGroup.POST("/:id/recognitions", h.Observation.Recognize)

		// 审核队列。
		v1.GET("/reviews", authed, h.Observation.ListPendingReview)
	}
	return engine
}
