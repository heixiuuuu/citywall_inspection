package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// UserIDKey 是 context 中当前用户 ID 的键。
const UserIDKey = "user_id"

// Actor 表示认证后的用户上下文。
type Actor struct {
	UserID int64    `json:"user_id"`
	Roles  []string `json:"roles"`
}

// Auth 中间件骨架：从 Authorization: Bearer <token> 提取 token，
// 解析后将 Actor 放入 context。
//
// 说明（初始化阶段）：
//   - 登录与 JWT 签发待 identity 模块实现，本中间件先完成 token 提取与占位校验；
//   - 前端路由守卫只能隐藏不可操作页面，真正的授权校验必须由服务端完成。
func Auth() gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		if header == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": gin.H{
					"code":    "UNAUTHORIZED",
					"message": "missing bearer token",
				},
				"request_id": GetRequestID(c),
			})
			return
		}

		const prefix = "Bearer "
		if !strings.HasPrefix(header, prefix) {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": gin.H{
					"code":    "UNAUTHORIZED",
					"message": "invalid authorization scheme",
				},
				"request_id": GetRequestID(c),
			})
			return
		}

		token := strings.TrimPrefix(header, prefix)
		if token == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": gin.H{
					"code":    "UNAUTHORIZED",
					"message": "empty bearer token",
				},
				"request_id": GetRequestID(c),
			})
			return
		}

		// TODO(init): 解析 JWT（identity 模块完成后接入），把 Actor 写入 context。
		c.Set("actor_token", token)
		c.Next()
	}
}

// GetActor 读取当前用户上下文。
func GetActor(c *gin.Context) (*Actor, bool) {
	v, ok := c.Get("actor")
	if !ok {
		return nil, false
	}
	a, ok := v.(*Actor)
	return a, ok
}
