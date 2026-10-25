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
	actor, _ := middleware.GetActor(c)
	pts, err := h.service.ListPoints(c.Request.Context(), !(hasAnyRole(actor, "ADMIN") && c.Query("include_inactive") == "true"))
	if err != nil {
		writeErr(c, err)
		return
	}
	writeData(c, http.StatusOK, gin.H{"items": pts})
}

// ListSegments 区段列表。
func (h *SiteHandler) ListSegments(c *gin.Context) {
	actor, _ := middleware.GetActor(c)
	segs, err := h.service.ListSegments(c.Request.Context(), !(hasAnyRole(actor, "ADMIN") && c.Query("include_inactive") == "true"))
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

func (h *SiteHandler) UpdatePoint(c *gin.Context) {
	actor, _ := middleware.GetActor(c)
	if !hasAnyRole(actor, "ADMIN") {
		writeError(c, http.StatusForbidden, "FORBIDDEN", "admin required")
		return
	}
	id, err := pathID(c)
	if err != nil {
		writeError(c, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return
	}
	var input site.CreatePointInput
	if err := c.ShouldBindJSON(&input); err != nil {
		writeError(c, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return
	}
	point, err := h.service.UpdatePoint(c.Request.Context(), id, input)
	if err != nil {
		writeErr(c, err)
		return
	}
	writeData(c, http.StatusOK, gin.H{"point": point})
}

func (h *SiteHandler) SaveSegment(c *gin.Context) {
	actor, _ := middleware.GetActor(c)
	if !hasAnyRole(actor, "ADMIN") {
		writeError(c, http.StatusForbidden, "FORBIDDEN", "admin required")
		return
	}
	var input site.SegmentInput
	if err := c.ShouldBindJSON(&input); err != nil {
		writeError(c, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return
	}
	var segment *site.WallSegment
	var err error
	status := http.StatusCreated
	if c.Param("id") == "" {
		segment, err = h.service.CreateSegment(c.Request.Context(), input)
	} else {
		id, parseErr := pathID(c)
		if parseErr != nil {
			writeError(c, http.StatusBadRequest, "VALIDATION_ERROR", parseErr.Error())
			return
		}
		segment, err = h.service.UpdateSegment(c.Request.Context(), id, input)
		status = http.StatusOK
	}
	if err != nil {
		writeErr(c, err)
		return
	}
	writeData(c, status, gin.H{"segment": segment})
}
