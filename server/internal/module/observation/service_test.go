package observation

import (
	"context"
	"errors"
	"os"
	"testing"

	"citywall/server/internal/infrastructure/recognizer"
	"citywall/server/internal/middleware"
	"citywall/server/internal/module/audit"
	"citywall/server/internal/module/task"
)

// ---- fakes ----

type fakeObsRepo struct {
	obs      map[int64]*Observation
	images   map[int64]*Image
	links    map[int64][]ObservationImage
	attempts []RecognitionAttempt
	detect   []Detection
	concl    []ConclusionRevision
	subs     []ObservationSubmission
	decisions []ReviewDecision
	next     int64
}

func newFakeObsRepo() *fakeObsRepo {
	return &fakeObsRepo{
		obs:    map[int64]*Observation{},
		images: map[int64]*Image{},
		links:  map[int64][]ObservationImage{},
		next:   1,
	}
}

func (f *fakeObsRepo) Insert(_ context.Context, o *Observation) error {
	o.ID = f.next
	f.next++
	f.obs[o.ID] = o
	return nil
}
func (f *fakeObsRepo) GetByID(_ context.Context, id int64) (*Observation, error) {
	o, ok := f.obs[id]
	if !ok {
		return nil, errors.New("not found")
	}
	return o, nil
}
func (f *fakeObsRepo) GetByTaskPoint(_ context.Context, tp int64) (*Observation, error) {
	for _, o := range f.obs {
		if o.TaskPointID != nil && *o.TaskPointID == tp {
			return o, nil
		}
	}
	return nil, errors.New("not found")
}
func (f *fakeObsRepo) Update(_ context.Context, o *Observation) error {
	f.obs[o.ID] = o
	return nil
}
func (f *fakeObsRepo) BumpRevision(_ context.Context, id, from int64) (int64, error) {
	o, ok := f.obs[id]
	if !ok || o.Revision != from {
		return 0, nil
	}
	// 模拟 SQL 语义：只报告影响行数，不修改内存（由 service 自行 o.Revision++）。
	return 1, nil
}
func (f *fakeObsRepo) UpdateStatus(_ context.Context, id, from int64, status string) (int64, error) {
	o, ok := f.obs[id]
	if !ok || o.Revision != from {
		return 0, nil
	}
	return 1, nil
}
func (f *fakeObsRepo) InsertImage(_ context.Context, img *Image) error {
	img.ID = f.next
	f.next++
	f.images[img.ID] = img
	return nil
}
func (f *fakeObsRepo) GetImage(_ context.Context, id int64) (*Image, error) {
	img, ok := f.images[id]
	if !ok {
		return nil, errors.New("not found")
	}
	return img, nil
}
func (f *fakeObsRepo) LinkImage(_ context.Context, obsID, imageID int64, role string) error {
	f.links[obsID] = append(f.links[obsID], ObservationImage{ObservationID: obsID, ImageID: imageID, ImageRole: role})
	return nil
}
func (f *fakeObsRepo) ListImages(_ context.Context, obsID int64) ([]ObservationImage, error) {
	return f.links[obsID], nil
}
func (f *fakeObsRepo) NextAttemptNo(_ context.Context, _ int64) (int, error) {
	return len(f.attempts) + 1, nil
}
func (f *fakeObsRepo) InsertAttempt(_ context.Context, a *RecognitionAttempt) error {
	f.attempts = append(f.attempts, *a)
	return nil
}
func (f *fakeObsRepo) UpdateAttempt(_ context.Context, a *RecognitionAttempt) error {
	for i := range f.attempts {
		if f.attempts[i].ID == a.ID {
			f.attempts[i] = *a
		}
	}
	return nil
}
func (f *fakeObsRepo) InsertDetections(_ context.Context, ds []Detection) error {
	f.detect = append(f.detect, ds...)
	return nil
}
func (f *fakeObsRepo) ListDetections(_ context.Context, _ int64) ([]Detection, error) {
	return f.detect, nil
}
func (f *fakeObsRepo) NextConclusionNo(_ context.Context, _ int64) (int, error) {
	return len(f.concl) + 1, nil
}
func (f *fakeObsRepo) InsertConclusion(_ context.Context, rev *ConclusionRevision, items []ConclusionItem) error {
	f.concl = append(f.concl, *rev)
	return nil
}
func (f *fakeObsRepo) LatestConclusion(_ context.Context, obsID int64) (*ConclusionRevision, error) {
	for i := len(f.concl) - 1; i >= 0; i-- {
		if f.concl[i].ObservationID == obsID {
			return &f.concl[i], nil
		}
	}
	return nil, errors.New("not found")
}
func (f *fakeObsRepo) NextSubmissionNo(_ context.Context, _ int64) (int, error) {
	return len(f.subs) + 1, nil
}
func (f *fakeObsRepo) InsertSubmission(_ context.Context, s *ObservationSubmission) error {
	f.subs = append(f.subs, *s)
	return nil
}
func (f *fakeObsRepo) LatestSubmission(_ context.Context, obsID int64) (*ObservationSubmission, error) {
	for i := len(f.subs) - 1; i >= 0; i-- {
		if f.subs[i].ObservationID == obsID {
			return &f.subs[i], nil
		}
	}
	return nil, errors.New("not found")
}
func (f *fakeObsRepo) InsertReviewDecision(_ context.Context, d *ReviewDecision) error {
	f.decisions = append(f.decisions, *d)
	return nil
}
func (f *fakeObsRepo) ListByStatuses(_ context.Context, statuses []string) ([]Observation, error) {
	var out []Observation
	for _, o := range f.obs {
		for _, s := range statuses {
			if o.Status == s {
				out = append(out, *o)
			}
		}
	}
	return out, nil
}
func (f *fakeObsRepo) StatusByTaskPoints(_ context.Context, ids []int64) (map[int64]string, error) {
	m := map[int64]string{}
	for _, o := range f.obs {
		for _, id := range ids {
			if o.TaskPointID != nil && *o.TaskPointID == id {
				m[id] = o.Status
			}
		}
	}
	return m, nil
}
func (f *fakeObsRepo) ObservationByImage(_ context.Context, imageID int64) (*Observation, error) {
	for obsID, links := range f.links {
		for _, l := range links {
			if l.ImageID == imageID {
				return f.obs[obsID], nil
			}
		}
	}
	return nil, errors.New("not found")
}

type fakeStorage struct{ keys []string }

func (f *fakeStorage) SaveImage(_ []byte, mediaType string) (string, error) {
	key := "k" + itoa(len(f.keys)) + mediaType
	f.keys = append(f.keys, key)
	return key, nil
}
func (f *fakeStorage) Open(relKey string) (*os.File, error) {
	return os.CreateTemp("", "cw-test-*")
}

type fakeRecognizer struct {
	res  *recognizer.Result
	err  error
	calls int
}

func (f *fakeRecognizer) Recognize(_ context.Context, attemptID, _, _ string, _ []byte) (*recognizer.Result, error) {
	f.calls++
	if f.err != nil {
		return nil, f.err
	}
	return f.res, nil
}

type fakeTaskPoint struct {
	points map[int64]*task.TaskPoint
}

func (f *fakeTaskPoint) GetTaskPoint(_ context.Context, id int64) (*task.TaskPoint, error) {
	p, ok := f.points[id]
	if !ok {
		return nil, errors.New("not found")
	}
	return p, nil
}
func (f *fakeTaskPoint) MarkInProgressIfAssigned(_ context.Context, _ int64) error { return nil }
func (f *fakeTaskPoint) ListPoints(_ context.Context, _ int64) ([]task.TaskPoint, error) {
	var out []task.TaskPoint
	for _, p := range f.points {
		out = append(out, *p)
	}
	return out, nil
}

// ---- helpers ----

func itoa(n int) string { return string(rune('0' + n)) }

func inspector() *middleware.Actor { return &middleware.Actor{UserID: 3, Roles: []string{"INSPECTOR"}} }
func reviewer() *middleware.Actor  { return &middleware.Actor{UserID: 2, Roles: []string{"REVIEWER"}} }

func newObsService() (*service, *fakeObsRepo) {
	repo := newFakeObsRepo()
	points := map[int64]*task.TaskPoint{1: {ID: 1, TaskID: 10, PointID: 100}}
	s := &service{
		repo:            repo,
		storage:         &fakeStorage{},
		recognizer:      &fakeRecognizer{res: &recognizer.Result{ModelVersion: "yolo_baseline_001", Detections: nil}},
		taskPoints:      &fakeTaskPoint{points: points},
		taskLife:        &fakeTaskPoint{points: points},
		taskPointLister: &fakeTaskPoint{points: points},
		audit:           audit.NewRecorder(nil),
	}
	return s, repo
}

func pngBytes() []byte {
	// 1x1 PNG
	return []byte("\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR\x00\x00\x00\x01\x00\x00\x00\x01\x08\x06\x00\x00\x00\x1f\x15\xc4\x89\x00\x00\x00\rIDAT\x08\xd7c\xf8\xff\xff?\x00\x05\xfe\x02\xfe\xdc\xcc\x59\xe7\x00\x00\x00\x00IEND\xaeB`\x82")
}

// ---- tests ----

func TestCreateSourceConstraints(t *testing.T) {
	s, _ := newObsService()
	// TASK 缺 task_point_id。
	if _, err := s.Create(context.Background(), inspector(), SourceTask, nil, "loc"); !errors.Is(err, ErrSourceConstraint) {
		t.Fatalf("expected source constraint, got %v", err)
	}
	// VOLUNTARY 带 task_point_id。
	tp := int64(1)
	if _, err := s.Create(context.Background(), inspector(), SourceVoluntary, &tp, "loc"); !errors.Is(err, ErrSourceConstraint) {
		t.Fatalf("expected source constraint, got %v", err)
	}
	// VOLUNTARY 缺位置。
	if _, err := s.Create(context.Background(), inspector(), SourceVoluntary, nil, ""); !errors.Is(err, ErrMissingLocation) {
		t.Fatalf("expected missing location, got %v", err)
	}
}

func TestCreateTaskPointDuplicate(t *testing.T) {
	s, _ := newObsService()
	tp := int64(1)
	_, err := s.Create(context.Background(), inspector(), SourceTask, &tp, "loc")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := s.Create(context.Background(), inspector(), SourceTask, &tp, "loc"); !errors.Is(err, ErrDuplicateRecord) {
		t.Fatalf("expected duplicate record, got %v", err)
	}
}

func TestSubmitRequiresOverviewAndConclusion(t *testing.T) {
	s, repo := newObsService()
	tp := int64(1)
	o, err := s.Create(context.Background(), inspector(), SourceTask, &tp, "loc")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	// 无概览图。
	if _, err := s.Submit(context.Background(), inspector(), o.ID, 1); !errors.Is(err, ErrMissingOverview) {
		t.Fatalf("expected missing overview, got %v", err)
	}
	// 有概览图但无结论（AddImage 不 bump revision）。
	_, _ = s.AddImage(context.Background(), inspector(), o.ID, "image/png", pngBytes(), ImageRoleOverview)
	if _, err := s.Submit(context.Background(), inspector(), o.ID, 1); !errors.Is(err, ErrMissingConclusion) {
		t.Fatalf("expected missing conclusion, got %v", err)
	}
	// 保存结论后提交成功（SaveConclusion bump revision 1→2）。
	if _, err := s.SaveConclusion(context.Background(), inspector(), o.ID, 1, ConclusionInput{Outcome: OutcomeNormal}); err != nil {
		t.Fatalf("conclusion: %v", err)
	}
	sub, err := s.Submit(context.Background(), inspector(), o.ID, 2)
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	if sub.Status != ObsSubmitted {
		t.Fatalf("status = %s", sub.Status)
	}
	if len(repo.subs) != 1 {
		t.Fatalf("submissions = %d", len(repo.subs))
	}
}

func TestReviewRules(t *testing.T) {
	s, _ := newObsService()
	tp := int64(1)
	o, _ := s.Create(context.Background(), inspector(), SourceTask, &tp, "loc")
	_, _ = s.AddImage(context.Background(), inspector(), o.ID, "image/png", pngBytes(), ImageRoleOverview)
	_, _ = s.SaveConclusion(context.Background(), inspector(), o.ID, 1, ConclusionInput{Outcome: OutcomeNormal})
	_, _ = s.Submit(context.Background(), inspector(), o.ID, 2)

	// 巡检员无权审核。
	if _, err := s.Review(context.Background(), inspector(), o.ID, 3, ReviewInput{Decision: ReviewAccept}); !errors.Is(err, ErrForbidden) {
		t.Fatalf("expected forbidden, got %v", err)
	}
	// 自审拒绝。
	if _, err := s.Review(context.Background(), &middleware.Actor{UserID: 3, Roles: []string{"REVIEWER", "INSPECTOR"}}, o.ID, 3, ReviewInput{Decision: ReviewAccept}); !errors.Is(err, ErrSelfReview) {
		t.Fatalf("expected self review rejected, got %v", err)
	}
	// RETURN 必须理由。
	if _, err := s.Review(context.Background(), reviewer(), o.ID, 3, ReviewInput{Decision: ReviewReturn}); !errors.Is(err, ErrReturnReasonRequired) {
		t.Fatalf("expected reason required, got %v", err)
	}
	// REJECT 仅限主动上报。
	if _, err := s.Review(context.Background(), reviewer(), o.ID, 3, ReviewInput{Decision: ReviewReject, Reason: "r"}); !errors.Is(err, ErrRejectOnlyVoluntary) {
		t.Fatalf("expected reject only voluntary, got %v", err)
	}
	// 正常通过。
	accepted, err := s.Review(context.Background(), reviewer(), o.ID, 3, ReviewInput{Decision: ReviewAccept})
	if err != nil {
		t.Fatalf("review accept: %v", err)
	}
	if accepted.Status != ObsAccepted {
		t.Fatalf("status = %s", accepted.Status)
	}
}

func TestRecognizeFailureKeepsRecord(t *testing.T) {
	s, repo := newObsService()
	tp := int64(1)
	o, _ := s.Create(context.Background(), inspector(), SourceTask, &tp, "loc")
	img, _ := s.AddImage(context.Background(), inspector(), o.ID, "image/png", pngBytes(), ImageRoleOverview)
	// 模拟识别服务不可用。
	s.recognizer = &fakeRecognizer{err: recognizer.ErrTimeout}
	attempt, dets, err := s.Recognize(context.Background(), inspector(), img.ID, "req1")
	if err == nil {
		t.Fatal("expected error from recognizer")
	}
	if attempt.Status != AttemptFailed || attempt.ErrorCode != "RECOGNITION_TIMEOUT" {
		t.Fatalf("attempt status=%s code=%s", attempt.Status, attempt.ErrorCode)
	}
	if len(dets) != 0 {
		t.Fatalf("detections = %d", len(dets))
	}
	if len(repo.attempts) != 1 {
		t.Fatalf("attempts = %d", len(repo.attempts))
	}
}

func TestRecognizeSuccessWithDetections(t *testing.T) {
	s, _ := newObsService()
	tp := int64(1)
	o, _ := s.Create(context.Background(), inspector(), SourceTask, &tp, "loc")
	img, _ := s.AddImage(context.Background(), inspector(), o.ID, "image/png", pngBytes(), ImageRoleOverview)
	s.recognizer = &fakeRecognizer{res: &recognizer.Result{
		ModelVersion: "yolo_baseline_001",
		Detections: []recognizer.Detection{
			{DetectionID: "det_1", ClassCode: "CRACK", Confidence: 0.82, BBox: recognizer.BBox{X1: 1, Y1: 2, X2: 30, Y2: 40}},
		},
	}}
	attempt, dets, err := s.Recognize(context.Background(), inspector(), img.ID, "req2")
	if err != nil {
		t.Fatalf("recognize: %v", err)
	}
	if attempt.Status != AttemptSucceeded {
		t.Fatalf("status = %s", attempt.Status)
	}
	if len(dets) != 1 || dets[0].ClassCode != "CRACK" {
		t.Fatalf("detections = %+v", dets)
	}
}

func TestGetImageContentForbiddenForOthers(t *testing.T) {
	s, _ := newObsService()
	tp := int64(1)
	o, _ := s.Create(context.Background(), inspector(), SourceTask, &tp, "loc")
	img, _ := s.AddImage(context.Background(), inspector(), o.ID, "image/png", pngBytes(), ImageRoleOverview)
	// 其他巡检员无权访问。
	other := &middleware.Actor{UserID: 99, Roles: []string{"INSPECTOR"}}
	if _, _, err := s.GetImageContent(context.Background(), other, img.ID); !errors.Is(err, ErrForbidden) {
		t.Fatalf("expected forbidden, got %v", err)
	}
	// 审核员可访问。
	_, rc, err := s.GetImageContent(context.Background(), reviewer(), img.ID)
	if err != nil {
		t.Fatalf("reviewer access: %v", err)
	}
	if rc != nil {
		rc.Close()
	}
}

func TestListImages(t *testing.T) {
	s, _ := newObsService()
	tp := int64(1)
	o, _ := s.Create(context.Background(), inspector(), SourceTask, &tp, "loc")
	if _, err := s.AddImage(context.Background(), inspector(), o.ID, "image/png", pngBytes(), ImageRoleOverview); err != nil {
		t.Fatalf("add overview: %v", err)
	}
	if _, err := s.AddImage(context.Background(), inspector(), o.ID, "image/png", pngBytes(), ImageRoleDetail); err != nil {
		t.Fatalf("add detail: %v", err)
	}

	imgs, err := s.ListImages(context.Background(), inspector(), o.ID)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(imgs) != 2 {
		t.Fatalf("len = %d, want 2", len(imgs))
	}
	roles := map[string]int{}
	for _, m := range imgs {
		roles[m.Role]++
		if m.ID == 0 || m.URL == "" {
			t.Fatalf("meta missing id/url: %+v", m)
		}
	}
	if roles[ImageRoleOverview] != 1 || roles[ImageRoleDetail] != 1 {
		t.Fatalf("roles = %v", roles)
	}

	// 越权：其他巡检员 403。
	other := &middleware.Actor{UserID: 99, Roles: []string{"INSPECTOR"}}
	if _, err := s.ListImages(context.Background(), other, o.ID); !errors.Is(err, ErrForbidden) {
		t.Fatalf("expected forbidden for other, got %v", err)
	}
	// 审核员可看。
	if _, err := s.ListImages(context.Background(), reviewer(), o.ID); err != nil {
		t.Fatalf("reviewer list: %v", err)
	}
	// 404：不存在记录。
	if _, err := s.ListImages(context.Background(), reviewer(), 99999); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected not found, got %v", err)
	}
}
