package api

import (
	"context"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"citywall/server/internal/middleware"
	"citywall/server/internal/module/report"
)

// ReportHandler 报告生成、台账与下载接口。
type ReportHandler struct {
	service report.Service
}

// NewReportHandler 构造报告 Handler。
func NewReportHandler(svc report.Service) *ReportHandler {
	return &ReportHandler{service: svc}
}

// CreateTask 生成单任务报告。
func (h *ReportHandler) CreateTask(c *gin.Context) {
	actor, ok := middleware.GetActor(c)
	if !ok {
		writeError(c, http.StatusUnauthorized, "UNAUTHORIZED", "not authenticated")
		return
	}
	taskID, err := strconv.ParseInt(c.Param("task_id"), 10, 64)
	if err != nil {
		writeError(c, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return
	}
	rep, err := h.service.CreateTask(context.Background(), actor, taskID)
	if err != nil {
		writeErr(c, err)
		return
	}
	writeData(c, http.StatusCreated, gin.H{"report": reportView(rep)})
}

type rangeRequest struct {
	Start string `json:"start" binding:"required"`
	End   string `json:"end" binding:"required"`
}

// CreateRange 生成时间范围报告。
func (h *ReportHandler) CreateRange(c *gin.Context) {
	actor, ok := middleware.GetActor(c)
	if !ok {
		writeError(c, http.StatusUnauthorized, "UNAUTHORIZED", "not authenticated")
		return
	}
	var req rangeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeError(c, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return
	}
	start, err := time.Parse(time.RFC3339, req.Start)
	if err != nil {
		writeError(c, http.StatusBadRequest, "VALIDATION_ERROR", "start must be RFC3339")
		return
	}
	end, err := time.Parse(time.RFC3339, req.End)
	if err != nil {
		writeError(c, http.StatusBadRequest, "VALIDATION_ERROR", "end must be RFC3339")
		return
	}
	rep, err := h.service.CreateRange(context.Background(), actor, start, end)
	if err != nil {
		writeErr(c, err)
		return
	}
	writeData(c, http.StatusCreated, gin.H{"report": reportView(rep)})
}

// List 报告台账（scope_type/task_id/page/page_size）。
func (h *ReportHandler) List(c *gin.Context) {
	actor, ok := middleware.GetActor(c)
	if !ok {
		writeError(c, http.StatusUnauthorized, "UNAUTHORIZED", "not authenticated")
		return
	}
	scopeType := c.Query("scope_type")
	var taskID *int64
	if v := c.Query("task_id"); v != "" {
		if id, err := strconv.ParseInt(v, 10, 64); err == nil {
			taskID = &id
		}
	}
	page := queryInt(c, "page", 1)
	pageSize := queryInt(c, "page_size", 20)
	res, err := h.service.List(context.Background(), actor, scopeType, taskID, page, pageSize)
	if err != nil {
		writeErr(c, err)
		return
	}
	items := make([]gin.H, 0, len(res.Items))
	for _, r := range res.Items {
		items = append(items, reportView(&r))
	}
	writeData(c, http.StatusOK, gin.H{"items": items, "total": res.Total, "page": page, "page_size": pageSize})
}

// Get 报告详情。
func (h *ReportHandler) Get(c *gin.Context) {
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
	rep, err := h.service.Get(context.Background(), actor, id)
	if err != nil {
		writeErr(c, err)
		return
	}
	writeData(c, http.StatusOK, gin.H{"report": reportView(rep)})
}

// Retry 重试失败报告。
func (h *ReportHandler) Retry(c *gin.Context) {
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
	rep, err := h.service.Retry(context.Background(), actor, id)
	if err != nil {
		writeErr(c, err)
		return
	}
	writeData(c, http.StatusOK, gin.H{"report": reportView(rep)})
}

// File 下载报告 PDF（鉴权后流式输出）。
func (h *ReportHandler) File(c *gin.Context) {
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
	info, f, err := h.service.OpenFile(context.Background(), actor, id)
	if err != nil {
		writeErr(c, err)
		return
	}
	defer f.Close()
	c.Header("Content-Type", info.MediaType)
	c.Header("Content-Disposition", `attachment; filename="report-`+strconv.FormatInt(id, 10)+`.pdf"`)
	c.Header("Content-Length", strconv.FormatInt(info.ByteSize, 10))
	c.Status(http.StatusOK)
	_, _ = io.Copy(c.Writer, f)
}

// reportView 输出视图（不暴露 storage_key 细节，只保留业务字段）。
func reportView(r *report.Report) gin.H {
	return gin.H{
		"id":           r.ID,
		"scope_type":   r.ScopeType,
		"task_id":      r.TaskID,
		"range_start":  r.RangeStart,
		"range_end":    r.RangeEnd,
		"generated_by": r.GeneratedBy,
		"snapshot_at":  r.SnapshotAt,
		"status":       r.Status,
		"error_code":   r.ErrorCode,
		"created_at":   r.CreatedAt,
		"finished_at":  r.FinishedAt,
		"download_url": "/api/v1/reports/" + strconv.FormatInt(r.ID, 10) + "/file",
	}
}
