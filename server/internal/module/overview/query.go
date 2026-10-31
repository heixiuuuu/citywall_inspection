package overview

import (
	"context"
	"database/sql"
	"time"

	"gorm.io/gorm"
)

type Count struct {
	Code  string `json:"code"`
	Total int64  `json:"total"`
}

type Summary struct {
	GeneratedAt       time.Time `json:"generated_at"`
	Tasks             []Count   `json:"tasks"`
	Records           []Count   `json:"records"`
	Issues            []Count   `json:"issues"`
	Categories        []Count   `json:"categories"`
	OverdueTasks      int64     `json:"overdue_tasks"`
	PendingAdmission  int64     `json:"pending_admission"`
	UrgentIssues      int64     `json:"urgent_issues"`
	ActivePoints      int64     `json:"active_points"`
	MissingLocation   int64     `json:"missing_location"`
	LowAccuracy       int64     `json:"low_accuracy"`
	FailedRecognition int64     `json:"failed_recognition"`
}

type Query struct{ db *gorm.DB }

func NewQuery(db *gorm.DB) *Query { return &Query{db: db} }

func (q *Query) Load(ctx context.Context, now time.Time) (*Summary, error) {
	s := &Summary{GeneratedAt: now.UTC(), Tasks: []Count{}, Records: []Count{}, Issues: []Count{}, Categories: []Count{}}
	err := q.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, group := range []struct {
			table  string
			target *[]Count
		}{
			{"tasks", &s.Tasks}, {"observations", &s.Records}, {"issues", &s.Issues},
		} {
			if err := tx.Table(group.table).Select("status AS code, COUNT(*) AS total").Group("status").Order("status").Scan(group.target).Error; err != nil {
				return err
			}
		}
		for _, query := range []struct {
			sql    string
			args   []any
			target *int64
		}{
			{"SELECT COUNT(*) FROM tasks WHERE due_at < ? AND status IN ('ASSIGNED','IN_PROGRESS','RETURNED','PENDING_REVIEW')", []any{now.UTC()}, &s.OverdueTasks},
			{"SELECT COUNT(*) FROM observations WHERE status = 'ACCEPTED' AND admitted_at IS NULL", nil, &s.PendingAdmission},
			{"SELECT COUNT(*) FROM issues WHERE status <> 'CLOSED' AND priority_code = 'URGENT'", nil, &s.UrgentIssues},
			{"SELECT COUNT(*) FROM inspection_points p JOIN wall_segments s ON s.id = p.segment_id WHERE p.active = 1 AND s.active = 1", nil, &s.ActivePoints},
			{"SELECT COUNT(*) FROM observations WHERE status IN ('SUBMITTED','ACCEPTED','RETURNED','REJECTED') AND (latitude IS NULL OR longitude IS NULL OR (latitude = 0 AND longitude = 0))", nil, &s.MissingLocation},
			{"SELECT COUNT(*) FROM observations WHERE status IN ('SUBMITTED','ACCEPTED','RETURNED','REJECTED') AND accuracy_m > 30", nil, &s.LowAccuracy},
			// 同图只统计最后一次尝试，成功重试后不继续计为故障。
			{"SELECT COUNT(*) FROM recognition_attempts a WHERE a.status = 'FAILED' AND NOT EXISTS (SELECT 1 FROM recognition_attempts b WHERE b.image_id = a.image_id AND b.attempt_no > a.attempt_no) AND EXISTS (SELECT 1 FROM observation_images oi WHERE oi.image_id = a.image_id)", nil, &s.FailedRecognition},
		} {
			if err := tx.Raw(query.sql, query.args...).Scan(query.target).Error; err != nil {
				return err
			}
		}
		// 已审核记录仅使用最后提交冻结的人工结论，排除误检与历史版本。
		return tx.Raw(`SELECT ci.final_class_code AS code, COUNT(*) AS total
			FROM observations o
			JOIN observation_submissions s ON s.observation_id = o.id
			JOIN conclusion_items ci ON ci.conclusion_revision_id = s.conclusion_revision_id
			WHERE o.status = 'ACCEPTED'
			AND NOT EXISTS (SELECT 1 FROM observation_submissions newer WHERE newer.observation_id = o.id AND newer.submission_no > s.submission_no)
			AND ci.decision IN ('CONFIRM','MODIFY','ADD') AND ci.final_class_code <> ''
			GROUP BY ci.final_class_code ORDER BY total DESC, code`).Scan(&s.Categories).Error
	}, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	return s, err
}
