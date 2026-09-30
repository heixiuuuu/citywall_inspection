// Package recognizer 是 Go 业务服务到 Python 识别服务的内部 HTTP 客户端。
//
// 契约见 server/api/openapi.yaml 与识别服务 success/failure 响应示例：
// Go 只发送自己已校验并持久化的原图，不转发浏览器 URL，也不透传 Python 异常堆栈。
package recognizer

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"

	"citywall/server/internal/config"
)

var (
	// ErrTimeout 内部调用超时（由 Go 标记，不删除原图）。
	ErrTimeout = errors.New("recognizer: timeout")
	// ErrBadResponse 响应结构不符合契约。
	ErrBadResponse = errors.New("recognizer: malformed response")
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
	cfg    config.RecognizerConfig
	client *http.Client
}

// New 创建 HTTP 识别客户端。
func New(cfg config.RecognizerConfig) *HTTPRecognizer {
	return &HTTPRecognizer{
		cfg: cfg,
		client: &http.Client{
			Timeout: cfg.Timeout(),
		},
	}
}

// Recognize 调用 POST {base}/internal/v1/recognitions（multipart/form-data）。
// 超时返回 ErrTimeout；非 2xx 返回包含错误码的错误。
func (r *HTTPRecognizer) Recognize(ctx context.Context, attemptID, imageID, requestID string, image []byte) (*Result, error) {
	body := &bytes.Buffer{}
	mw := multipart.NewWriter(body)
	if err := mw.WriteField("attempt_id", attemptID); err != nil {
		return nil, err
	}
	if err := mw.WriteField("image_id", imageID); err != nil {
		return nil, err
	}
	if err := mw.WriteField("request_id", requestID); err != nil {
		return nil, err
	}
	fw, err := mw.CreateFormFile("image", "upload.jpg")
	if err != nil {
		return nil, err
	}
	if _, err := fw.Write(image); err != nil {
		return nil, err
	}
	if err := mw.Close(); err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		r.cfg.BaseURL+"/internal/v1/recognitions", body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", mw.FormDataContentType())
	if r.cfg.InternalToken != "" {
		req.Header.Set("X-Internal-Token", r.cfg.InternalToken)
	}

	resp, err := r.client.Do(req)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) || isTimeout(err) {
			return nil, ErrTimeout
		}
		return nil, fmt.Errorf("recognizer: request failed: %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		var errBody struct {
			Error struct {
				Code    string `json:"code"`
				Message string `json:"message"`
			} `json:"error"`
		}
		if err := json.Unmarshal(data, &errBody); err != nil {
			return nil, fmt.Errorf("recognizer: http %d: %w", resp.StatusCode, ErrBadResponse)
		}
		return nil, fmt.Errorf("recognizer: %s: %s", errBody.Error.Code, errBody.Error.Message)
	}

	var result Result
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("recognizer: %w", ErrBadResponse)
	}
	return &result, nil
}

func isTimeout(err error) bool {
	var ne interface{ Timeout() bool }
	if errors.As(err, &ne) {
		return ne.Timeout()
	}
	return false
}
