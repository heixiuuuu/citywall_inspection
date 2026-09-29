// Package config 负责加载服务配置。
//
// 配置来源优先级（高 → 低）：
//  1. 环境变量（如 CITY_WALL_DB_DSN 可整体覆盖数据库连接）
//  2. 配置文件（默认 conf/server.json，可用 -config 或 CITY_WALL_CONFIG 指定）
//
// 字段命名与 conf/server.json 保持一致（snake_case）。
package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"
)

// Config 是 server.json 的整体结构。
type Config struct {
	App        AppConfig        `json:"app"`
	Server     ServerConfig     `json:"server"`
	Database   DatabaseConfig   `json:"database"`
	Storage    StorageConfig    `json:"storage"`
	Recognizer RecognizerConfig `json:"recognizer"`
	Auth       AuthConfig       `json:"auth"`
	Upload     UploadConfig     `json:"upload"`
	Report     ReportConfig     `json:"report"`
	Log        LogConfig        `json:"log"`
	CORS       CORSConfig       `json:"cors"`
}

// AppConfig 应用基本信息。
type AppConfig struct {
	Name string `json:"name"`
	Env  string `json:"env"` // dev / prod
}

// ServerConfig HTTP 服务参数。
type ServerConfig struct {
	Listen             string `json:"listen"`
	ReadTimeoutS       int    `json:"read_timeout_s"`
	WriteTimeoutS      int    `json:"write_timeout_s"`
	GracefulShutdownS  int    `json:"graceful_shutdown_s"`
}

// DatabaseConfig MySQL 连接参数。
type DatabaseConfig struct {
	Driver             string `json:"driver"`
	Host               string `json:"host"`
	Port               int    `json:"port"`
	User               string `json:"user"`
	Password           string `json:"password"`
	DBName             string `json:"dbname"`
	Charset            string `json:"charset"`
	Required           bool   `json:"required"` // true 时连接失败必须退出，false 时降级启动（dev 便利）
	MaxOpenConns       int    `json:"max_open_conns"`
	MaxIdleConns       int    `json:"max_idle_conns"`
	ConnMaxLifetimeMin int    `json:"conn_max_lifetime_min"`
	LogLevel           string `json:"log_level"`
	DSNOverride        string `json:"-"` // 环境变量 CITY_WALL_DB_DSN 覆盖
}

// StorageConfig 受控图片与报告目录。
type StorageConfig struct {
	ImageDir          string   `json:"image_dir"`
	ReportDir         string   `json:"report_dir"`
	TempDir           string   `json:"temp_dir"`
	MaxImageBytes     int64    `json:"max_image_bytes"`
	AllowedMediaTypes []string `json:"allowed_media_types"`
}

// RecognizerConfig 内部识别服务（Python/FastAPI）调用参数。
type RecognizerConfig struct {
	BaseURL       string `json:"base_url"`
	InternalToken string `json:"internal_token"`
	TimeoutS      int    `json:"timeout_s"`
	MaxRetries    int    `json:"max_retries"`
}

// AuthConfig 身份与 JWT 参数。
type AuthConfig struct {
	JWTSecret       string `json:"jwt_secret"`
	TokenTTLHours   int    `json:"token_ttl_hours"`
	WechatAppID     string `json:"wechat_app_id"`
	WechatAppSecret string `json:"wechat_app_secret"`
}

// UploadConfig 上传约束与幂等/跟踪头。
type UploadConfig struct {
	MaxBodyBytes        int64  `json:"max_body_bytes"`
	IdempotencyKeyHeader string `json:"idempotency_key_header"`
	RequestIDHeader     string `json:"request_id_header"`
}

// ReportConfig PDF 报告生成参数。
type ReportConfig struct {
	GenerationTimeoutS int `json:"generation_timeout_s"`
}

// LogConfig 日志参数。
type LogConfig struct {
	Level  string `json:"level"`
	Format string `json:"format"`
	Output string `json:"output"`
}

// CORSConfig 允许跨域来源（仅 PC 管理端 dev 联调需要）。
type CORSConfig struct {
	AllowedOrigins []string `json:"allowed_origins"`
}

// Load 读取配置文件并应用环境变量覆盖。
func Load(path string) (*Config, error) {
	// 环境变量指定配置文件时优先。
	if p := os.Getenv("CITY_WALL_CONFIG"); p != "" {
		path = p
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config file %q: %w", path, err)
	}

	cfg := &Config{}
	if err := json.Unmarshal(raw, cfg); err != nil {
		return nil, fmt.Errorf("parse config file %q: %w", path, err)
	}

	if err := cfg.applyEnvOverrides(); err != nil {
		return nil, err
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

// applyEnvOverrides 允许用环境变量覆盖关键敏感配置，避免密钥写入版本库。
func (c *Config) applyEnvOverrides() error {
	if v := os.Getenv("CITY_WALL_DB_DSN"); v != "" {
		c.Database.DSNOverride = v
	}
	if v := os.Getenv("CITY_WALL_JWT_SECRET"); v != "" {
		c.Auth.JWTSecret = v
	}
	if v := os.Getenv("CITY_WALL_RECOGNIZER_TOKEN"); v != "" {
		c.Recognizer.InternalToken = v
	}
	return nil
}

// Validate 检查关键字段。
func (c *Config) Validate() error {
	if c.Server.Listen == "" {
		return errors.New("config: server.listen is required")
	}
	if c.Recognizer.BaseURL == "" {
		return errors.New("config: recognizer.base_url is required")
	}
	if c.Auth.JWTSecret == "" {
		return errors.New("config: auth.jwt_secret is required")
	}
	if c.Storage.ImageDir == "" || c.Storage.ReportDir == "" {
		return errors.New("config: storage.image_dir and storage.report_dir are required")
	}
	return nil
}

// DSN 返回 MySQL 连接串；存在环境变量整体覆盖时优先。
func (c *DatabaseConfig) DSN() string {
	if c.DSNOverride != "" {
		return c.DSNOverride
	}
	return fmt.Sprintf(
		"%s:%s@tcp(%s:%d)/%s?charset=%s&parseTime=True&loc=UTC",
		c.User, c.Password, c.Host, c.Port, c.DBName, c.Charset,
	)
}

// TokenTTL 返回 JWT 有效期。
func (c *AuthConfig) TokenTTL() time.Duration {
	return time.Duration(c.TokenTTLHours) * time.Hour
}

// Timeout 返回识别调用超时。
func (c *RecognizerConfig) Timeout() time.Duration {
	return time.Duration(c.TimeoutS) * time.Second
}
