// Package middleware 提供 HTTP 中间件。
package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// CORS 允许跨域访问。
// 客户端包括：PC 管理端 dev（http://localhost:*）、微信小程序/App（无 Origin 或 file:// 下 Origin 为 null）。
// 不携带 Cookie 的鉴权（Authorization 头），放开全部 Origin 即可；生产可按配置收紧。
func CORS() gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if origin == "" {
			origin = "*"
		}
		c.Header("Access-Control-Allow-Origin", origin)
		c.Header("Vary", "Origin")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Authorization, Content-Type, X-Request-Id, Idempotency-Key")
		c.Header("Access-Control-Expose-Headers", "X-Request-Id, Content-Disposition")
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}
