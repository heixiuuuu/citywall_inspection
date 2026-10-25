package api

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"citywall/server/internal/middleware"
	"citywall/server/internal/module/audit"
	"citywall/server/internal/module/issue"
	"citywall/server/internal/module/observation"
	"citywall/server/internal/module/report"
	"citywall/server/internal/module/site"
	"citywall/server/internal/module/task"
	mysqldriver "github.com/go-sql-driver/mysql"
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
	var dbErr *mysqldriver.MySQLError
	if errors.As(err, &dbErr) && dbErr.Number == 1062 {
		writeError(c, http.StatusConflict, "CONFLICT", "record already exists")
		return
	}
	switch {
	case err == nil:
		return
	case errors.Is(err, site.ErrInvalidInput), errors.Is(err, task.ErrInvalidAssignee):
		writeError(c, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
	case errors.Is(err, site.ErrPointNotFound), errors.Is(err, site.ErrSegmentNotFound):
		writeError(c, http.StatusNotFound, "NOT_FOUND", err.Error())
	case errors.Is(err, task.ErrNotFound), errors.Is(err, observation.ErrNotFound), errors.Is(err, issue.ErrNotFound),
		errors.Is(err, report.ErrNotFound):
		writeError(c, http.StatusNotFound, "NOT_FOUND", err.Error())
	case errors.Is(err, task.ErrForbidden), errors.Is(err, observation.ErrForbidden), errors.Is(err, audit.ErrForbidden),
		errors.Is(err, issue.ErrForbidden), errors.Is(err, report.ErrForbidden):
		writeError(c, http.StatusForbidden, "FORBIDDEN", err.Error())
	case errors.Is(err, audit.ErrDBUnavailable):
		writeError(c, http.StatusServiceUnavailable, "DB_UNAVAILABLE", err.Error())
	case errors.Is(err, report.ErrNotReady):
		writeError(c, http.StatusConflict, "NOT_READY", err.Error())
	case errors.Is(err, task.ErrStatusConflict), errors.Is(err, observation.ErrStatusConflict),
		errors.Is(err, task.ErrInvalidState), errors.Is(err, observation.ErrInvalidState),
		errors.Is(err, task.ErrDuplicatePoint), errors.Is(err, task.ErrNoPoints),
		errors.Is(err, task.ErrUnfinishedPoints), errors.Is(err, observation.ErrDuplicateRecord),
		errors.Is(err, observation.ErrSelfReview), errors.Is(err, issue.ErrInvalidState),
		errors.Is(err, issue.ErrStatusConflict), errors.Is(err, issue.ErrSelfVerify),
		errors.Is(err, report.ErrBadState):
		writeError(c, http.StatusConflict, "CONFLICT", err.Error())
	case errors.Is(err, task.ErrPointNotFound), errors.Is(err, observation.ErrSourceConstraint),
		errors.Is(err, observation.ErrMissingOverview), errors.Is(err, observation.ErrMissingConclusion),
		errors.Is(err, observation.ErrMissingLocation), errors.Is(err, observation.ErrReturnReasonRequired),
		errors.Is(err, observation.ErrRejectOnlyVoluntary), errors.Is(err, issue.ErrReasonRequired),
		errors.Is(err, issue.ErrAdmitEmpty), errors.Is(err, issue.ErrAdmitNotAccepted),
		errors.Is(err, issue.ErrActionImageMiss), errors.Is(err, report.ErrInvalidScope):
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
