// Package middleware 提供 Gin 中间件：请求关联 ID、恢复、认证与幂等。
package middleware

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"

	"github.com/gin-gonic/gin"

	"citywall/server/internal/module/idempotency"
)

// Idempotency 中间件：读取幂等键请求头，同 actor+operation+key 的重复请求
// 直接短路返回（不重复执行，无副作用）。
// 幂等记录在 handler 执行成功（2xx）后才补写；执行失败不写，允许同键重试。
// 图片上传不做中间件幂等（其幂等已在 observation service 落库处理，避免中间件
// 吞掉响应中的图片 ID）。
func Idempotency(svc idempotency.Service, headerName string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if svc == nil || headerName == "" {
			c.Next()
			return
		}
		key := c.GetHeader(headerName)
		if key == "" {
			c.Next()
			return
		}
		actor, ok := GetActor(c)
		if !ok {
			c.Next()
			return
		}
		op := c.Request.Method + " " + c.FullPath()
		done, _, err := svc.Check(context.Background(), actor.UserID, op, key)
		if err != nil {
			// 幂等存储故障：放行执行（降级，不阻断业务）。
			c.Next()
			return
		}
		if done {
			c.JSON(http.StatusOK, gin.H{
				"data":       gin.H{"idempotent": true, "replayed": true},
				"request_id": GetRequestID(c),
			})
			c.Abort()
			return
		}
		c.Next()
		// handler 成功（2xx）且未被后续短路 → 补写幂等记录。
		status := c.Writer.Status()
		if status >= http.StatusOK && status < http.StatusMultipleChoices && !c.IsAborted() {
			sum := sha256.Sum256([]byte(op + "|" + key))
			reqHash := hex.EncodeToString(sum[:])
			_ = svc.Record(context.Background(), actor.UserID, op, key, reqHash)
		}
	}
}
