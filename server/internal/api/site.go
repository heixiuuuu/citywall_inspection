package api

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"

	"citywall/server/internal/middleware"
	"citywall/server/internal/module/site"
)

// SiteHandler 区段与巡检点位接口。
type SiteHandler struct {
	service site.Service
}

// NewSiteHandler 构造点位 Handler。
func NewSiteHandler(svc site.Service) *SiteHandler {
	return &SiteHandler{service: svc}
}

// ListPoints 点位列表（巡检端只读，管理端维护）。
func (h *SiteHandler) ListPoints(c *gin.Context) {
	pts, err := h.service.ListPoints(context.Background(), true)
	if err != nil {
		writeErr(c, err)
		return
	}
	writeData(c, http.StatusOK, gin.H{"items": pts})
}

// ListSegments 区段列表。
func (h *SiteHandler) ListSegments(c *gin.Context) {
	segs, err := h.service.ListSegments(context.Background(), true)
	if err != nil {
		writeErr(c, err)
		return
	}
	writeData(c, http.StatusOK, gin.H{"items": segs})
}

// CreatePoint 创建巡检点位（管理员）。
func (h *SiteHandler) CreatePoint(c *gin.Context) {
	actor, ok := middleware.GetActor(c)
	if !ok {
		writeError(c, http.StatusUnauthorized, "UNAUTHORIZED", "not authenticated")
		return
	}
	if !hasAnyRole(actor, "ADMIN") {
		writeError(c, http.StatusForbidden, "FORBIDDEN", "admin required")
		return
	}
	var in site.CreatePointInput
	if err := c.ShouldBindJSON(&in); err != nil {
		writeError(c, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return
	}
	p, err := h.service.CreatePoint(context.Background(), in)
	if err != nil {
		writeErr(c, err)
		return
	}
	writeData(c, http.StatusCreated, gin.H{"point": p})
}
