// Package storage 管理受控图片与报告目录。
// 目录不对外静态公开；图片访问一律经业务层鉴权后由接口输出。
package storage

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"

	"citywall/server/internal/config"
)

var (
	// ErrUnsupportedMediaType 图片格式不在允许列表内。
	ErrUnsupportedMediaType = errors.New("storage: unsupported media type")
	// ErrImageTooLarge 图片超过大小上限。
	ErrImageTooLarge = errors.New("storage: image too large")
)

// Storage 对外提供文件落盘与路径管理能力。
type Storage interface {
	// SaveImage 将图片字节写入受控目录，返回服务端生成的相对 key。
	SaveImage(data []byte, mediaType string) (string, error)
	// Open 读取受控目录下的文件（调用方负责鉴权）。
	Open(relKey string) (*os.File, error)
}

// LocalStorage 基于本地目录的实现。
type LocalStorage struct {
	cfg config.StorageConfig
}

// New 创建本地存储并确保目录存在。
func New(cfg config.StorageConfig) (*LocalStorage, error) {
	s := &LocalStorage{cfg: cfg}
	for _, dir := range []string{cfg.ImageDir, cfg.ReportDir, cfg.TempDir} {
		if dir == "" {
			continue
		}
		if err := os.MkdirAll(dir, 0o750); err != nil {
			return nil, err
		}
	}
	return s, nil
}

// SaveImage 落盘并返回相对 key（如 3f2a...c1.jpg）。
// 初始化阶段提供最小实现；后续按 04 文档补 EXIF 方向、孤儿文件清理等。
func (s *LocalStorage) SaveImage(data []byte, mediaType string) (string, error) {
	if !s.allowed(mediaType) {
		return "", ErrUnsupportedMediaType
	}
	if int64(len(data)) > s.cfg.MaxImageBytes {
		return "", ErrImageTooLarge
	}

	rel := randKey(mediaType)
	if err := os.WriteFile(filepath.Join(s.cfg.ImageDir, rel), data, 0o640); err != nil {
		return "", err
	}
	return rel, nil
}

// Open 打开受控目录下的文件。路径穿越防护：仅允许 base 内的相对路径。
func (s *LocalStorage) Open(relKey string) (*os.File, error) {
	if filepath.IsAbs(relKey) {
		return nil, errors.New("storage: absolute path not allowed")
	}
	full := filepath.Join(s.cfg.ImageDir, relKey)
	abs := filepath.Clean(full)
	base := filepath.Clean(s.cfg.ImageDir)
	if abs != base && !hasPrefix(abs, base+string(os.PathSeparator)) {
		return nil, errors.New("storage: path escapes image dir")
	}
	return os.Open(abs)
}

func (s *LocalStorage) allowed(mediaType string) bool {
	for _, t := range s.cfg.AllowedMediaTypes {
		if t == mediaType {
			return true
		}
	}
	return false
}

func hasPrefix(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}

// randKey 生成随机存储文件名（crypto/rand 16 字节，不可枚举）。
func randKey(mediaType string) string {
	ext := map[string]string{
		"image/jpeg": "jpg",
		"image/png":  "png",
		"image/webp": "webp",
	}[mediaType]
	if ext == "" {
		ext = "bin"
	}
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "unknown-" + ext
	}
	return hex.EncodeToString(b) + "." + ext
}
