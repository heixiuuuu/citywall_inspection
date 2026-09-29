package router

import (
	"github.com/gin-gonic/gin"

	"citywall/server/internal/api"
)

// RegisterTaskRoutes 注册任务路由（03 文档 6.2）。
func RegisterTaskRoutes(g *gin.RouterGroup, h *api.TaskHandler) {
	// 首条链路：创建草稿。派发、提交、完成、取消等随模块实现逐步补充。
	g.POST("/tasks", h.Create)
}
