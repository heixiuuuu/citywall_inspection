package issue

import (
	"bytes"
	"context"
	"errors"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"time"

	"citywall/server/internal/middleware"
	"citywall/server/internal/module/audit"
	"citywall/server/internal/module/observation"
)

// 领域错误（Handler 映射 HTTP 状态）。
var (
	ErrNotFound         = errors.New("issue: not found")
	ErrForbidden        = errors.New("issue: forbidden")
	ErrInvalidState     = errors.New("issue: invalid state for operation")
	ErrStatusConflict   = errors.New("issue: status or revision conflict")
	ErrSelfVerify       = errors.New("issue: cannot verify own submitted verification")
	ErrReasonRequired   = errors.New("issue: reason required")
	ErrAssigneeSelf     = errors.New("issue: cannot assign to self as verifier")
	ErrAdmitEmpty       = errors.New("issue: admit requires new issues, linked issues or no-issue reason")
	ErrAdmitNotAccepted = errors.New("issue: observation not accepted or already admitted")
	ErrActionImageMiss  = errors.New("issue: action not found")
)

// ObservationAccess 由 observation 模块实现：受理时校验记录状态并取审核决定。
type ObservationAccess interface {
	// AcceptedAdmissionInfo 返回已通过审核记录的受理信息（仅 ACCEPTED 记录可受理）。
	AcceptedAdmissionInfo(ctx context.Context, obsID int64) (*observation.AcceptedInfo, error)
}

// NewIssueInput 受理时新建问题。
type NewIssueInput struct {
	Title          string `json:"title" binding:"required"`
	SegmentID      *int64 `json:"segment_id"`
	PointID        *int64 `json:"point_id"`
	LocationText   string `json:"location_text"`
	Description    string `json:"description"`
	PriorityCode   string `json:"priority_code" binding:"required,oneof=WATCH PROCESS URGENT"`
	PriorityReason string `json:"priority_reason"`
}

// AdmitInput 受理输入（新建 / 关联 / 不立项三选一或组合）。
type AdmitInput struct {
	NewIssues     []NewIssueInput `json:"new_issues"`
	LinkIssueIDs  []int64         `json:"link_issue_ids"`
	NoIssueReason string          `json:"no_issue_reason"`
}

// AdmitResult 受理结果。
type AdmitResult struct {
	CreatedIssueIDs []int64 `json:"created_issue_ids"`
	LinkedIssueIDs  []int64 `json:"linked_issue_ids"`
	NoIssueReason   string  `json:"no_issue_reason"`
	AlreadyAdmitted bool    `json:"already_admitted"`
}

// ActionInput 追加处置动作（建议/进展/观察/申请核验）。
type ActionInput struct {
	ActionType        string     `json:"action_type" binding:"required,oneof=ADVICE PROGRESS OBSERVE REQUEST_VERIFY"`
	Content           string     `json:"content" binding:"required"`
	PlannedFollowUpAt *time.Time `json:"planned_follow_up_at"`
}

// ListInput 台账查询。
type ListInput struct {
	Status   string
	Page     int
	PageSize int
}

// ListResult 分页台账结果。
type ListResult struct {
	Items []Issue
	Total int64
}

// ObservationBrief 关联记录摘要（供详情展示来源）。
type ObservationBrief struct {
	ObservationID int64     `json:"observation_id"`
	Source        string    `json:"source"`
	Outcome       string    `json:"outcome"`
	ReporterID    int64     `json:"reporter_id"`
	LocationText  string    `json:"location_text"`
	Status        string    `json:"status"`
	LinkReason    string    `json:"link_reason"`
	LinkedAt      time.Time `json:"linked_at"`
}

// Service 问题业务规则。
type Service interface {
	List(ctx context.Context, actor *middleware.Actor, in ListInput) (*ListResult, error)
	Get(ctx context.Context, actor *middleware.Actor, id int64) (*Issue, []ObservationBrief, []IssueAction, error)
	Assign(ctx context.Context, actor *middleware.Actor, id, revision, assigneeID int64, reason string) (*Issue, error)
	AddAction(ctx context.Context, actor *middleware.Actor, id, revision int64, in ActionInput) (*Issue, error)
	RequestVerification(ctx context.Context, actor *middleware.Actor, id, revision int64, content string) (*Issue, error)
	Verify(ctx context.Context, actor *middleware.Actor, id, revision int64, decision, reason string) (*Issue, error)
	Reopen(ctx context.Context, actor *middleware.Actor, id, revision int64, reason string) (*Issue, error)
	AddActionImage(ctx context.Context, actor *middleware.Actor, issueID, actionID int64, data []byte, mediaType string) (*ActionImage, error)
	AdmitFromObservation(ctx context.Context, actor *middleware.Actor, obsID int64, in AdmitInput) (*AdmitResult, error)
}

// ActionImage 动作附件元数据（由 ImageStore 注册）。
type ActionImage struct {
	ID        int64  `json:"id"`
	MediaType string `json:"media_type"`
	ByteSize  int64  `json:"byte_size"`
	Width     int    `json:"width"`
	Height    int    `json:"height"`
	URL       string `json:"url"`
}

// ImageStore 图片落盘与注册（由 infrastructure/storage + images 表实现注入）。
type ImageStore interface {
	// Save 将图片字节写入受控目录，返回 storage_key 与 sha256。
	Save(data []byte, mediaType string) (key string, sha256 string, err error)
	// Register 登记图片元数据到 images 表，返回 image_id。
	Register(ctx context.Context, key, sha256, mediaType string, byteSize int64, width, height int, uploadedBy int64) (int64, error)
}

type service struct {
	repo      Repository
	obsAccess ObservationAccess
	images    ImageStore
	audit     audit.Recorder
}

// NewService 构造问题服务。
func NewService(repo Repository, obs ObservationAccess, images ImageStore, rec audit.Recorder) Service {
	return &service{repo: repo, obsAccess: obs, images: images, audit: rec}
}

func (s *service) List(ctx context.Context, actor *middleware.Actor, in ListInput) (*ListResult, error) {
	isAdmin := hasAdminOrReviewer(actor)
	page, pageSize := in.Page, in.PageSize
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}
	items, err := s.repo.ListPage(ctx, actor.UserID, isAdmin, in.Status, pageSize, (page-1)*pageSize)
	if err != nil {
		return nil, err
	}
	total, err := s.repo.CountIssues(ctx, actor.UserID, isAdmin, in.Status)
	if err != nil {
		return nil, err
	}
	return &ListResult{Items: items, Total: total}, nil
}

func (s *service) Get(ctx context.Context, actor *middleware.Actor, id int64) (*Issue, []ObservationBrief, []IssueAction, error) {
	i, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, nil, nil, ErrNotFound
	}
	if !hasAdminOrReviewer(actor) && (i.AssigneeID == nil || *i.AssigneeID != actor.UserID) {
		return nil, nil, nil, ErrForbidden
	}
	links, err := s.repo.ListObservations(ctx, id)
	if err != nil {
		return nil, nil, nil, err
	}
	briefs := make([]ObservationBrief, 0, len(links))
	for _, l := range links {
		briefs = append(briefs, ObservationBrief{
			ObservationID: l.ObservationID,
			LinkReason:    l.LinkReason,
			LinkedAt:      l.LinkedAt,
		})
	}
	actions, err := s.repo.ListActions(ctx, id)
	if err != nil {
		return nil, nil, nil, err
	}
	return i, briefs, actions, nil
}

func (s *service) Assign(ctx context.Context, actor *middleware.Actor, id, revision, assigneeID int64, reason string) (*Issue, error) {
	if !hasRole(actor, "ADMIN") {
		return nil, ErrForbidden
	}
	i, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, ErrNotFound
	}
	if i.Status == StatusClosed {
		return nil, ErrInvalidState
	}
	if i.Revision != revision {
		return nil, ErrStatusConflict
	}
	n, err := s.repo.UpdateAssignee(ctx, id, revision, assigneeID)
	if err != nil {
		return nil, err
	}
	if n == 0 {
		return nil, ErrStatusConflict
	}
	// 已分配 → 保持原状态；待分配 → 进入 OPEN。
	action := &IssueAction{IssueID: id, ActorID: actor.UserID, ActionType: ActionAssign, Content: reason}
	if reason == "" {
		action.Content = "指派责任人"
	}
	if err := s.repo.InsertAction(ctx, action); err != nil {
		return nil, err
	}
	newStatus := i.Status
	if i.Status == StatusUnassigned {
		newStatus = StatusOpen
		// UpdateAssignee 已使 revision 递增（fromRevision → fromRevision+1），
		// 状态更新使用更新后的 revision，避免条件不匹配。
		n2, err := s.repo.UpdateStatus(ctx, id, revision+1, newStatus)
		if err != nil {
			return nil, err
		}
		if n2 == 0 {
			return nil, ErrStatusConflict
		}
		i.Revision = revision + 2 // 两次条件更新，revision 递增两次。
	} else {
		i.Revision = revision + 1
	}
	i.AssigneeID = &assigneeID
	i.Status = newStatus
	s.audit.Record(ctx, &audit.Event{
		EntityType: "issue", EntityID: id, Action: "assign", ActorID: actor.UserID,
		OldStatus: statusOf(i, ""), NewStatus: newStatus, Revision: &i.Revision, Reason: reason,
	})
	return i, nil
}

// AddAction 追加建议/进展/观察并驱动状态流转（OPEN→IN_PROGRESS、IN_PROGRESS↔MONITORING）。
func (s *service) AddAction(ctx context.Context, actor *middleware.Actor, id, revision int64, in ActionInput) (*Issue, error) {
	i, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, ErrNotFound
	}
	if !canAct(actor, i) {
		return nil, ErrForbidden
	}
	if i.Revision != revision {
		return nil, ErrStatusConflict
	}
	next := i.Status
	switch in.ActionType {
	case ActionAdvice:
		// 建议不改变状态。
		if i.Status != StatusOpen && i.Status != StatusInProgress && i.Status != StatusMonitoring {
			return nil, ErrInvalidState
		}
	case ActionProgress:
		switch i.Status {
		case StatusOpen, StatusMonitoring:
			next = StatusInProgress
		case StatusInProgress:
		default:
			return nil, ErrInvalidState
		}
	case ActionObserve:
		switch i.Status {
		case StatusOpen, StatusInProgress, StatusMonitoring:
			next = StatusMonitoring
		default:
			return nil, ErrInvalidState
		}
	case ActionRequestVerify:
		return s.RequestVerification(ctx, actor, id, revision, in.Content)
	}
	action := &IssueAction{
		IssueID: id, ActorID: actor.UserID, ActionType: in.ActionType,
		Content: in.Content, PlannedFollowUpAt: in.PlannedFollowUpAt,
	}
	if err := s.repo.InsertAction(ctx, action); err != nil {
		return nil, err
	}
	// 无论状态是否变化，每次动作都递增 revision（条件更新，保持乐观锁一致）。
	n, err := s.repo.UpdateStatus(ctx, id, revision, next)
	if err != nil {
		return nil, err
	}
	if n == 0 {
		return nil, ErrStatusConflict
	}
	i.Status = next
	i.Revision++
	s.audit.Record(ctx, &audit.Event{
		EntityType: "issue", EntityID: id, Action: "action:" + in.ActionType, ActorID: actor.UserID,
		OldStatus: statusOf(i, ""), NewStatus: i.Status, Revision: &i.Revision,
	})
	return i, nil
}

// RequestVerification 责任人提交处理/观察结论，进入待核验。
func (s *service) RequestVerification(ctx context.Context, actor *middleware.Actor, id, revision int64, content string) (*Issue, error) {
	i, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, ErrNotFound
	}
	if !canAct(actor, i) {
		return nil, ErrForbidden
	}
	if i.Status != StatusInProgress && i.Status != StatusMonitoring {
		return nil, ErrInvalidState
	}
	if i.Revision != revision {
		return nil, ErrStatusConflict
	}
	action := &IssueAction{IssueID: id, ActorID: actor.UserID, ActionType: ActionRequestVerify, Content: content}
	if err := s.repo.InsertAction(ctx, action); err != nil {
		return nil, err
	}
	n, err := s.repo.UpdateStatus(ctx, id, revision, StatusPendingVerification)
	if err != nil {
		return nil, err
	}
	if n == 0 {
		return nil, ErrStatusConflict
	}
	i.Status = StatusPendingVerification
	i.Revision++
	s.audit.Record(ctx, &audit.Event{
		EntityType: "issue", EntityID: id, Action: "request_verify", ActorID: actor.UserID,
		OldStatus: statusOf(i, ""), NewStatus: StatusPendingVerification, Revision: &i.Revision,
	})
	return i, nil
}

// Verify 审核员核验：PASS → CLOSED；RETURN → IN_PROGRESS。提交人不能自核验。
func (s *service) Verify(ctx context.Context, actor *middleware.Actor, id, revision int64, decision, reason string) (*Issue, error) {
	if !hasAdminOrReviewer(actor) {
		return nil, ErrForbidden
	}
	i, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, ErrNotFound
	}
	if i.Status != StatusPendingVerification {
		return nil, ErrInvalidState
	}
	if i.Revision != revision {
		return nil, ErrStatusConflict
	}
	// 提交核验的人不能自行核验关闭。
	last, err := s.repo.ListActions(ctx, id)
	if err != nil {
		return nil, err
	}
	for idx := len(last) - 1; idx >= 0; idx-- {
		if last[idx].ActionType == ActionRequestVerify {
			if last[idx].ActorID == actor.UserID {
				return nil, ErrSelfVerify
			}
			break
		}
	}
	if decision != "PASS" && decision != "RETURN" {
		return nil, ErrInvalidState
	}
	if decision == "RETURN" && reason == "" {
		return nil, ErrReasonRequired
	}
	actionType := ActionVerifyPass
	next := StatusClosed
	if decision == "RETURN" {
		actionType = ActionVerifyReturn
		next = StatusInProgress
	}
	action := &IssueAction{IssueID: id, ActorID: actor.UserID, ActionType: actionType, Content: reason}
	if err := s.repo.InsertAction(ctx, action); err != nil {
		return nil, err
	}
	var n int64
	if next == StatusClosed {
		n, err = s.repo.Close(ctx, id, revision, reason)
	} else {
		n, err = s.repo.UpdateStatus(ctx, id, revision, next)
	}
	if err != nil {
		return nil, err
	}
	if n == 0 {
		return nil, ErrStatusConflict
	}
	i.Status = next
	i.Revision++
	if next == StatusClosed {
		now := time.Now().UTC()
		i.ClosedAt = &now
		i.CloseReason = reason
	}
	s.audit.Record(ctx, &audit.Event{
		EntityType: "issue", EntityID: id, Action: actionType, ActorID: actor.UserID,
		OldStatus: StatusPendingVerification, NewStatus: next, Revision: &i.Revision, Reason: reason,
	})
	return i, nil
}

// Reopen 关闭后重新打开（有权限的审核员，保留先前关闭记录）。
func (s *service) Reopen(ctx context.Context, actor *middleware.Actor, id, revision int64, reason string) (*Issue, error) {
	if !hasAdminOrReviewer(actor) {
		return nil, ErrForbidden
	}
	i, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, ErrNotFound
	}
	if i.Status != StatusClosed {
		return nil, ErrInvalidState
	}
	if reason == "" {
		return nil, ErrReasonRequired
	}
	if i.Revision != revision {
		return nil, ErrStatusConflict
	}
	action := &IssueAction{IssueID: id, ActorID: actor.UserID, ActionType: ActionReopen, Content: reason}
	if err := s.repo.InsertAction(ctx, action); err != nil {
		return nil, err
	}
	n, err := s.repo.UpdateStatus(ctx, id, revision, StatusOpen)
	if err != nil {
		return nil, err
	}
	if n == 0 {
		return nil, ErrStatusConflict
	}
	i.Status = StatusOpen
	i.ClosedAt = nil
	i.CloseReason = ""
	i.Revision++
	s.audit.Record(ctx, &audit.Event{
		EntityType: "issue", EntityID: id, Action: "reopen", ActorID: actor.UserID,
		OldStatus: StatusClosed, NewStatus: StatusOpen, Revision: &i.Revision, Reason: reason,
	})
	return i, nil
}

// AddActionImage 上传问题处置动作附件（受控存储 + images 表注册 + 关联动作）。
func (s *service) AddActionImage(ctx context.Context, actor *middleware.Actor, issueID, actionID int64, data []byte, mediaType string) (*ActionImage, error) {
	if s.images == nil {
		return nil, ErrForbidden
	}
	i, err := s.repo.GetByID(ctx, issueID)
	if err != nil {
		return nil, ErrNotFound
	}
	if !canAct(actor, i) {
		return nil, ErrForbidden
	}
	actions, err := s.repo.ListActions(ctx, issueID)
	if err != nil {
		return nil, err
	}
	found := false
	for _, a := range actions {
		if a.ID == actionID {
			found = true
			break
		}
	}
	if !found {
		return nil, ErrActionImageMiss
	}
	key, sha, err := s.images.Save(data, mediaType)
	if err != nil {
		return nil, err
	}
	w, h := imageSize(data)
	imgID, err := s.images.Register(ctx, key, sha, mediaType, int64(len(data)), w, h, actor.UserID)
	if err != nil {
		return nil, err
	}
	if err := s.repo.LinkActionImage(ctx, actionID, imgID); err != nil {
		return nil, err
	}
	return &ActionImage{
		ID: imgID, MediaType: mediaType, ByteSize: int64(len(data)),
		URL: "/api/v1/images/" + itoa(imgID) + "/content",
	}, nil
}

// AdmitFromObservation 受理已通过审核的记录：新建问题/关联已有问题/记录不立项原因。
// 已受理的记录再次受理返回已有关联（幂等）。
func (s *service) AdmitFromObservation(ctx context.Context, actor *middleware.Actor, obsID int64, in AdmitInput) (*AdmitResult, error) {
	if !hasAdminOrReviewer(actor) {
		return nil, ErrForbidden
	}
	if s.obsAccess == nil {
		return nil, ErrInvalidState
	}
	info, err := s.obsAccess.AcceptedAdmissionInfo(ctx, obsID)
	if err != nil {
		return nil, err
	}
	// 已有关联 → 幂等返回现状。
	existing, err := s.repo.ListByObservation(ctx, obsID)
	if err != nil {
		return nil, err
	}
	if len(existing) > 0 {
		ids := make([]int64, 0, len(existing))
		for _, e := range existing {
			ids = append(ids, e.IssueID)
		}
		return &AdmitResult{CreatedIssueIDs: []int64{}, LinkedIssueIDs: ids, AlreadyAdmitted: true}, nil
	}
	if len(in.NewIssues) == 0 && len(in.LinkIssueIDs) == 0 && in.NoIssueReason == "" {
		return nil, ErrAdmitEmpty
	}
	createdIDs, linkedIDs := []int64{}, []int64{}
	err = s.repo.InTx(ctx, func(tx Repository) error {
		for _, ni := range in.NewIssues {
			iss := &Issue{
				Title: ni.Title, SegmentID: ni.SegmentID, PointID: ni.PointID,
				LocationText: firstNonEmpty(ni.LocationText, info.LocationText),
				Description:  ni.Description,
				PriorityCode: ni.PriorityCode, PriorityReason: ni.PriorityReason,
				Status: StatusUnassigned, Revision: 1, OpenedAt: time.Now().UTC(),
			}
			if err := tx.Insert(ctx, iss); err != nil {
				return err
			}
			if err := tx.LinkObservation(ctx, &IssueObservation{
				IssueID: iss.ID, ObservationID: obsID, ReviewDecisionID: info.ReviewDecisionID,
				LinkReason: "受理新建", LinkedBy: actor.UserID, LinkedAt: time.Now().UTC(),
			}); err != nil {
				return err
			}
			createdIDs = append(createdIDs, iss.ID)
			s.audit.Record(ctx, &audit.Event{
				EntityType: "issue", EntityID: iss.ID, Action: "admit_create", ActorID: actor.UserID,
				NewStatus: StatusUnassigned, Reason: "from observation " + itoa(obsID),
			})
		}
		for _, lid := range in.LinkIssueIDs {
			iss, err := tx.GetByID(ctx, lid)
			if err != nil {
				return ErrNotFound
			}
			if iss.Status == StatusClosed {
				return ErrInvalidState
			}
			if err := tx.LinkObservation(ctx, &IssueObservation{
				IssueID: lid, ObservationID: obsID, ReviewDecisionID: info.ReviewDecisionID,
				LinkReason: "关联已有问题", LinkedBy: actor.UserID, LinkedAt: time.Now().UTC(),
			}); err != nil {
				return err
			}
			linkedIDs = append(linkedIDs, lid)
			s.audit.Record(ctx, &audit.Event{
				EntityType: "issue", EntityID: lid, Action: "admit_link", ActorID: actor.UserID,
				Reason: "from observation " + itoa(obsID),
			})
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	s.audit.Record(ctx, &audit.Event{
		EntityType: "observation", EntityID: obsID, Action: "admit", ActorID: actor.UserID,
		Reason: in.NoIssueReason,
	})
	return &AdmitResult{CreatedIssueIDs: createdIDs, LinkedIssueIDs: linkedIDs, NoIssueReason: in.NoIssueReason}, nil
}

func hasAdminOrReviewer(actor *middleware.Actor) bool {
	if actor == nil {
		return false
	}
	for _, r := range actor.Roles {
		if r == "ADMIN" || r == "REVIEWER" {
			return true
		}
	}
	return false
}

func hasRole(actor *middleware.Actor, role string) bool {
	if actor == nil {
		return false
	}
	for _, r := range actor.Roles {
		if r == role {
			return true
		}
	}
	return false
}

// canAct 责任人本人或管理员/审核员可对问题操作。
func canAct(actor *middleware.Actor, i *Issue) bool {
	if hasAdminOrReviewer(actor) {
		return true
	}
	return i.AssigneeID != nil && *i.AssigneeID == actor.UserID
}

func statusOf(i *Issue, _ string) string { return i.Status }

func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

func itoa(n int64) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}

func imageSize(data []byte) (int, int) {
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return 0, 0
	}
	return cfg.Width, cfg.Height
}
