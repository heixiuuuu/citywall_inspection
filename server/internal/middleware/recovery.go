package middleware

import (
	"log"
	"net/http"
	"runtime/debug"

	"github.com/gin-gonic/gin"
)

// Recovery 中间件：捕获 panic，记录 request_id 与堆栈，返回 500 统一错误响应。
func Recovery() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("panic recovered: %v\nrequest_id=%s\n%s",
					r, GetRequestID(c), string(debug.Stack()))
				c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
					"error": gin.H{
						"code":    "INTERNAL_ERROR",
						"message": "internal server error",
					},
					"request_id": GetRequestID(c),
				})
			}
		}()
		c.Next()
	}
}
