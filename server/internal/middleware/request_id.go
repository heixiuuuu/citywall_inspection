// Package middleware 提供请求级认证与日志中间件。
package middleware

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"

	"github.com/gin-gonic/gin"
)

// RequestIDKey 是 context 中 request_id 的键。
const RequestIDKey = "request_id"

// RequestID 中间件：接收或生成 X-Request-Id，写入响应头与 context，供日志关联。
func RequestID(headerName string) gin.HandlerFunc {
	return func(c *gin.Context) {
		rid := c.GetHeader(headerName)
		if rid == "" {
			rid = newRequestID()
		}
		c.Set(RequestIDKey, rid)
		c.Writer.Header().Set(headerName, rid)
		c.Next()
	}
}

// GetRequestID 从 context 读取 request_id。
func GetRequestID(c *gin.Context) string {
	if v, ok := c.Get(RequestIDKey); ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

func newRequestID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return ""
	}
	return hex.EncodeToString(b)
}

// 防止 http 包未使用（后续扩展中间件时移除）。
var _ = http.StatusInternalServerError
