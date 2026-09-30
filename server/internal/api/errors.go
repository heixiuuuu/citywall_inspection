package api

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"citywall/server/internal/middleware"
	"citywall/server/internal/module/audit"
	"citywall/server/internal/module/observation"
	"citywall/server/internal/module/task"
)

// writeData 统一成功响应：{"data": ..., "request_id": ...}。
func writeData(c *gin.Context, status int, data gin.H) {
	c.JSON(status, gin.H{"data": data, "request_id": middleware.GetRequestID(c)})
}

// writeError 统一失败响应：{"error": {...}, "request_id": ...}。
func writeError(c *gin.Context, status int, code, message string) {
	c.JSON(status, gin.H{
		"error": gin.H{
			"code":    code,
			"message": message,
		},
		"request_id": middleware.GetRequestID(c),
	})
}

// writeErr 将领域错误映射为 HTTP 错误。
func writeErr(c *gin.Context, err error) {
	switch {
	case err == nil:
		return
	case errors.Is(err, task.ErrNotFound), errors.Is(err, observation.ErrNotFound):
		writeError(c, http.StatusNotFound, "NOT_FOUND", err.Error())
	case errors.Is(err, task.ErrForbidden), errors.Is(err, observation.ErrForbidden), errors.Is(err, audit.ErrForbidden):
		writeError(c, http.StatusForbidden, "FORBIDDEN", err.Error())
	case errors.Is(err, audit.ErrDBUnavailable):
		writeError(c, http.StatusServiceUnavailable, "DB_UNAVAILABLE", err.Error())
	case errors.Is(err, task.ErrStatusConflict), errors.Is(err, observation.ErrStatusConflict),
		errors.Is(err, task.ErrInvalidState), errors.Is(err, observation.ErrInvalidState),
		errors.Is(err, task.ErrDuplicatePoint), errors.Is(err, task.ErrNoPoints),
		errors.Is(err, task.ErrUnfinishedPoints), errors.Is(err, observation.ErrDuplicateRecord),
		errors.Is(err, observation.ErrSelfReview):
		writeError(c, http.StatusConflict, "CONFLICT", err.Error())
	case errors.Is(err, task.ErrPointNotFound), errors.Is(err, observation.ErrSourceConstraint),
		errors.Is(err, observation.ErrMissingOverview), errors.Is(err, observation.ErrMissingConclusion),
		errors.Is(err, observation.ErrMissingLocation), errors.Is(err, observation.ErrReturnReasonRequired),
		errors.Is(err, observation.ErrRejectOnlyVoluntary):
		writeError(c, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
	default:
		writeError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "internal server error")
	}
}

// hasAnyRole 判断 actor 是否包含任一角色。
func hasAnyRole(actor *middleware.Actor, roles ...string) bool {
	if actor == nil {
		return false
	}
	for _, have := range actor.Roles {
		for _, want := range roles {
			if have == want {
				return true
			}
		}
	}
	return false
}
