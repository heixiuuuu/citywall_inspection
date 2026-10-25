package api

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"

	"citywall/server/internal/middleware"
	"citywall/server/internal/module/identity"
)

// IdentityHandler 登录与当前用户接口。
type IdentityHandler struct {
	service identity.Service
}

// NewIdentityHandler 构造身份 Handler。
func NewIdentityHandler(svc identity.Service) *IdentityHandler {
	return &IdentityHandler{service: svc}
}

type loginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// Login PC 账号密码登录。
func (h *IdentityHandler) Login(c *gin.Context) {
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeError(c, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return
	}
	sess, err := h.service.Login(context.Background(), req.Username, req.Password)
	if err != nil {
		switch err {
		case identity.ErrInvalidCredentials:
			writeError(c, http.StatusUnauthorized, "INVALID_CREDENTIALS", err.Error())
		case identity.ErrUserDisabled:
			writeError(c, http.StatusForbidden, "USER_DISABLED", err.Error())
		default:
			writeErr(c, err)
		}
		return
	}
	writeData(c, http.StatusOK, gin.H{"token": sess.Token, "user": sess.User, "roles": sess.Roles})
}

type wechatLoginRequest struct {
	Code string `json:"code" binding:"required"`
}

// WechatLogin 小程序微信登录。
func (h *IdentityHandler) WechatLogin(c *gin.Context) {
	var req wechatLoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeError(c, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return
	}
	sess, err := h.service.WechatLogin(context.Background(), req.Code)
	if err != nil {
		switch err {
		case identity.ErrWechatCodeInvalid, identity.ErrUserNotFound:
			writeError(c, http.StatusUnauthorized, "INVALID_CODE", err.Error())
		default:
			writeErr(c, err)
		}
		return
	}
	writeData(c, http.StatusOK, gin.H{"token": sess.Token, "user": sess.User, "roles": sess.Roles})
}

// Me 当前用户及权限。
func (h *IdentityHandler) Me(c *gin.Context) {
	actor, ok := middleware.GetActor(c)
	if !ok {
		writeError(c, http.StatusUnauthorized, "UNAUTHORIZED", "not authenticated")
		return
	}
	u, roles, err := h.service.GetUser(context.Background(), actor.UserID)
	if err != nil {
		writeErr(c, err)
		return
	}
	writeData(c, http.StatusOK, gin.H{"user": u, "roles": roles})
}
