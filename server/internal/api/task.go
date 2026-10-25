// Package api 提供 HTTP Handler 层。
// Handler 只负责参数绑定、调用 Service 并组织响应，不直接调用 GORM。
package api

import (
	"context"
	"net/http"
	"strconv"

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

func (h *TaskHandler) ListAssignees(c *gin.Context) {
	actor, ok := middleware.GetActor(c)
	if !ok {
		writeError(c, http.StatusUnauthorized, "UNAUTHORIZED", "not authenticated")
		return
	}
	users, err := h.service.ListAssignees(c.Request.Context(), actor)
	if err != nil {
		writeErr(c, err)
		return
	}
	writeData(c, http.StatusOK, gin.H{"items": users})
}

// Create 创建任务草稿（管理员）。响应结构见 openapi.yaml。
func (h *TaskHandler) Create(c *gin.Context) {
	actor, ok := middleware.GetActor(c)
	if !ok {
		writeError(c, http.StatusUnauthorized, "UNAUTHORIZED", "not authenticated")
		return
	}
	var in task.CreateInput
	if err := c.ShouldBindJSON(&in); err != nil {
		writeError(c, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return
	}
	t, err := h.service.Create(context.Background(), actor, in)
	if err != nil {
		writeErr(c, err)
		return
	}
	writeData(c, http.StatusCreated, gin.H{"task": t})
}

// Dispatch 派发任务（管理员）。
func (h *TaskHandler) Dispatch(c *gin.Context) {
	actor, ok := middleware.GetActor(c)
	if !ok {
		writeError(c, http.StatusUnauthorized, "UNAUTHORIZED", "not authenticated")
		return
	}
	id, revision, ok := idAndRevision(c)
	if !ok {
		return
	}
	t, err := h.service.Dispatch(context.Background(), actor, id, revision)
	if err != nil {
		writeErr(c, err)
		return
	}
	writeData(c, http.StatusOK, gin.H{"task": t})
}

// List 任务列表（按权限过滤，支持 status/page/page_size）。
func (h *TaskHandler) List(c *gin.Context) {
	actor, ok := middleware.GetActor(c)
	if !ok {
		writeError(c, http.StatusUnauthorized, "UNAUTHORIZED", "not authenticated")
		return
	}
	in := task.ListInput{
		Status:   c.Query("status"),
		Page:     queryInt(c, "page", 1),
		PageSize: queryInt(c, "page_size", 20),
	}
	res, err := h.service.List(context.Background(), actor, in)
	if err != nil {
		writeErr(c, err)
		return
	}
	writeData(c, http.StatusOK, gin.H{"tasks": res.Tasks, "total": res.Total, "page": in.Page, "page_size": in.PageSize})
}

// Get 任务详情与任务点。
func (h *TaskHandler) Get(c *gin.Context) {
	actor, ok := middleware.GetActor(c)
	if !ok {
		writeError(c, http.StatusUnauthorized, "UNAUTHORIZED", "not authenticated")
		return
	}
	id, err := pathID(c)
	if err != nil {
		writeError(c, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return
	}
	t, points, err := h.service.Get(context.Background(), actor, id)
	if err != nil {
		writeErr(c, err)
		return
	}
	writeData(c, http.StatusOK, gin.H{"task": t, "points": points})
}

// SubmitTask 提交整任务（负责人）。
func (h *TaskHandler) SubmitTask(c *gin.Context) {
	actor, ok := middleware.GetActor(c)
	if !ok {
		writeError(c, http.StatusUnauthorized, "UNAUTHORIZED", "not authenticated")
		return
	}
	id, revision, ok := idAndRevision(c)
	if !ok {
		return
	}
	t, err := h.service.SubmitTask(context.Background(), actor, id, revision)
	if err != nil {
		writeErr(c, err)
		return
	}
	writeData(c, http.StatusOK, gin.H{"task": t})
}

type completeTaskRequest struct {
	Revision  int64  `json:"revision" binding:"required"`
	FinalNote string `json:"final_note"`
}

// Complete 完成任务（审核员）。
func (h *TaskHandler) Complete(c *gin.Context) {
	actor, ok := middleware.GetActor(c)
	if !ok {
		writeError(c, http.StatusUnauthorized, "UNAUTHORIZED", "not authenticated")
		return
	}
	id, err := pathID(c)
	if err != nil {
		writeError(c, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return
	}
	var req completeTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeError(c, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return
	}
	t, err := h.service.Complete(context.Background(), actor, id, req.Revision, req.FinalNote)
	if err != nil {
		writeErr(c, err)
		return
	}
	writeData(c, http.StatusOK, gin.H{"task": t})
}

type cancelTaskRequest struct {
	Revision int64  `json:"revision" binding:"required"`
	Reason   string `json:"reason" binding:"required"`
}

// Cancel 取消任务（管理员；非终态可取消，已取消幂等）。
func (h *TaskHandler) Cancel(c *gin.Context) {
	actor, ok := middleware.GetActor(c)
	if !ok {
		writeError(c, http.StatusUnauthorized, "UNAUTHORIZED", "not authenticated")
		return
	}
	id, err := pathID(c)
	if err != nil {
		writeError(c, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return
	}
	var req cancelTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeError(c, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return
	}
	t, err := h.service.Cancel(context.Background(), actor, id, req.Revision, req.Reason)
	if err != nil {
		writeErr(c, err)
		return
	}
	writeData(c, http.StatusOK, gin.H{"task": t})
}

// idAndRevision 解析路径 id 与请求体 revision。
func idAndRevision(c *gin.Context) (int64, int64, bool) {
	id, err := pathID(c)
	if err != nil {
		writeError(c, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return 0, 0, false
	}
	var body struct {
		Revision int64 `json:"revision" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		writeError(c, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return 0, 0, false
	}
	return id, body.Revision, true
}

func pathID(c *gin.Context) (int64, error) {
	return strconv.ParseInt(c.Param("id"), 10, 64)
}

// queryInt 读取查询参数整数，缺失或非法时返回默认值。
func queryInt(c *gin.Context, key string, def int) int {
	raw := c.Query(key)
	if raw == "" {
		return def
	}
	v, err := strconv.Atoi(raw)
	if err != nil || v < 1 {
		return def
	}
	return v
}
