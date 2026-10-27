package report

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strconv"
	"time"

	"gorm.io/gorm"
)

// snapshotRepository 实现 SnapshotSource：跨业务表做只读快照查询。
type snapshotRepository struct {
	db *gorm.DB
}

// NewSnapshotSource 构造快照查询源。
func NewSnapshotSource(db *gorm.DB) SnapshotSource {
	return &snapshotRepository{db: db}
}

// ErrTaskNotFound 任务不存在（报告生成失败的错误码来源）。
var ErrTaskNotFound = errors.New("report: task not found")

// taskRow 任务查询行。
type taskRow struct {
	ID           int64
	Title        string
	AssigneeID   int64
	AssigneeName string
	DueAt        *time.Time
	Status       string
	Description  string
	DispatchedAt *time.Time
	CompletedAt  *time.Time
}

// pointRow 任务点查询行。
type pointRow struct {
	TaskPointID       int64
	SequenceNo        int
	SnapshotName      *string
	PointCode         string
	PointName         string
	SnapshotLatitude  *float64
	SnapshotLongitude *float64
	SnapshotRadius    *float64
	SnapshotHint      *string
}

func (s *snapshotRepository) LoadTask(ctx context.Context, taskID int64) (*TaskSource, error) {
	var result *TaskSource
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var err error
		result, err = (&snapshotRepository{db: tx}).loadTask(ctx, taskID)
		return err
	}, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	return result, err
}

func (s *snapshotRepository) LoadRange(ctx context.Context, start, end time.Time) (*RangeSource, error) {
	var result *RangeSource
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var err error
		result, err = (&snapshotRepository{db: tx}).loadRange(ctx, start, end)
		return err
	}, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	return result, err
}

func (s *snapshotRepository) loadTask(ctx context.Context, taskID int64) (*TaskSource, error) {
	var t taskRow
	err := s.db.WithContext(ctx).Table("tasks").
		Select("tasks.id, tasks.title, tasks.assignee_id, u.display_name AS assignee_name, "+
			"tasks.due_at, tasks.status, tasks.description, tasks.dispatched_at, tasks.completed_at").
		Joins("LEFT JOIN users u ON u.id = tasks.assignee_id").
		Where("tasks.id = ?", taskID).Scan(&t).Error
	if err != nil {
		return nil, err
	}
	if t.ID == 0 {
		return nil, ErrTaskNotFound
	}
	var rows []pointRow
	err = s.db.WithContext(ctx).Table("task_points").
		Select("task_points.id AS task_point_id, task_points.sequence_no, task_points.snapshot_name, "+
			"ip.code AS point_code, COALESCE(task_points.snapshot_name, ip.name) AS point_name, "+
			"task_points.snapshot_latitude, task_points.snapshot_longitude, task_points.snapshot_radius_m, "+
			"task_points.snapshot_location_hint").
		Joins("LEFT JOIN inspection_points ip ON ip.id = task_points.point_id").
		Where("task_points.task_id = ?", taskID).
		Order("task_points.sequence_no ASC").Scan(&rows).Error
	if err != nil {
		return nil, err
	}

	src := &TaskSource{
		TaskID: t.ID, Title: t.Title, AssigneeID: t.AssigneeID, AssigneeName: t.AssigneeName,
		DueAt: t.DueAt, Status: t.Status, Description: t.Description,
		DispatchedAt: t.DispatchedAt, CompletedAt: t.CompletedAt,
		PointTotal: len(rows),
	}
	// 该任务所有点位关联的记录。
	pointIDs := make([]int64, 0, len(rows))
	for _, r := range rows {
		pointIDs = append(pointIDs, r.TaskPointID)
	}
	obsViews, err := s.loadObservationsByTaskPoints(ctx, pointIDs)
	if err != nil {
		return nil, err
	}
	obsByPoint := map[int64]*ObservationView{}
	for _, ov := range obsViews {
		if ov.TaskPointID != nil {
			obsByPoint[*ov.TaskPointID] = ov
		}
	}
	for _, row := range rows {
		pt := PointView{
			TaskPointID: row.TaskPointID, SequenceNo: row.SequenceNo,
			PointName: row.PointName, PointCode: row.PointCode,
		}
		if row.SnapshotLatitude != nil && row.SnapshotLongitude != nil {
			pt.SnapshotLocation = locText(*row.SnapshotLatitude, *row.SnapshotLongitude, row.SnapshotHint)
		}
		if o, ok := obsByPoint[row.TaskPointID]; ok {
			pt.Observation = o
			if o.Status == "ACCEPTED" && o.Outcome == "UNREACHABLE" {
				src.PointUnreachable++
				src.PointUncovered++
			} else if o.Status == "SUBMITTED" || o.Status == "ACCEPTED" {
				src.PointCovered++
				if o.Outcome == "NORMAL" {
					src.PointNormal++
				}
				if o.Outcome == "SUSPECTED" {
					src.PointSuspected++
				}
			} else {
				src.PointUncovered++
			}
		} else {
			src.PointUncovered++ // 无有效记录 → 本次未覆盖
		}
		src.Points = append(src.Points, pt)
	}

	// 关联问题（任务点记录 → 问题）。
	issueIDs, err := s.issuesForTaskPoints(ctx, pointIDs)
	if err != nil {
		return nil, err
	}
	if len(issueIDs) > 0 {
		src.Issues, err = s.loadIssues(ctx, issueIDs)
		if err != nil {
			return nil, err
		}
	}
	// 汇总错误识别统计。
	src.RejectedCount, src.MissingCount, src.RetryCount = aggregateRecognitionStats(obsViews)
	return src, nil
}

func (s *snapshotRepository) loadRange(ctx context.Context, start, end time.Time) (*RangeSource, error) {
	var obsIDs []int64
	err := s.db.WithContext(ctx).Table("observations").
		Where("submitted_at >= ? AND submitted_at <= ?", start, end).
		Pluck("id", &obsIDs).Error
	if err != nil {
		return nil, err
	}
	src := &RangeSource{Start: start, End: end, TotalRecords: len(obsIDs)}
	views, err := s.loadObservationsByIDs(ctx, obsIDs)
	if err != nil {
		return nil, err
	}
	for _, v := range views {
		src.Observations = append(src.Observations, *v)
	}
	for _, v := range views {
		switch v.Status {
		case "SUBMITTED":
			src.SubmittedCount++
		case "ACCEPTED":
			src.AcceptedCount++
		case "RETURNED":
			src.ReturnedCount++
		case "REJECTED":
			src.RejectedCount++
		}
		switch v.Outcome {
		case "NORMAL":
			src.NormalCount++
		case "SUSPECTED":
			src.SuspectedCount++
		case "UNREACHABLE":
			src.UnreachableCount++
		}
	}
	// 范围报告问题：与范围内记录关联的问题 + 范围内新建的问题。
	issueIDs, err := s.issuesForObservations(ctx, obsIDs)
	if err != nil {
		return nil, err
	}
	rangeIssueIDs, err := s.issuesOpenedInRange(ctx, start, end)
	if err != nil {
		return nil, err
	}
	all := mergeUnique(issueIDs, rangeIssueIDs)
	if len(all) > 0 {
		src.Issues, err = s.loadIssues(ctx, all)
		if err != nil {
			return nil, err
		}
	}
	return src, nil
}

// ---------- 记录组装 ----------

func (s *snapshotRepository) loadObservationsByTaskPoints(ctx context.Context, pointIDs []int64) ([]*ObservationView, error) {
	if len(pointIDs) == 0 {
		return nil, nil
	}
	var obs []Observation
	err := s.db.WithContext(ctx).Where("task_point_id IN ?", pointIDs).Find(&obs).Error
	if err != nil {
		return nil, err
	}
	ids := make([]int64, 0, len(obs))
	for _, o := range obs {
		ids = append(ids, o.ID)
	}
	return s.assembleObservations(ctx, obs, ids)
}

func (s *snapshotRepository) loadObservationsByIDs(ctx context.Context, obsIDs []int64) ([]*ObservationView, error) {
	if len(obsIDs) == 0 {
		return nil, nil
	}
	var obs []Observation
	err := s.db.WithContext(ctx).Where("id IN ?", obsIDs).Order("id ASC").Find(&obs).Error
	if err != nil {
		return nil, err
	}
	return s.assembleObservations(ctx, obs, obsIDs)
}

// assembleObservations 批量组装记录的图片/识别/结论/审核视图。
func (s *snapshotRepository) assembleObservations(ctx context.Context, obs []Observation, ids []int64) ([]*ObservationView, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	views := make([]*ObservationView, 0, len(obs))
	byID := map[int64]*ObservationView{}
	for i := range obs {
		o := &obs[i]
		v := &ObservationView{
			ID: o.ID, Source: o.Source, TaskPointID: o.TaskPointID, ReporterID: o.ReporterID,
			LocationText: o.LocationText, Latitude: o.Latitude, Longitude: o.Longitude,
			AccuracyM: o.AccuracyM, DistanceM: o.DistanceM, Status: o.Status, Outcome: o.InspectionOutcome,
			Reviews:     []ReviewView{},
			SubmittedAt: o.SubmittedAt, ReviewedAt: o.ReviewedAt, AdmittedAt: o.AdmittedAt, AdmittedBy: o.AdmittedBy, NoIssueReason: o.NoIssueReason,
		}
		v.LocationAbnormal = locationAbnormal(*o)
		views = append(views, v)
		byID[o.ID] = v
	}

	// 图片。
	var rels []ObservationImage
	if err := s.db.WithContext(ctx).Where("observation_id IN ?", ids).Find(&rels).Error; err != nil {
		return nil, err
	}
	var subs []ObservationSubmission
	if err := s.db.WithContext(ctx).Where("observation_id IN ?", ids).Order("observation_id, submission_no, id").Find(&subs).Error; err != nil {
		return nil, err
	}
	linked := map[[2]int64]bool{}
	for _, rel := range rels {
		linked[[2]int64{rel.ObservationID, rel.ImageID}] = true
	}
	for _, submission := range subs {
		var snapshotImages []int64
		if err := json.Unmarshal([]byte(submission.ImageIDsSnapshotJSON), &snapshotImages); err != nil {
			return nil, err
		}
		for _, imageID := range snapshotImages {
			key := [2]int64{submission.ObservationID, imageID}
			if !linked[key] {
				rels = append(rels, ObservationImage{ObservationID: submission.ObservationID, ImageID: imageID, ImageRole: "ARCHIVED"})
				linked[key] = true
			}
		}
	}
	imgIDs := make([]int64, 0, len(rels))
	for _, r := range rels {
		imgIDs = append(imgIDs, r.ImageID)
	}
	var imgs []Image
	if len(imgIDs) > 0 {
		if err := s.db.WithContext(ctx).Where("id IN ?", imgIDs).Find(&imgs).Error; err != nil {
			return nil, err
		}
	}
	imgByID := map[int64]Image{}
	for _, im := range imgs {
		imgByID[im.ID] = im
	}
	imgRole := map[int64]string{}
	for _, r := range rels {
		imgRole[r.ImageID] = r.ImageRole
	}
	for _, r := range rels {
		im, ok := imgByID[r.ImageID]
		if !ok {
			continue
		}
		v := byID[r.ObservationID]
		if v == nil {
			continue
		}
		v.Images = append(v.Images, ImageView{
			ID: im.ID, StorageKey: im.StorageKey, MediaType: im.MediaType,
			Width: im.Width, Height: im.Height, Role: imgRole[im.ID],
		})
	}

	// 识别尝试 + 候选。
	var attempts []RecognitionAttempt
	if err := s.db.WithContext(ctx).Where("image_id IN ?", imgIDs).Order("attempt_no ASC").Find(&attempts).Error; err != nil {
		return nil, err
	}
	if len(attempts) > 0 {
		attemptIDs := make([]int64, 0, len(attempts))
		for _, a := range attempts {
			attemptIDs = append(attemptIDs, a.ID)
		}
		var dets []Detection
		if err := s.db.WithContext(ctx).Where("attempt_id IN ?", attemptIDs).Find(&dets).Error; err != nil {
			return nil, err
		}
		detByAttempt := map[int64][]DetectionView{}
		for _, d := range dets {
			detByAttempt[d.AttemptID] = append(detByAttempt[d.AttemptID], DetectionView{
				ClassCode: d.ClassCode, Confidence: d.Confidence, X1: d.X1, Y1: d.Y1, X2: d.X2, Y2: d.Y2,
			})
		}
		for _, a := range attempts {
			av := AttemptView{
				ImageID:      a.ImageID,
				AnalysisJSON: a.AnalysisJSON,
				AttemptNo:    a.AttemptNo, Status: a.Status, ModelVersion: a.ModelVersion,
				ErrorCode: a.ErrorCode, LatencyMs: a.LatencyMs,
				Detections: detByAttempt[a.ID],
			}
			for _, r := range rels {
				if r.ImageID == a.ImageID {
					byID[r.ObservationID].Attempts = append(byID[r.ObservationID].Attempts, av)
					break
				}
			}
		}
	}

	// 结论版本 + 结论项。
	var cons []ConclusionRevision
	if err := s.db.WithContext(ctx).Where("observation_id IN ?", ids).Order("revision_no ASC").Find(&cons).Error; err != nil {
		return nil, err
	}
	if len(cons) > 0 {
		conIDs := make([]int64, 0, len(cons))
		for _, c := range cons {
			conIDs = append(conIDs, c.ID)
		}
		var items []ConclusionItem
		if err := s.db.WithContext(ctx).Where("conclusion_revision_id IN ?", conIDs).Find(&items).Error; err != nil {
			return nil, err
		}
		itemsByCon := map[int64][]ItemView{}
		for _, it := range items {
			itemsByCon[it.ConclusionRevisionID] = append(itemsByCon[it.ConclusionRevisionID], ItemView{
				ImageID: it.ImageID, Decision: it.Decision, FinalClassCode: it.FinalClassCode,
				FinalBoxJSON: it.FinalBoxJSON, Description: it.Description, SourceDetectionID: it.SourceDetectionID,
			})
		}
		for _, c := range cons {
			cv := ConclusionView{
				RevisionNo: c.RevisionNo, Outcome: c.Outcome, Description: c.Description,
				PriorityCode: c.PriorityCode, AuthorID: c.AuthorID, CreatedAt: c.CreatedAt,
				Items: itemsByCon[c.ID],
			}
			byID[c.ObservationID].Conclusions = append(byID[c.ObservationID].Conclusions, cv)
		}
	}

	// 审核决定（经提交快照关联）。
	if len(subs) > 0 {
		subIDs := make([]int64, 0, len(subs))
		for _, su := range subs {
			subIDs = append(subIDs, su.ID)
		}
		var revs []ReviewDecision
		if err := s.db.WithContext(ctx).Table("review_decisions AS r").Select("r.*, COALESCE(u.display_name, '') AS reviewer_name").
			Joins("LEFT JOIN users u ON u.id = r.reviewer_id").Where("r.submission_id IN ?", subIDs).Scan(&revs).Error; err != nil {
			return nil, err
		}
		reviewBySubmission := map[int64]ReviewDecision{}
		for _, rv := range revs {
			reviewBySubmission[rv.SubmissionID] = rv
		}
		for _, submission := range subs {
			rv, ok := reviewBySubmission[submission.ID]
			if !ok {
				continue
			}
			rvw := ReviewView{
				SubmissionNo: submission.SubmissionNo, ReviewerName: rv.ReviewerName,
				Decision: rv.Decision, ReviewerID: rv.ReviewerID, Disposition: rv.Disposition,
				Reason: rv.Reason, CreatedAt: rv.CreatedAt,
			}
			byID[submission.ObservationID].Reviews = append(byID[submission.ObservationID].Reviews, rvw)
		}
	}
	return views, nil
}

// ---------- 问题查询 ----------

func (s *snapshotRepository) issuesForTaskPoints(ctx context.Context, pointIDs []int64) ([]int64, error) {
	if len(pointIDs) == 0 {
		return nil, nil
	}
	var obsIDs []int64
	if err := s.db.WithContext(ctx).Table("observations").
		Where("task_point_id IN ?", pointIDs).Pluck("id", &obsIDs).Error; err != nil {
		return nil, err
	}
	return s.issuesForObservations(ctx, obsIDs)
}

func (s *snapshotRepository) issuesForObservations(ctx context.Context, obsIDs []int64) ([]int64, error) {
	if len(obsIDs) == 0 {
		return nil, nil
	}
	var ids []int64
	err := s.db.WithContext(ctx).Table("issue_observations").
		Where("observation_id IN ?", obsIDs).Distinct().Pluck("issue_id", &ids).Error
	return ids, err
}

func (s *snapshotRepository) issuesOpenedInRange(ctx context.Context, start, end time.Time) ([]int64, error) {
	var ids []int64
	err := s.db.WithContext(ctx).Table("issues").
		Where("opened_at >= ? AND opened_at <= ?", start, end).Pluck("id", &ids).Error
	return ids, err
}

func (s *snapshotRepository) loadIssues(ctx context.Context, ids []int64) ([]IssueLinkView, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	rows := []IssueLinkView{}
	err := s.db.WithContext(ctx).Table("issues AS i").
		Select("i.id, i.title, i.status, i.priority_code, i.assignee_id, COALESCE(u.display_name,'') AS assignee_name, i.opened_at, i.closed_at, COALESCE(i.close_reason,'') AS close_reason").
		Joins("LEFT JOIN users u ON u.id = i.assignee_id").Where("i.id IN ?", ids).Order("i.opened_at, i.id").Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	actions := []IssueActionView{}
	err = s.db.WithContext(ctx).Table("issue_actions AS a").
		Select("a.id, a.issue_id, a.actor_id, COALESCE(u.display_name,'') AS actor_name, a.action_type, a.content, a.planned_follow_up_at, a.created_at").
		Joins("LEFT JOIN users u ON u.id = a.actor_id").Where("a.issue_id IN ?", ids).Order("a.created_at, a.id").Scan(&actions).Error
	if err != nil {
		return nil, err
	}
	index := map[int64]int{}
	type actionPhoto struct {
		ActionID int64
		ImageView
	}
	var photos []actionPhoto
	err = s.db.WithContext(ctx).Table("issue_action_images AS ai").
		Select("ai.issue_action_id AS action_id, i.id, i.storage_key, i.media_type, i.width, i.height").
		Joins("JOIN images AS i ON i.id = ai.image_id").
		Joins("JOIN issue_actions AS a ON a.id = ai.issue_action_id").
		Where("a.issue_id IN ?", ids).Order("i.id").Scan(&photos).Error
	if err != nil {
		return nil, err
	}
	byAction := map[int64][]ImageView{}
	for _, photo := range photos {
		photo.Role = "处置附件"
		byAction[photo.ActionID] = append(byAction[photo.ActionID], photo.ImageView)
	}
	for idx := range rows {
		rows[idx].Actions = []IssueActionView{}
		index[rows[idx].ID] = idx
	}
	for _, action := range actions {
		action.Images = byAction[action.ID]
		if action.Images == nil {
			action.Images = []ImageView{}
		}
		if idx, ok := index[action.IssueID]; ok {
			rows[idx].Actions = append(rows[idx].Actions, action)
		}
	}
	return rows, nil
}

// ---------- 工具 ----------

// locationAbnormal 依据 02 文档判定定位异常（距离超限或精度过差）。
func locationAbnormal(o Observation) string {
	if o.DistanceM != nil && *o.DistanceM > 50 {
		return "距目标点超过 50m"
	}
	if o.AccuracyM != nil && *o.AccuracyM > 30 {
		return "定位精度超过 30m"
	}
	return ""
}

func locText(lat, lon float64, hint *string) string {
	parts := make([]rune, 0, 48)
	for _, c := range formatCoord(lat) {
		parts = append(parts, c)
	}
	parts = append(parts, ',')
	for _, c := range formatCoord(lon) {
		parts = append(parts, c)
	}
	if hint != nil && *hint != "" {
		parts = append(parts, ' ', '(', ' ')
		for _, c := range *hint {
			parts = append(parts, c)
		}
		parts = append(parts, ')')
	}
	return string(parts)
}

func formatCoord(v float64) string {
	return strconv.FormatFloat(v, 'f', 6, 64)
}

// aggregateRecognitionStats 汇总误检/漏检/重新识别。
func aggregateRecognitionStats(views []*ObservationView) (rejected, missing, retry int) {
	for _, v := range views {
		for _, a := range v.Attempts {
			if a.AttemptNo > 1 {
				retry++
			}
		}
		for _, c := range v.Conclusions {
			for _, it := range c.Items {
				switch it.Decision {
				case "REJECT":
					rejected++
				case "ADD":
					missing++
				}
			}
		}
	}
	return
}

func mergeUnique(a, b []int64) []int64 {
	seen := map[int64]bool{}
	out := make([]int64, 0, len(a)+len(b))
	for _, v := range append(a, b...) {
		if seen[v] {
			continue
		}
		seen[v] = true
		out = append(out, v)
	}
	return out
}

// 复用 observation 模块模型做只读查询（不导入避免环依赖；定义同构行结构）。
type Observation struct {
	AdmittedAt        *time.Time
	AdmittedBy        *int64
	NoIssueReason     string
	ID                int64
	Source            string
	TaskPointID       *int64
	ReporterID        int64
	LocationText      string
	Latitude          *float64
	Longitude         *float64
	AccuracyM         *float64
	DistanceM         *float64
	Status            string
	InspectionOutcome string
	SubmittedAt       *time.Time
	ReviewedAt        *time.Time
}
type ObservationImage struct {
	ObservationID int64
	ImageID       int64
	ImageRole     string
}
type Image struct {
	ID         int64
	StorageKey string
	MediaType  string
	Width      int
	Height     int
}
type RecognitionAttempt struct {
	AnalysisJSON string
	ID           int64
	ImageID      int64
	AttemptNo    int
	Status       string
	ModelVersion string
	ErrorCode    string
	LatencyMs    int
}
type Detection struct {
	ID             int64
	AttemptID      int64
	ClassCode      string
	Confidence     float64
	X1, Y1, X2, Y2 int
}
type ConclusionRevision struct {
	ID            int64
	ObservationID int64
	RevisionNo    int
	Outcome       string
	Description   string
	PriorityCode  string
	AuthorID      int64
	CreatedAt     time.Time
}
type ConclusionItem struct {
	ID                   int64
	ConclusionRevisionID int64
	ImageID              int64
	SourceDetectionID    *int64
	Decision             string
	FinalClassCode       string
	FinalBoxJSON         string
	Description          string
}
type ObservationSubmission struct {
	ImageIDsSnapshotJSON string
	ID                   int64
	ObservationID        int64
	SubmissionNo         int
}
type ReviewDecision struct {
	ReviewerName string
	ID           int64
	SubmissionID int64
	ReviewerID   int64
	Decision     string
	Disposition  string
	Reason       string
	CreatedAt    time.Time
}

func (Observation) TableName() string           { return "observations" }
func (ObservationImage) TableName() string      { return "observation_images" }
func (Image) TableName() string                 { return "images" }
func (RecognitionAttempt) TableName() string    { return "recognition_attempts" }
func (Detection) TableName() string             { return "detections" }
func (ConclusionRevision) TableName() string    { return "conclusion_revisions" }
func (ConclusionItem) TableName() string        { return "conclusion_items" }
func (ObservationSubmission) TableName() string { return "observation_submissions" }
func (ReviewDecision) TableName() string        { return "review_decisions" }

// snapshotJSON 将快照序列化为 JSON。
func snapshotJSON(v any) (string, error) {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return "", err
	}
	return string(b), nil
}
