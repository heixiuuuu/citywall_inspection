package api

import (
	"context"
	"net/http"
	"time"

	"citywall/server/internal/middleware"
	"citywall/server/internal/module/overview"
	"github.com/gin-gonic/gin"
)

type OverviewQuery interface {
	Load(context.Context, time.Time) (*overview.Summary, error)
}

type OverviewHandler struct{ query OverviewQuery }

func NewOverviewHandler(query OverviewQuery) *OverviewHandler { return &OverviewHandler{query: query} }

func (h *OverviewHandler) Get(c *gin.Context) {
	actor, ok := middleware.GetActor(c)
	if !ok {
		writeError(c, http.StatusUnauthorized, "UNAUTHORIZED", "not authenticated")
		return
	}
	if !actorHasRole(actor, "ADMIN") && !actorHasRole(actor, "REVIEWER") {
		writeError(c, http.StatusForbidden, "FORBIDDEN", "overview requires administrator or reviewer")
		return
	}
	if h.query == nil {
		writeError(c, http.StatusServiceUnavailable, "DB_UNAVAILABLE", "database unavailable")
		return
	}
	result, err := h.query.Load(c.Request.Context(), time.Now())
	if err != nil {
		writeErr(c, err)
		return
	}
	writeData(c, http.StatusOK, gin.H{"summary": result})
}
