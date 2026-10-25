// Package router 集中注册 HTTP 路由。
// 只绑定 URL、方法、中间件与 Handler，不写业务状态规则。
package router

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"citywall/server/internal/api"
	"citywall/server/internal/authn"
	"citywall/server/internal/middleware"
	"citywall/server/internal/module/idempotency"
)

// Handlers 聚合各模块 Handler，供 Router 注册。
type Handlers struct {
	Identity    *api.IdentityHandler
	Site        *api.SiteHandler
	Task        *api.TaskHandler
	Observation *api.ObservationHandler
	Audit       *api.AuditHandler
	Issue       *api.IssueHandler
	Report      *api.ReportHandler
}

// Router 构建 gin 引擎：全局中间件 + 健康检查 + /api/v1 分组。
// idemSvc 为落库幂等服务（可为 nil：不启用 DB 幂等中间件）。
func Router(h Handlers, tm *authn.TokenManager, reqIDHeader string, idemSvc idempotency.Service, idemHeader string) *gin.Engine {
	engine := gin.New()
	engine.Use(middleware.Recovery(), middleware.RequestID(reqIDHeader), middleware.CORS())

	engine.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	authed := middleware.Auth(tm)
	idem := middleware.Idempotency(idemSvc, idemHeader)

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
		v1.PUT("/inspection-points/:id", authed, h.Site.UpdatePoint)
		v1.POST("/wall-segments", authed, h.Site.SaveSegment)
		v1.PUT("/wall-segments/:id", authed, h.Site.SaveSegment)

		// 任务（派发/提交/完成等易重复操作挂幂等中间件）。
		taskGroup := v1.Group("/tasks", authed)
		taskGroup.POST("", h.Task.Create)
		taskGroup.POST("/:id/dispatch", idem, h.Task.Dispatch)
		taskGroup.GET("", h.Task.List)
		taskGroup.GET("/assignees", h.Task.ListAssignees)
		taskGroup.GET("/:id", h.Task.Get)
		taskGroup.POST("/:id/submit", idem, h.Task.SubmitTask)
		taskGroup.POST("/:id/complete", idem, h.Task.Complete)
		taskGroup.POST("/:id/cancel", idem, h.Task.Cancel)

		// 现场记录与图片（上传幂等在 observation service 落库）。
		obsGroup := v1.Group("/observations", authed)
		obsGroup.POST("", h.Observation.Create)
		obsGroup.GET("", h.Observation.List)
		obsGroup.GET("/:id", h.Observation.Evidence)
		obsGroup.PATCH("/:id", h.Observation.Update)
		obsGroup.POST("/:id/images", h.Observation.AddImage)
		obsGroup.GET("/:id/images", h.Observation.ListImages)
		obsGroup.DELETE("/:id/images/:image_id", h.Observation.RemoveImage)
		obsGroup.PUT("/:id/conclusion", h.Observation.SaveConclusion)
		obsGroup.POST("/:id/submit", idem, h.Observation.Submit)
		obsGroup.POST("/:id/review", idem, h.Observation.Review)

		imgGroup := v1.Group("/images", authed)
		imgGroup.GET("/:id/content", h.Observation.GetImageContent)
		imgGroup.POST("/:id/recognitions", h.Observation.Recognize)

		// 审核队列。
		v1.GET("/reviews", authed, h.Observation.ListPendingReview)

		// 问题台账与处置（管理员/审核员/责任人按权限）。
		if h.Issue != nil {
			issueGroup := v1.Group("/issues", authed)
			issueGroup.GET("", h.Issue.List)
			issueGroup.GET("/:id", h.Issue.Get)
			issueGroup.POST("/:id/assign", idem, h.Issue.Assign)
			issueGroup.POST("/:id/actions", idem, h.Issue.AddAction)
			issueGroup.POST("/:id/actions/:action_id/images", h.Issue.AddActionImage)
			issueGroup.POST("/:id/request-verification", idem, h.Issue.RequestVerification)
			issueGroup.POST("/:id/verify", idem, h.Issue.Verify)
			issueGroup.POST("/:id/reopen", idem, h.Issue.Reopen)
			// 审核通过记录的受理：新建/关联问题或记录不立项原因。
			v1.POST("/observations/:id/admit", authed, idem, h.Issue.AdmitFromObservation)
		}

		// 审计事件（只读，仅管理员）。
		auditGroup := v1.Group("/audit", authed)
		auditGroup.GET("/events", h.Audit.ListEvents)

		// 报告（管理员/审核员）：生成、台账、重试与 PDF 下载。
		if h.Report != nil {
			reportGroup := v1.Group("/reports", authed)
			reportGroup.POST("/task/:task_id", h.Report.CreateTask)
			reportGroup.POST("/range", h.Report.CreateRange)
			reportGroup.GET("", h.Report.List)
			reportGroup.GET("/:id", h.Report.Get)
			reportGroup.POST("/:id/retry", h.Report.Retry)
			reportGroup.GET("/:id/file", h.Report.File)
		}
	}
	return engine
}
