package report

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"io"
	"log"
	"os"
	"path/filepath"
	"time"

	"citywall/server/internal/config"
	"citywall/server/internal/infrastructure/imageutil"
	"citywall/server/internal/middleware"
	"citywall/server/internal/module/audit"
)

// ReportStorage 报告与图片文件存储（由 infrastructure/storage 实现）。
type ReportStorage interface {
	// SaveReport 落盘 PDF 报告，返回相对 key。
	SaveReport(data []byte) (string, error)
	// OpenReport 打开报告文件。
	OpenReport(relKey string) (*os.File, error)
	// Open 打开图片文件（按受控目录）。
	Open(relKey string) (*os.File, error)
}

// Service 报告业务规则。
type Service interface {
	// CreateTask 生成单任务报告（同步生成，失败保留 FAILED 记录可重试）。
	CreateTask(ctx context.Context, actor *middleware.Actor, taskID int64) (*Report, error)
	// CreateRange 生成时间范围报告。
	CreateRange(ctx context.Context, actor *middleware.Actor, start, end time.Time) (*Report, error)
	// List 报告台账。
	List(ctx context.Context, actor *middleware.Actor, scopeType string, taskID *int64, page, pageSize int) (*ListResult, error)
	// Get 报告详情。
	Get(ctx context.Context, actor *middleware.Actor, id int64) (*Report, error)
	// Retry 重试失败的报告生成。
	Retry(ctx context.Context, actor *middleware.Actor, id int64) (*Report, error)
	// OpenFile 下载已生成 PDF（鉴权后由接口输出）。
	OpenFile(ctx context.Context, actor *middleware.Actor, id int64) (*FileInfo, *os.File, error)
}

// ListResult 台账分页结果。
type ListResult struct {
	Items []Report `json:"items"`
	Total int64    `json:"total"`
}

// StatusLabel 中文化状态。
func StatusLabel(s string) string {
	switch s {
	case StatusReady:
		return "READY"
	case StatusFailed:
		return "FAILED"
	default:
		return "GENERATING"
	}
}

type service struct {
	repo    Repository
	source  SnapshotSource
	storage ReportStorage
	cfg     config.ReportConfig
	tempDir string
	audit   audit.Recorder
}

// NewService 构造报告服务。
func NewService(repo Repository, source SnapshotSource, storage ReportStorage, cfg config.ReportConfig, tempDir string, rec audit.Recorder) Service {
	return &service{repo: repo, source: source, storage: storage, cfg: cfg, tempDir: tempDir, audit: rec}
}

func (s *service) CreateTask(ctx context.Context, actor *middleware.Actor, taskID int64) (*Report, error) {
	if !hasAdminOrReviewer(actor) {
		return nil, ErrForbidden
	}
	rep := &Report{
		ScopeType: ScopeTask, TaskID: &taskID, GeneratedBy: actor.UserID,
		SnapshotAt: time.Now().UTC(), Status: StatusGenerating, SnapshotJSON: "{}",
	}
	if err := s.repo.Insert(ctx, rep); err != nil {
		return nil, err
	}
	return s.generate(ctx, actor, rep)
}

func (s *service) CreateRange(ctx context.Context, actor *middleware.Actor, start, end time.Time) (*Report, error) {
	if !hasAdminOrReviewer(actor) {
		return nil, ErrForbidden
	}
	if !end.After(start) {
		return nil, ErrInvalidScope
	}
	rep := &Report{
		ScopeType: ScopeDateRange, RangeStart: &start, RangeEnd: &end,
		GeneratedBy: actor.UserID, SnapshotAt: time.Now().UTC(),
		Status: StatusGenerating, SnapshotJSON: "{}",
	}
	if err := s.repo.Insert(ctx, rep); err != nil {
		return nil, err
	}
	return s.generate(ctx, actor, rep)
}

func (s *service) List(ctx context.Context, actor *middleware.Actor, scopeType string, taskID *int64, page, pageSize int) (*ListResult, error) {
	if !hasAdminOrReviewer(actor) {
		return nil, ErrForbidden
	}
	if scopeType != "" && scopeType != ScopeTask && scopeType != ScopeDateRange {
		return nil, ErrInvalidScope
	}
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}
	items, err := s.repo.ListPage(ctx, scopeType, taskID, pageSize, (page-1)*pageSize)
	if err != nil {
		return nil, err
	}
	total, err := s.repo.Count(ctx, scopeType, taskID)
	if err != nil {
		return nil, err
	}
	return &ListResult{Items: items, Total: total}, nil
}

func (s *service) Get(ctx context.Context, actor *middleware.Actor, id int64) (*Report, error) {
	if !hasAdminOrReviewer(actor) {
		return nil, ErrForbidden
	}
	rep, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, ErrNotFound
	}
	return rep, nil
}

func (s *service) Retry(ctx context.Context, actor *middleware.Actor, id int64) (*Report, error) {
	if !hasAdminOrReviewer(actor) {
		return nil, ErrForbidden
	}
	rep, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, ErrNotFound
	}
	if rep.Status != StatusFailed {
		return nil, ErrBadState
	}
	n, err := s.repo.MarkGenerating(ctx, id)
	if err != nil {
		return nil, err
	}
	if n == 0 {
		return nil, ErrBadState
	}
	rep.Status = StatusGenerating
	return s.generate(ctx, actor, rep)
}

// generate 加载快照 → 渲染 PDF → 落盘 → 标记 READY；任何失败标记 FAILED 并返回错误。
func (s *service) generate(ctx context.Context, actor *middleware.Actor, rep *Report) (*Report, error) {
	now := time.Now().UTC()
	rep.SnapshotAt = now
	var snap any
	var loadErr error
	switch rep.ScopeType {
	case ScopeTask:
		var value *TaskSource
		value, loadErr = s.source.LoadTask(ctx, *rep.TaskID)
		if value != nil {
			snap = value
		}
	case ScopeDateRange:
		var value *RangeSource
		value, loadErr = s.source.LoadRange(ctx, *rep.RangeStart, *rep.RangeEnd)
		if value != nil {
			snap = value
		}
	default:
		_ = s.repo.MarkFailed(ctx, rep.ID, "INVALID_SCOPE")
		rep.Status = StatusFailed
		return rep, ErrInvalidScope
	}
	if snap == nil || loadErr != nil {
		_ = s.repo.MarkFailed(ctx, rep.ID, "SNAPSHOT_LOAD_FAILED")
		rep.Status = StatusFailed
		return rep, ErrTaskNotFound
	}
	jsonStr, err := snapshotJSON(snap)
	if err != nil {
		_ = s.repo.MarkFailed(ctx, rep.ID, "SNAPSHOT_LOAD_FAILED")
		rep.Status = StatusFailed
		return rep, err
	}
	opts := renderOpts{
		GeneratedBy: actorName(actor),
		SnapshotAt:  now.In(reportLocation).Format("2006-01-02 15:04:05") + " (UTC+8)",
		FontPath:    s.cfg.FontPath,
		OpenImage: func(key string) (string, int, int, string, func(), error) {
			f, err := s.storage.Open(key)
			if err != nil {
				return "", 0, 0, "", nil, err
			}
			data, err := io.ReadAll(f)
			f.Close()
			if err != nil {
				return "", 0, 0, "", nil, err
			}
			preview, err := imageutil.Preview(data, guessMediaType(key))
			if err != nil {
				return "", 0, 0, "", nil, err
			}
			data = preview.Data
			w, h, mediaType, ext := preview.Width, preview.Height, preview.MediaType, "png"
			if mediaType == "image/jpeg" {
				ext = "jpg"
			}
			var b [8]byte
			if _, err := rand.Read(b[:]); err != nil {
				return "", 0, 0, "", nil, err
			}
			tmp := filepath.Join(s.tempDir, "report-img-"+hex.EncodeToString(b[:])+"."+ext)
			if err := os.WriteFile(tmp, data, 0o600); err != nil {
				return "", 0, 0, "", nil, err
			}
			return tmp, w, h, mediaType, func() { _ = os.Remove(tmp) }, nil
		},
	}
	var data []byte
	switch rep.ScopeType {
	case ScopeTask:
		data, err = renderTaskPDF(snap.(*TaskSource), opts)
	case ScopeDateRange:
		data, err = renderRangePDF(snap.(*RangeSource), opts)
	}
	if err != nil {
		_ = s.repo.MarkFailed(ctx, rep.ID, "PDF_GENERATION_FAILED")
		log.Printf("report generation failed: id=%d error=%v", rep.ID, err)
		rep.Status = StatusFailed
		return rep, err
	}
	key, err := s.storage.SaveReport(data)
	if err != nil {
		_ = s.repo.MarkFailed(ctx, rep.ID, "PDF_STORAGE_FAILED")
		rep.Status = StatusFailed
		return rep, err
	}
	if err := s.repo.MarkReady(ctx, rep.ID, key, jsonStr); err != nil {
		_ = s.repo.MarkFailed(ctx, rep.ID, "DB_UPDATE_FAILED")
		rep.Status = StatusFailed
		return rep, err
	}
	rep.Status = StatusReady
	rep.FileKey = &key
	rep.SnapshotJSON = jsonStr
	rep.FinishedAt = &now
	s.audit.Record(ctx, &audit.Event{
		EntityType: "report", EntityID: rep.ID, Action: "generate:" + rep.ScopeType,
		ActorID: actor.UserID, Reason: itoa64(rep.ID),
	})
	return rep, nil
}

func (s *service) OpenFile(ctx context.Context, actor *middleware.Actor, id int64) (*FileInfo, *os.File, error) {
	if !hasAdminOrReviewer(actor) {
		return nil, nil, ErrForbidden
	}
	rep, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, nil, ErrNotFound
	}
	if rep.Status != StatusReady || rep.FileKey == nil {
		return nil, nil, ErrNotReady
	}
	f, err := s.storage.OpenReport(*rep.FileKey)
	if err != nil {
		return nil, nil, err
	}
	st, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, nil, err
	}
	return &FileInfo{FileKey: *rep.FileKey, MediaType: "application/pdf", ByteSize: st.Size()}, f, nil
}

// ---------- 工具 ----------

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

func actorName(actor *middleware.Actor) string {
	if actor == nil {
		return "-"
	}
	return "user#" + itoa64(actor.UserID)
}

func guessMediaType(key string) string {
	if len(key) >= 4 && key[len(key)-4:] == ".png" {
		return "image/png"
	}
	if len(key) >= 4 && key[len(key)-4:] == ".jpg" {
		return "image/jpeg"
	}
	if len(key) >= 5 && key[len(key)-5:] == ".webp" {
		return "image/webp"
	}
	return "image/jpeg"
}

func itoa64(n int64) string {
	if n == 0 {
		return "0"
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}
