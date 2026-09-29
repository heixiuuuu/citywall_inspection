package router

import (
	"github.com/gin-gonic/gin"

	"citywall/server/internal/api"
)

// RegisterObservationRoutes 注册现场记录路由（03 文档 6.2）。
func RegisterObservationRoutes(g *gin.RouterGroup, h *api.ObservationHandler) {
	// 首条链路：创建草稿。图片上传、结论、提交、审核等随模块实现逐步补充。
	g.POST("/observations", h.Create)
}
