// Package identity 承载用户身份与登录（PC 账号密码、小程序微信身份）。
package identity

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"time"

	"golang.org/x/crypto/bcrypt"

	"citywall/server/internal/authn"
	"citywall/server/internal/module/audit"
)

// 角色常量（01 文档：巡检员/管理员/审核员）。
const (
	RoleInspector = "INSPECTOR"
	RoleAdmin     = "ADMIN"
	RoleReviewer  = "REVIEWER"
)

var (
	// ErrInvalidCredentials 账号或密码错误。
	ErrInvalidCredentials = errors.New("identity: invalid credentials")
	// ErrUserDisabled 用户被禁用。
	ErrUserDisabled = errors.New("identity: user disabled")
	// ErrUserNotFound 用户不存在。
	ErrUserNotFound = errors.New("identity: user not found")
	// ErrWechatCodeInvalid 微信临时代码无效（MVP 测试身份映射失败）。
	ErrWechatCodeInvalid = errors.New("identity: wechat code invalid")
)

// User 业务用户。username 或 wechat_openid 至少一种身份。
type User struct {
	ID           int64     `gorm:"primaryKey;column:id" json:"id"`
	Username     *string   `gorm:"column:username;uniqueIndex" json:"username"`
	PasswordHash string    `gorm:"column:password_hash" json:"-"`
	WechatOpenid *string   `gorm:"column:wechat_openid;uniqueIndex" json:"wechat_openid"`
	DisplayName  string    `gorm:"column:display_name" json:"display_name"`
	Status       string    `gorm:"column:status" json:"status"` // ACTIVE / DISABLED
	CreatedAt    time.Time `gorm:"column:created_at" json:"created_at"`
	UpdatedAt    time.Time `gorm:"column:updated_at" json:"updated_at"`
}

// UserRole 用户角色（复合主键）。
type UserRole struct {
	UserID    int64     `gorm:"primaryKey;column:user_id" json:"user_id"`
	RoleCode  string    `gorm:"primaryKey;column:role_code" json:"role_code"`
	CreatedAt time.Time `gorm:"column:created_at" json:"created_at"`
}

// TableName 显式表名。
func (User) TableName() string     { return "users" }
func (UserRole) TableName() string { return "user_roles" }

// Session 登录返回结果。
type Session struct {
	Token string   `json:"token"`
	User  User     `json:"user"`
	Roles []string `json:"roles"`
}

// Repository 用户数据访问。
type Repository interface {
	// GetByUsername 按账号查询。
	GetByUsername(ctx context.Context, username string) (*User, error)
	// GetByWechatOpenid 按微信身份查询。
	GetByWechatOpenid(ctx context.Context, openid string) (*User, error)
	// GetByID 按 ID 查询。
	GetByID(ctx context.Context, id int64) (*User, error)
	// GetRoles 查询用户角色。
	GetRoles(ctx context.Context, userID int64) ([]string, error)
}

// Service 登录与用户查询。
type Service interface {
	// Login PC 账号密码登录。
	Login(ctx context.Context, username, password string) (*Session, error)
	// WechatLogin 交换微信 code 并查询已绑定人员。
	WechatLogin(ctx context.Context, code string) (*Session, error)
	// GetUser 返回用户与角色。
	GetUser(ctx context.Context, userID int64) (*User, []string, error)
	// HashPassword 生成密码哈希（seed 工具使用）。
	HashPassword(password string) (string, error)
}

type service struct {
	repo   Repository
	tm     *authn.TokenManager
	aud    audit.Recorder
	wechat WechatConfig
}

type WechatConfig struct {
	AppID  string
	Secret string
}

// NewService 构造身份服务。
func NewService(repo Repository, tm *authn.TokenManager, aud audit.Recorder, wechat ...WechatConfig) Service {
	if aud == nil {
		aud = audit.NewRecorder(nil)
	}
	svc := &service{repo: repo, tm: tm, aud: aud}
	if len(wechat) > 0 {
		svc.wechat = wechat[0]
	}
	return svc
}

func (s *service) Login(ctx context.Context, username, password string) (*Session, error) {
	u, err := s.repo.GetByUsername(ctx, username)
	if err != nil {
		return nil, ErrInvalidCredentials
	}
	if u.Status != "ACTIVE" {
		return nil, ErrUserDisabled
	}
	if bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password)) != nil {
		return nil, ErrInvalidCredentials
	}
	s.aud.Record(ctx, &audit.Event{
		EntityType: "user",
		EntityID:   u.ID,
		Action:     "login",
		ActorID:    u.ID,
		Reason:     username,
	})
	return s.buildSession(ctx, u)
}

func (s *service) WechatLogin(ctx context.Context, code string) (*Session, error) {
	if s.wechat.AppID == "" || s.wechat.Secret == "" {
		return nil, ErrWechatCodeInvalid
	}
	params := url.Values{"appid": {s.wechat.AppID}, "secret": {s.wechat.Secret}, "js_code": {code}, "grant_type": {"authorization_code"}}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.weixin.qq.com/sns/jscode2session?"+params.Encode(), nil)
	if err != nil {
		return nil, ErrWechatCodeInvalid
	}
	response, err := (&http.Client{Timeout: 10 * time.Second}).Do(req)
	if err != nil {
		return nil, ErrWechatCodeInvalid
	}
	defer response.Body.Close()
	var session struct {
		OpenID  string `json:"openid"`
		ErrCode int    `json:"errcode"`
	}
	if response.StatusCode != http.StatusOK || json.NewDecoder(response.Body).Decode(&session) != nil || session.ErrCode != 0 || session.OpenID == "" {
		return nil, ErrWechatCodeInvalid
	}
	openid := session.OpenID
	u, err := s.repo.GetByWechatOpenid(ctx, openid)
	if err != nil {
		return nil, ErrUserNotFound
	}
	if u.Status != "ACTIVE" {
		return nil, ErrUserDisabled
	}
	s.aud.Record(ctx, &audit.Event{
		EntityType: "user",
		EntityID:   u.ID,
		Action:     "wechat_login",
		ActorID:    u.ID,
		Reason:     openid,
	})
	return s.buildSession(ctx, u)
}

func (s *service) GetUser(ctx context.Context, userID int64) (*User, []string, error) {
	u, err := s.repo.GetByID(ctx, userID)
	if err != nil {
		return nil, nil, ErrUserNotFound
	}
	roles, err := s.repo.GetRoles(ctx, userID)
	if err != nil {
		return nil, nil, err
	}
	return u, roles, nil
}

func (s *service) HashPassword(password string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(b), err
}

func (s *service) buildSession(ctx context.Context, u *User) (*Session, error) {
	roles, err := s.repo.GetRoles(ctx, u.ID)
	if err != nil {
		return nil, err
	}
	token, err := s.tm.Sign(u.ID, roles)
	if err != nil {
		return nil, err
	}
	return &Session{Token: token, User: *u, Roles: roles}, nil
}
