package api

import (
	"context"
	"errors"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"

	"citywall/server/internal/infrastructure/imageutil"
	"citywall/server/internal/middleware"
	"citywall/server/internal/module/observation"
)

// ObservationHandler 现场记录相关接口。
type ObservationHandler struct {
	service        observation.Service
	idemHeaderName string
}

// NewObservationHandler 构造现场记录 Handler。
// idempotencyKeyHeader 为上传幂等键请求头名（可为空：不启用幂等）。
func NewObservationHandler(svc observation.Service, idempotencyKeyHeader string) *ObservationHandler {
	return &ObservationHandler{service: svc, idemHeaderName: idempotencyKeyHeader}
}

type createObservationRequest struct {
	Source       string `json:"source" binding:"required,oneof=TASK VOLUNTARY"`
	TaskPointID  *int64 `json:"task_point_id"`
	LocationText string `json:"location_text" binding:"required"`
}

// Create 创建现场记录草稿（小程序）。source=TASK 必须带 task_point_id。
func (h *ObservationHandler) Create(c *gin.Context) {
	actor, ok := middleware.GetActor(c)
	if !ok {
		writeError(c, http.StatusUnauthorized, "UNAUTHORIZED", "not authenticated")
		return
	}
	var req createObservationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeError(c, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return
	}
	o, err := h.service.Create(context.Background(), actor, req.Source, req.TaskPointID, req.LocationText)
	if err != nil {
		writeErr(c, err)
		return
	}
	writeData(c, http.StatusCreated, gin.H{"observation": o})
}

// Update 更新草稿位置/现场判断（DRAFT/RETURNED）。
func (h *ObservationHandler) Update(c *gin.Context) {
	actor, ok := middleware.GetActor(c)
	if !ok {
		writeError(c, http.StatusUnauthorized, "UNAUTHORIZED", "not authenticated")
		return
	}
	id, err := pathID(c)
	if err != nil {
		writeError(c, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return
	}
	var body struct {
		Revision int64 `json:"revision" binding:"required"`
		observation.PatchInput
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		writeError(c, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return
	}
	o, err := h.service.Update(context.Background(), actor, id, body.Revision, body.PatchInput)
	if err != nil {
		writeErr(c, err)
		return
	}
	writeData(c, http.StatusOK, gin.H{"observation": o})
}

// AddImage 上传图片（multipart/form-data：image + image_role）。
func (h *ObservationHandler) AddImage(c *gin.Context) {
	actor, ok := middleware.GetActor(c)
	if !ok {
		writeError(c, http.StatusUnauthorized, "UNAUTHORIZED", "not authenticated")
		return
	}
	id, err := pathID(c)
	if err != nil {
		writeError(c, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return
	}
	role := c.PostForm("image_role")
	if role == "" {
		role = observation.ImageRoleOverview
	}
	fileHeader, err := c.FormFile("image")
	if err != nil {
		writeError(c, http.StatusBadRequest, "VALIDATION_ERROR", "image file required")
		return
	}
	f, err := fileHeader.Open()
	if err != nil {
		writeErr(c, err)
		return
	}
	defer f.Close()

	buf, err := io.ReadAll(f)
	if err != nil {
		writeErr(c, err)
		return
	}
	mediaType := detectMediaType(fileHeader.Header.Get("Content-Type"))
	var idemKey string
	if h.idemHeaderName != "" {
		idemKey = c.GetHeader(h.idemHeaderName)
	}
	img, err := h.service.AddImage(context.Background(), actor, id, mediaType, buf, role, idemKey)
	if err != nil {
		if errors.Is(err, observation.ErrIdempotentReplay) {
			// 同幂等键重放：返回 200，无副作用。
			writeData(c, http.StatusOK, gin.H{"idempotent": true, "replayed": true})
			return
		}
		writeErr(c, err)
		return
	}
	writeData(c, http.StatusCreated, gin.H{"image": img})
}

// GetImageContent 受控图片流（鉴权后返回）。
func (h *ObservationHandler) GetImageContent(c *gin.Context) {
	h.getImage(c, false)
}

func (h *ObservationHandler) GetImagePreview(c *gin.Context) {
	h.getImage(c, true)
}

func (h *ObservationHandler) getImage(c *gin.Context, preview bool) {
	actor, ok := middleware.GetActor(c)
	if !ok {
		writeError(c, http.StatusUnauthorized, "UNAUTHORIZED", "not authenticated")
		return
	}
	id, err := pathID(c)
	if err != nil {
		writeError(c, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return
	}
	img, rc, err := h.service.GetImageContent(context.Background(), actor, id)
	if err != nil {
		writeErr(c, err)
		return
	}
	defer rc.Close()
	if preview {
		data, err := io.ReadAll(rc)
		if err != nil {
			writeErr(c, err)
			return
		}
		preview, err := imageutil.Preview(data, img.MediaType)
		if err != nil {
			writeErr(c, err)
			return
		}
		c.Data(http.StatusOK, preview.MediaType, preview.Data)
		return
	}
	c.DataFromReader(http.StatusOK, img.ByteSize, img.MediaType, rc, nil)
}

type recognizeRequest struct {
	RequestID string `json:"request_id"`
}

// Recognize 触发一次识别尝试（重试新建 attempt，不覆盖旧结果）。
func (h *ObservationHandler) Recognize(c *gin.Context) {
	actor, ok := middleware.GetActor(c)
	if !ok {
		writeError(c, http.StatusUnauthorized, "UNAUTHORIZED", "not authenticated")
		return
	}
	id, err := pathID(c)
	if err != nil {
		writeError(c, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return
	}
	var req recognizeRequest
	_ = c.ShouldBindJSON(&req)
	attempt, detections, err := h.service.Recognize(c.Request.Context(), actor, id, middleware.GetRequestID(c))
	if err != nil {
		if attempt != nil {
			writeData(c, http.StatusOK, gin.H{"attempt": attempt, "detections": []observation.Detection{}})
			return
		}
		writeErr(c, err)
		return
	}
	writeData(c, http.StatusOK, gin.H{"attempt": attempt, "detections": detections})
}

// SaveConclusion 保存人工结论（整体替换当前草稿结论版本）。
func (h *ObservationHandler) SaveConclusion(c *gin.Context) {
	actor, ok := middleware.GetActor(c)
	if !ok {
		writeError(c, http.StatusUnauthorized, "UNAUTHORIZED", "not authenticated")
		return
	}
	id, err := pathID(c)
	if err != nil {
		writeError(c, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return
	}
	var body struct {
		Revision int64 `json:"revision" binding:"required"`
		observation.ConclusionInput
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		writeError(c, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return
	}
	rev, err := h.service.SaveConclusion(context.Background(), actor, id, body.Revision, body.ConclusionInput)
	if err != nil {
		writeErr(c, err)
		return
	}
	writeData(c, http.StatusCreated, gin.H{"conclusion": rev})
}

// Submit 提交记录（DRAFT/RETURNED → SUBMITTED）。
func (h *ObservationHandler) Submit(c *gin.Context) {
	actor, ok := middleware.GetActor(c)
	if !ok {
		writeError(c, http.StatusUnauthorized, "UNAUTHORIZED", "not authenticated")
		return
	}
	id, revision, ok := idAndRevision(c)
	if !ok {
		return
	}
	o, err := h.service.Submit(context.Background(), actor, id, revision)
	if err != nil {
		writeErr(c, err)
		return
	}
	writeData(c, http.StatusOK, gin.H{"observation": o})
}

// Review 审核记录（通过/退回/驳回主动上报）。
func (h *ObservationHandler) Review(c *gin.Context) {
	actor, ok := middleware.GetActor(c)
	if !ok {
		writeError(c, http.StatusUnauthorized, "UNAUTHORIZED", "not authenticated")
		return
	}
	id, err := pathID(c)
	if err != nil {
		writeError(c, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return
	}
	var body struct {
		Revision int64 `json:"revision" binding:"required"`
		observation.ReviewInput
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		writeError(c, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return
	}
	o, err := h.service.Review(context.Background(), actor, id, body.Revision, body.ReviewInput)
	if err != nil {
		writeErr(c, err)
		return
	}
	writeData(c, http.StatusOK, gin.H{"observation": o})
}

// ListPendingReview 待审核记录列表（审核员）。
func (h *ObservationHandler) ListPendingReview(c *gin.Context) {
	actor, ok := middleware.GetActor(c)
	if !ok {
		writeError(c, http.StatusUnauthorized, "UNAUTHORIZED", "not authenticated")
		return
	}
	items, err := h.service.ListPendingReview(context.Background(), actor)
	if err != nil {
		writeErr(c, err)
		return
	}
	writeData(c, http.StatusOK, gin.H{"items": items})
}

// ListImages 记录图片列表（负责人本人或管理员/审核员）。
func (h *ObservationHandler) ListImages(c *gin.Context) {
	actor, ok := middleware.GetActor(c)
	if !ok {
		writeError(c, http.StatusUnauthorized, "UNAUTHORIZED", "not authenticated")
		return
	}
	id, err := pathID(c)
	if err != nil {
		writeError(c, http.StatusBadRequest, "VALIDATION_ERROR", "invalid id")
		return
	}
	imgs, err := h.service.ListImages(context.Background(), actor, id)
	if err != nil {
		writeErr(c, err)
		return
	}
	writeData(c, http.StatusOK, gin.H{"images": imgs})
}

// List 记录列表（管理员/审核员全部，巡检员自己的；支持 status/page/page_size）。
func (h *ObservationHandler) List(c *gin.Context) {
	actor, ok := middleware.GetActor(c)
	if !ok {
		writeError(c, http.StatusUnauthorized, "UNAUTHORIZED", "not authenticated")
		return
	}
	in := observation.ListInput{
		PendingAdmission: c.Query("pending_admission") == "1",
		TaskPointID:      int64(queryInt(c, "task_point_id", 0)),
		Status:           c.Query("status"),
		TaskID:           int64(queryInt(c, "task_id", 0)),
		Source:           c.Query("source"),
		PointID:          int64(queryInt(c, "point_id", 0)),
		DateFrom:         c.Query("date_from"),
		DateTo:           c.Query("date_to"),
		Page:             queryInt(c, "page", 1),
		PageSize:         queryInt(c, "page_size", 20),
	}
	res, err := h.service.List(context.Background(), actor, in)
	if err != nil {
		writeErr(c, err)
		return
	}
	writeData(c, http.StatusOK, gin.H{"items": res.Items, "total": res.Total, "page": in.Page, "page_size": in.PageSize})
}

// detectMediaType 依据 Content-Type 归一化图片类型。
func detectMediaType(contentType string) string {
	switch contentType {
	case "image/png":
		return "image/png"
	case "image/webp":
		return "image/webp"
	default:
		return "image/jpeg"
	}
}
