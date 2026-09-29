// Package api 提供 HTTP Handler 层。
// Handler 只负责参数绑定、调用 Service 并组织响应，不直接调用 GORM。
package api

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"

	"citywall/server/internal/middleware"
	"citywall/server/internal/module/task"
)

// TaskHandler 任务相关接口。
type TaskHandler struct {
	service task.Service
}

// NewTaskHandler 构造任务 Handler。
func NewTaskHandler(svc task.Service) *TaskHandler {
	return &TaskHandler{service: svc}
}

type createTaskRequest struct {
	Title        string  `json:"title" binding:"required"`
	AssigneeID   int64   `json:"assignee_id" binding:"required"`
	DueAt        string  `json:"due_at"` // ISO 8601，可选
	Description  string  `json:"description"`
	PointIDs     []int64 `json:"point_ids" binding:"required,min=1"`
}

// Create 创建任务草稿（管理员）。响应结构见 openapi.yaml。
func (h *TaskHandler) Create(c *gin.Context) {
	var req createTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeError(c, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return
	}

	// 初始化阶段身份模块未接入，先以 0 表示未知操作者；接入后从 middleware.GetActor 读取。
	t, err := h.service.Create(context.Background(), 0, req.Title, req.AssigneeID, req.DueAt, req.Description, req.PointIDs)
	if err != nil {
		// TODO(init): 按错误类型映射 404/409 等；ErrNotImplemented 先返回 501。
		writeError(c, http.StatusNotImplemented, "NOT_IMPLEMENTED", err.Error())
		return
	}
	writeData(c, http.StatusCreated, gin.H{"task": t})
}

// 统一响应封装。
func writeData(c *gin.Context, status int, data gin.H) {
	c.JSON(status, gin.H{"data": data, "request_id": middleware.GetRequestID(c)})
}

func writeError(c *gin.Context, status int, code, message string) {
	c.JSON(status, gin.H{
		"error": gin.H{
			"code":    code,
			"message": message,
		},
		"request_id": middleware.GetRequestID(c),
	})
}
