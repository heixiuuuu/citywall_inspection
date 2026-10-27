package api

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"citywall/server/internal/middleware"
	"citywall/server/internal/module/issue"
)

// IssueHandler 问题台账、处置与受理接口。
type IssueHandler struct {
	service issue.Service
}

// NewIssueHandler 构造问题 Handler。
func NewIssueHandler(svc issue.Service) *IssueHandler {
	return &IssueHandler{service: svc}
}

func (h *IssueHandler) ListAssignees(c *gin.Context) {
	actor, ok := middleware.GetActor(c)
	if !ok {
		writeError(c, http.StatusUnauthorized, "UNAUTHORIZED", "not authenticated")
		return
	}
	users, err := h.service.ListAssignees(c.Request.Context(), actor)
	if err != nil {
		writeErr(c, err)
		return
	}
	writeData(c, http.StatusOK, gin.H{"items": users})
}

// List 问题台账（管理员/审核员全部，巡检员仅被指派；支持 status/page/page_size）。
func (h *IssueHandler) List(c *gin.Context) {
	actor, ok := middleware.GetActor(c)
	if !ok {
		writeError(c, http.StatusUnauthorized, "UNAUTHORIZED", "not authenticated")
		return
	}
	in := issue.ListInput{
		Status:   c.Query("status"),
		Page:     queryInt(c, "page", 1),
		PageSize: queryInt(c, "page_size", 20),
	}
	res, err := h.service.List(context.Background(), actor, in)
	if err != nil {
		writeErr(c, err)
		return
	}
	writeData(c, http.StatusOK, gin.H{"items": res.Items, "total": res.Total, "page": in.Page, "page_size": in.PageSize})
}

// Get 问题详情（问题、关联记录、处置动作）。
func (h *IssueHandler) Get(c *gin.Context) {
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
	iss, briefs, actions, err := h.service.Get(context.Background(), actor, id)
	if err != nil {
		writeErr(c, err)
		return
	}
	writeData(c, http.StatusOK, gin.H{"issue": iss, "observations": briefs, "actions": actions})
}

type assignRequest struct {
	Revision   int64  `json:"revision" binding:"required"`
	AssigneeID int64  `json:"assignee_id" binding:"required"`
	Reason     string `json:"reason"`
}

// Assign 指派/改派责任人（管理员）。
func (h *IssueHandler) Assign(c *gin.Context) {
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
	var req assignRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeError(c, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return
	}
	iss, err := h.service.Assign(context.Background(), actor, id, req.Revision, req.AssigneeID, req.Reason)
	if err != nil {
		writeErr(c, err)
		return
	}
	writeData(c, http.StatusOK, gin.H{"issue": iss})
}

type actionRequest struct {
	Revision          int64   `json:"revision" binding:"required"`
	ActionType        string  `json:"action_type" binding:"required,oneof=ADVICE PROGRESS OBSERVE REQUEST_VERIFY"`
	Content           string  `json:"content" binding:"required"`
	PlannedFollowUpAt *string `json:"planned_follow_up_at"`
}

// AddAction 追加处置动作（建议/进展/观察/申请核验）。
func (h *IssueHandler) AddAction(c *gin.Context) {
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
	var req actionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeError(c, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return
	}
	in := issue.ActionInput{
		ActionType: req.ActionType,
		Content:    req.Content,
	}
	if req.PlannedFollowUpAt != nil {
		if t, err := time.Parse(time.RFC3339, *req.PlannedFollowUpAt); err == nil {
			in.PlannedFollowUpAt = &t
		}
	}
	iss, err := h.service.AddAction(context.Background(), actor, id, req.Revision, in)
	if err != nil {
		writeErr(c, err)
		return
	}
	writeData(c, http.StatusOK, gin.H{"issue": iss})
}

type requestVerifyRequest struct {
	Revision int64  `json:"revision" binding:"required"`
	Content  string `json:"content" binding:"required"`
}

// RequestVerification 责任人申请核验。
func (h *IssueHandler) RequestVerification(c *gin.Context) {
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
	var req requestVerifyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeError(c, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return
	}
	iss, err := h.service.RequestVerification(context.Background(), actor, id, req.Revision, req.Content)
	if err != nil {
		writeErr(c, err)
		return
	}
	writeData(c, http.StatusOK, gin.H{"issue": iss})
}

type verifyRequest struct {
	Revision int64  `json:"revision" binding:"required"`
	Decision string `json:"decision" binding:"required,oneof=PASS RETURN"`
	Reason   string `json:"reason"`
}

// Verify 审核员核验：PASS 关闭，RETURN 退回继续处理。
func (h *IssueHandler) Verify(c *gin.Context) {
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
	var req verifyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeError(c, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return
	}
	iss, err := h.service.Verify(context.Background(), actor, id, req.Revision, req.Decision, req.Reason)
	if err != nil {
		writeErr(c, err)
		return
	}
	writeData(c, http.StatusOK, gin.H{"issue": iss})
}

type reopenRequest struct {
	Revision int64  `json:"revision" binding:"required"`
	Reason   string `json:"reason" binding:"required"`
}

// Reopen 重新打开已关闭问题（审核员，保留先前关闭记录）。
func (h *IssueHandler) Reopen(c *gin.Context) {
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
	var req reopenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeError(c, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return
	}
	iss, err := h.service.Reopen(context.Background(), actor, id, req.Revision, req.Reason)
	if err != nil {
		writeErr(c, err)
		return
	}
	writeData(c, http.StatusOK, gin.H{"issue": iss})
}

// AddActionImage 上传问题处置动作附件（multipart：image）。
func (h *IssueHandler) AddActionImage(c *gin.Context) {
	actor, ok := middleware.GetActor(c)
	if !ok {
		writeError(c, http.StatusUnauthorized, "UNAUTHORIZED", "not authenticated")
		return
	}
	issueID, err := pathID(c)
	if err != nil {
		writeError(c, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return
	}
	actionID, err := strconv.ParseInt(c.Param("action_id"), 10, 64)
	if err != nil {
		writeError(c, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return
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
	buf := make([]byte, fileHeader.Size)
	if _, err := f.Read(buf); err != nil {
		writeErr(c, err)
		return
	}
	img, err := h.service.AddActionImage(context.Background(), actor, issueID, actionID, buf, detectMediaType(fileHeader.Header.Get("Content-Type")))
	if err != nil {
		writeErr(c, err)
		return
	}
	writeData(c, http.StatusCreated, gin.H{"image": img})
}

// AdmitFromObservation 受理已通过审核的记录：新建/关联问题或记录不立项原因。
func (h *IssueHandler) AdmitFromObservation(c *gin.Context) {
	actor, ok := middleware.GetActor(c)
	if !ok {
		writeError(c, http.StatusUnauthorized, "UNAUTHORIZED", "not authenticated")
		return
	}
	obsID, err := pathID(c)
	if err != nil {
		writeError(c, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return
	}
	var req issue.AdmitInput
	if err := c.ShouldBindJSON(&req); err != nil {
		writeError(c, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return
	}
	res, err := h.service.AdmitFromObservation(context.Background(), actor, obsID, req)
	if err != nil {
		writeErr(c, err)
		return
	}
	writeData(c, http.StatusOK, gin.H{"admit": res})
}
