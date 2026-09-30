// Package authn 提供 JWT 签发与解析，供 identity 登录模块签发、middleware 校验使用。
package authn

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

var (
	// ErrInvalidToken token 无效或过期。
	ErrInvalidToken = errors.New("authn: invalid token")
)

// Claims 是业务 token 的载荷。
type Claims struct {
	UserID int64    `json:"user_id"`
	Roles  []string `json:"roles"`
	jwt.RegisteredClaims
}

// TokenManager 负责签发与解析 token。
type TokenManager struct {
	secret []byte
	ttl    time.Duration
}

// NewTokenManager 构造 token 管理器。
func NewTokenManager(secret string, ttl time.Duration) *TokenManager {
	return &TokenManager{secret: []byte(secret), ttl: ttl}
}

// Sign 为用户签发 token。
func (m *TokenManager) Sign(userID int64, roles []string) (string, error) {
	now := time.Now()
	claims := Claims{
		UserID: userID,
		Roles:  roles,
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(m.ttl)),
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(m.secret)
}

// Parse 解析并校验 token，返回 Claims。
func (m *TokenManager) Parse(tokenStr string) (*Claims, error) {
	claims := &Claims{}
	token, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, ErrInvalidToken
		}
		return m.secret, nil
	})
	if err != nil || !token.Valid {
		return nil, ErrInvalidToken
	}
	return claims, nil
}
