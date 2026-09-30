package api

import (
	"context"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"citywall/server/internal/middleware"
	"citywall/server/internal/module/audit"
)

// AuditHandler 审计查询接口（只读，仅管理员）。
type AuditHandler struct {
	query *audit.QueryService
}

// NewAuditHandler 构造审计查询 Handler。
func NewAuditHandler(q *audit.QueryService) *AuditHandler {
	return &AuditHandler{query: q}
}

// ListEvents 分页查询审计事件。
// GET /api/v1/audit/events?entity_type=&entity_id=&actor_id=&page=&page_size=
func (h *AuditHandler) ListEvents(c *gin.Context) {
	actor, ok := middleware.GetActor(c)
	if !ok {
		writeError(c, http.StatusUnauthorized, "UNAUTHORIZED", "not authenticated")
		return
	}
	var entityID, actorID *int64
	if v := c.Query("entity_id"); v != "" {
		n, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			writeError(c, http.StatusBadRequest, "VALIDATION_ERROR", "entity_id must be int64")
			return
		}
		entityID = &n
	}
	if v := c.Query("actor_id"); v != "" {
		n, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			writeError(c, http.StatusBadRequest, "VALIDATION_ERROR", "actor_id must be int64")
			return
		}
		actorID = &n
	}
	f := audit.EventFilter{
		EntityType: c.Query("entity_type"),
		EntityID:   entityID,
		ActorID:    actorID,
	}
	page, pageSize := queryInt(c, "page", 1), queryInt(c, "page_size", 50)
	res, err := h.query.List(context.Background(), f, page, pageSize, actorHasRole(actor, "ADMIN"))
	if err != nil {
		writeErr(c, err)
		return
	}
	writeData(c, http.StatusOK, gin.H{"events": res.Items, "total": res.Total, "page": page, "page_size": pageSize})
}

// actorHasRole 判断当前登录用户是否拥有指定角色。
func actorHasRole(actor *middleware.Actor, role string) bool {
	if actor == nil {
		return false
	}
	for _, r := range actor.Roles {
		if r == role {
			return true
		}
	}
	return false
}
