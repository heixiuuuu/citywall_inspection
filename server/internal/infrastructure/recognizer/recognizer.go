// Package recognizer 是 Go 业务服务到 Python 识别服务的内部 HTTP 客户端。
//
// 契约见 server/api/openapi.yaml 与识别服务 success/failure 响应示例：
// Go 只发送自己已校验并持久化的原图，不转发浏览器 URL，也不透传 Python 异常堆栈。
package recognizer

import (
	"context"
	"errors"
	"time"

	"citywall/server/internal/config"
)

var (
	// ErrNotImplemented 骨架阶段的占位错误，标识接口待接入。
	ErrNotImplemented = errors.New("recognizer: client not implemented yet")
	// ErrTimeout 内部调用超时（由 Go 标记，不删除原图）。
	ErrTimeout = errors.New("recognizer: timeout")
)

// Detection 对应识别服务成功响应中的单个检测候选。
// bbox 为按 EXIF 方向统一后的原图像素坐标，左上角原点。
type Detection struct {
	DetectionID string  `json:"detection_id"`
	ClassCode   string  `json:"class_code"`
	Confidence  float64 `json:"confidence"`
	BBox        BBox    `json:"bbox"`
}

// BBox 检测框坐标。
type BBox struct {
	X1 int `json:"x1"`
	Y1 int `json:"y1"`
	X2 int `json:"x2"`
	Y2 int `json:"y2"`
}

// Result 识别成功响应。
type Result struct {
	SchemaVersion string      `json:"schema_version"`
	AttemptID     string      `json:"attempt_id"`
	ModelVersion  string      `json:"model_version"`
	ImageWidth    int         `json:"image_width"`
	ImageHeight   int         `json:"image_height"`
	LatencyMs     int         `json:"latency_ms"`
	Detections    []Detection `json:"detections"`
}

// Recognizer 抽象内部识别客户端，便于测试替身。
type Recognizer interface {
	// Recognize 同步识别单张图片。失败时返回错误码（INVALID_IMAGE / IMAGE_TOO_LARGE /
	// MODEL_UNAVAILABLE / INFERENCE_FAILED / RECOGNITION_TIMEOUT）。
	Recognize(ctx context.Context, attemptID, imageID, requestID string, image []byte) (*Result, error)
}

// HTTPRecognizer 走 HTTP 调用 Python 识别服务的实现。
type HTTPRecognizer struct {
	cfg config.RecognizerConfig
}

// New 创建 HTTP 识别客户端。
func New(cfg config.RecognizerConfig) *HTTPRecognizer {
	return &HTTPRecognizer{cfg: cfg}
}

// Recognize 调用 POST {base}/internal/v1/recognitions（multipart/form-data）。
// TODO(init): 实现 multipart 请求、超时控制与失败错误码映射，见 03 文档第 7 节。
func (r *HTTPRecognizer) Recognize(ctx context.Context, attemptID, imageID, requestID string, image []byte) (*Result, error) {
	_ = ctx
	_ = attemptID
	_ = imageID
	_ = requestID
	_ = image
	_ = r.cfg.Timeout()
	return nil, ErrNotImplemented
}

// Ping 健康检查用：确认识别服务可达（内部 /healthz）。
// TODO(init): 实现后用于 /readyz 聚合。
func (r *HTTPRecognizer) Ping(ctx context.Context) error {
	_ = ctx
	_ = time.Now
	return nil
}
