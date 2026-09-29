package api

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"

	"citywall/server/internal/module/observation"
)

// ObservationHandler 现场记录相关接口。
type ObservationHandler struct {
	service observation.Service
}

// NewObservationHandler 构造现场记录 Handler。
func NewObservationHandler(svc observation.Service) *ObservationHandler {
	return &ObservationHandler{service: svc}
}

type createObservationRequest struct {
	Source       string `json:"source" binding:"required,oneof=TASK VOLUNTARY"`
	TaskPointID  *int64 `json:"task_point_id"`
	LocationText string `json:"location_text" binding:"required"`
}

// Create 创建现场记录草稿（小程序）。source=TASK 必须带 task_point_id。
func (h *ObservationHandler) Create(c *gin.Context) {
	var req createObservationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeError(c, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return
	}

	o, err := h.service.Create(context.Background(), 0, req.Source, req.TaskPointID, req.LocationText)
	if err != nil {
		writeError(c, http.StatusNotImplemented, "NOT_IMPLEMENTED", err.Error())
		return
	}
	writeData(c, http.StatusCreated, gin.H{"observation": o})
}
