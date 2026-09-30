package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"citywall/server/internal/authn"
)

// UserIDKey 是 context 中当前用户 ID 的键。
const UserIDKey = "user_id"

// Actor 表示认证后的用户上下文。
type Actor struct {
	UserID int64    `json:"user_id"`
	Roles  []string `json:"roles"`
}

// Auth 中间件：解析 Authorization: Bearer <token>，
// 校验 JWT 后把 Actor 放入 context；无效 token 返回 401。
//
// 说明：前端路由守卫只能隐藏不可操作页面，真正的授权校验必须由服务端完成；
// 角色级权限由各 Service 在业务方法中校验。
func Auth(tm *authn.TokenManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		if header == "" {
			abortUnauthorized(c, "missing bearer token")
			return
		}

		const prefix = "Bearer "
		if !strings.HasPrefix(header, prefix) {
			abortUnauthorized(c, "invalid authorization scheme")
			return
		}

		tokenStr := strings.TrimPrefix(header, prefix)
		if tokenStr == "" {
			abortUnauthorized(c, "empty bearer token")
			return
		}

		claims, err := tm.Parse(tokenStr)
		if err != nil {
			abortUnauthorized(c, "invalid or expired token")
			return
		}

		c.Set("actor", &Actor{UserID: claims.UserID, Roles: claims.Roles})
		c.Set(UserIDKey, claims.UserID)
		c.Next()
	}
}

// GetActor 读取当前用户上下文；未认证返回 nil,false。
func GetActor(c *gin.Context) (*Actor, bool) {
	v, ok := c.Get("actor")
	if !ok {
		return nil, false
	}
	a, ok := v.(*Actor)
	return a, ok
}

func abortUnauthorized(c *gin.Context, message string) {
	c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
		"error": gin.H{
			"code":    "UNAUTHORIZED",
			"message": message,
		},
		"request_id": GetRequestID(c),
	})
}
