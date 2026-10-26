package issue

import (
	"context"
	"errors"
	"testing"
	"time"

	"citywall/server/internal/middleware"
	"citywall/server/internal/module/audit"
	"citywall/server/internal/module/observation"
)

// ---- fake repository ----

type fakeIssueRepo struct {
	issues     map[int64]*Issue
	nextID     int64
	obsLinks   []IssueObservation
	actions    []IssueAction
	actionImgs []IssueActionImage
}

func newFakeRepo() *fakeIssueRepo {
	return &fakeIssueRepo{issues: map[int64]*Issue{}, nextID: 1}
}

func (f *fakeIssueRepo) Insert(_ context.Context, i *Issue) error {
	i.ID = f.nextID
	f.nextID++
	f.issues[i.ID] = i
	return nil
}

func (f *fakeIssueRepo) GetByID(_ context.Context, id int64) (*Issue, error) {
	i, ok := f.issues[id]
	if !ok {
		return nil, errors.New("not found")
	}
	cp := *i
	return &cp, nil
}

func (f *fakeIssueRepo) ListPage(_ context.Context, actorID int64, isAdmin bool, status string, limit, offset int) ([]Issue, error) {
	var out []Issue
	for _, i := range f.issues {
		if !isAdmin && (i.AssigneeID == nil || *i.AssigneeID != actorID) {
			continue
		}
		if status != "" && i.Status != status {
			continue
		}
		out = append(out, *i)
	}
	return out, nil
}

func (f *fakeIssueRepo) CountIssues(_ context.Context, actorID int64, isAdmin bool, status string) (int64, error) {
	items, _ := f.ListPage(context.Background(), actorID, isAdmin, status, 0, 0)
	return int64(len(items)), nil
}

func (f *fakeIssueRepo) UpdateStatus(_ context.Context, id, fromRevision int64, status string) (int64, error) {
	i, ok := f.issues[id]
	if !ok {
		return 0, errors.New("not found")
	}
	if i.Revision != fromRevision {
		return 0, nil
	}
	i.Status = status
	if status == StatusOpen {
		i.ClosedAt = nil
		i.CloseReason = ""
	}
	i.Revision++
	return 1, nil
}

func (f *fakeIssueRepo) UpdateAssignee(_ context.Context, id, fromRevision int64, assigneeID int64) (int64, error) {
	i, ok := f.issues[id]
	if !ok {
		return 0, errors.New("not found")
	}
	if i.Revision != fromRevision {
		return 0, nil
	}
	i.AssigneeID = &assigneeID
	i.Revision++
	return 1, nil
}

func (f *fakeIssueRepo) Close(ctx context.Context, id, fromRevision int64, reason string) (int64, error) {
	n, err := f.UpdateStatus(ctx, id, fromRevision, StatusClosed)
	if err == nil && n == 1 {
		now := time.Now().UTC()
		f.issues[id].ClosedAt = &now
		f.issues[id].CloseReason = reason
	}
	return n, err
}

func (f *fakeIssueRepo) InsertAction(_ context.Context, a *IssueAction) error {
	a.ID = f.nextID
	f.nextID++
	f.actions = append(f.actions, *a)
	return nil
}

func (f *fakeIssueRepo) ListActions(_ context.Context, issueID int64) ([]IssueAction, error) {
	var out []IssueAction
	for _, a := range f.actions {
		if a.IssueID == issueID {
			out = append(out, a)
		}
	}
	return out, nil
}

func (f *fakeIssueRepo) LinkObservation(_ context.Context, link *IssueObservation) error {
	f.obsLinks = append(f.obsLinks, *link)
	return nil
}

func (f *fakeIssueRepo) ListObservations(_ context.Context, issueID int64) ([]IssueObservation, error) {
	var out []IssueObservation
	for _, l := range f.obsLinks {
		if l.IssueID == issueID {
			out = append(out, l)
		}
	}
	return out, nil
}

func (f *fakeIssueRepo) ListByObservation(_ context.Context, observationID int64) ([]IssueObservation, error) {
	var out []IssueObservation
	for _, l := range f.obsLinks {
		if l.ObservationID == observationID {
			out = append(out, l)
		}
	}
	return out, nil
}

func (f *fakeIssueRepo) LinkActionImage(_ context.Context, actionID, imageID int64) error {
	f.actionImgs = append(f.actionImgs, IssueActionImage{IssueActionID: actionID, ImageID: imageID})
	return nil
}

func (f *fakeIssueRepo) InTx(_ context.Context, fn func(Repository) error) error {
	return fn(f)
}

// ---- fake obs access ----

type fakeObsAccess struct{}

func (fakeObsAccess) AcceptedAdmissionInfo(_ context.Context, obsID int64) (*observation.AcceptedInfo, error) {
	if obsID != 100 && obsID != 101 {
		return nil, observation.ErrNotFound
	}
	return &observation.AcceptedInfo{
		Source:           observation.SourceVoluntary,
		LocationText:     "南门段东侧",
		ReviewDecisionID: 7,
	}, nil
}

// ---- fake image store ----

type fakeImageStore struct{}

func (fakeImageStore) Save(_ []byte, _ string) (string, string, error) { return "k1", "sha1", nil }
func (fakeImageStore) Register(_ context.Context, key, sha, mt string, bs int64, w, h int, up int64) (int64, error) {
	return 1000, nil
}

// ---- helpers ----

func newTestSvc() (Service, *fakeIssueRepo) {
	repo := newFakeRepo()
	svc := NewService(repo, fakeObsAccess{}, fakeImageStore{}, audit.NewRecorder(nil))
	return svc, repo
}

func mkIssue(repo *fakeIssueRepo) int64 {
	i := &Issue{Title: "裂缝", LocationText: "点位1", PriorityCode: PriorityWatch, Status: StatusUnassigned, Revision: 1, OpenedAt: time.Now()}
	_ = repo.Insert(context.Background(), i)
	return i.ID
}

func admin() *middleware.Actor    { return &middleware.Actor{UserID: 1, Roles: []string{"ADMIN"}} }
func reviewer() *middleware.Actor { return &middleware.Actor{UserID: 2, Roles: []string{"REVIEWER"}} }
func inspector(id int64) *middleware.Actor {
	return &middleware.Actor{UserID: id, Roles: []string{"INSPECTOR"}}
}

// ---- tests ----

func TestAssignFlow(t *testing.T) {
	svc, repo := newTestSvc()
	id := mkIssue(repo)
	if _, err := svc.Assign(context.Background(), inspector(3), id, 1, 9, "指定责任人"); err != ErrForbidden {
		t.Fatalf("inspector assign should be forbidden, got %v", err)
	}
	iss, err := svc.Assign(context.Background(), admin(), id, 1, 9, "指定责任人")
	if err != nil {
		t.Fatalf("assign failed: %v", err)
	}
	if iss.Status != StatusOpen || iss.Revision != 3 {
		t.Fatalf("expected OPEN rev3, got %s rev%d", iss.Status, iss.Revision)
	}
	// 改派：保持 OPEN（revision 3 → 4）。
	iss2, err := svc.Assign(context.Background(), admin(), id, 3, 10, "改派")
	if err != nil {
		t.Fatalf("reassign failed: %v", err)
	}
	if iss2.Status != StatusOpen || iss2.Revision != 4 {
		t.Fatalf("expected OPEN rev4, got %s rev%d", iss2.Status, iss2.Revision)
	}
	// revision 冲突。
	if _, err := svc.Assign(context.Background(), admin(), id, 1, 11, ""); !errors.Is(err, ErrStatusConflict) {
		t.Fatalf("expected conflict, got %v", err)
	}
}

func TestActionStateTransitions(t *testing.T) {
	svc, repo := newTestSvc()
	id := mkIssue(repo)
	_, _ = svc.Assign(context.Background(), admin(), id, 1, 9, "")
	// OPEN + PROGRESS → IN_PROGRESS。
	iss, err := svc.AddAction(context.Background(), inspector(9), id, 3, ActionInput{ActionType: ActionProgress, Content: "已加固"})
	if err != nil {
		t.Fatalf("progress failed: %v", err)
	}
	if iss.Status != StatusInProgress {
		t.Fatalf("expected IN_PROGRESS, got %s", iss.Status)
	}
	// IN_PROGRESS + OBSERVE → MONITORING。
	iss, err = svc.AddAction(context.Background(), inspector(9), id, 4, ActionInput{ActionType: ActionObserve, Content: "持续观察"})
	if err != nil {
		t.Fatalf("observe failed: %v", err)
	}
	if iss.Status != StatusMonitoring {
		t.Fatalf("expected MONITORING, got %s", iss.Status)
	}
	// MONITORING + PROGRESS → IN_PROGRESS。
	iss, err = svc.AddAction(context.Background(), inspector(9), id, 5, ActionInput{ActionType: ActionProgress, Content: "发现新扩展"})
	if err != nil {
		t.Fatalf("progress2 failed: %v", err)
	}
	if iss.Status != StatusInProgress {
		t.Fatalf("expected IN_PROGRESS, got %s", iss.Status)
	}
	// 非责任人不能操作。
	if _, err := svc.AddAction(context.Background(), inspector(99), id, 6, ActionInput{ActionType: ActionProgress, Content: "x"}); !errors.Is(err, ErrForbidden) {
		t.Fatalf("expected forbidden, got %v", err)
	}
	// CLOSED 后不能加动作（验证：先核验关闭再试）——见 TestVerifyFlow。
}

func TestVerifyFlow(t *testing.T) {
	svc, repo := newTestSvc()
	id := mkIssue(repo)
	_, _ = svc.Assign(context.Background(), admin(), id, 1, 9, "")
	// 审核员兼任责任人：先处理并申请核验，然后自核验应被拒。
	_, _ = svc.AddAction(context.Background(), reviewer(), id, 3, ActionInput{ActionType: ActionProgress, Content: "处理完成"})
	_, _ = svc.RequestVerification(context.Background(), reviewer(), id, 4, "请核验：已修复")
	if _, err := svc.Verify(context.Background(), reviewer(), id, 5, "PASS", ""); !errors.Is(err, ErrSelfVerify) {
		t.Fatalf("expected self verify error, got %v", err)
	}
	// 另一位审核员（管理员）核验通过 → CLOSED。
	iss, err := svc.Verify(context.Background(), admin(), id, 5, "PASS", "核验通过")
	if err != nil {
		t.Fatalf("verify pass failed: %v", err)
	}
	if iss.Status != StatusClosed || iss.ClosedAt == nil {
		t.Fatalf("expected CLOSED with closed_at, got %s", iss.Status)
	}
	// 已关闭不可再核验。
	if _, err := svc.Verify(context.Background(), reviewer(), id, 6, "RETURN", "x"); !errors.Is(err, ErrInvalidState) {
		t.Fatalf("expected invalid state, got %v", err)
	}
}

func TestVerifyReturnAndReopen(t *testing.T) {
	svc, repo := newTestSvc()
	id := mkIssue(repo)
	_, _ = svc.Assign(context.Background(), admin(), id, 1, 9, "")
	_, _ = svc.AddAction(context.Background(), inspector(9), id, 3, ActionInput{ActionType: ActionProgress, Content: "尝试处理"})
	_, _ = svc.RequestVerification(context.Background(), inspector(9), id, 4, "申请核验")
	// 退回：RETURN 必须带理由。
	if _, err := svc.Verify(context.Background(), reviewer(), id, 5, "RETURN", ""); !errors.Is(err, ErrReasonRequired) {
		t.Fatalf("expected reason required, got %v", err)
	}
	iss, err := svc.Verify(context.Background(), reviewer(), id, 5, "RETURN", "证据不足，请补充照片")
	if err != nil {
		t.Fatalf("verify return failed: %v", err)
	}
	if iss.Status != StatusInProgress {
		t.Fatalf("expected IN_PROGRESS after return, got %s", iss.Status)
	}
	// 再次申请核验并关闭。
	_, _ = svc.AddAction(context.Background(), inspector(9), id, 6, ActionInput{ActionType: ActionProgress, Content: "补充照片后完成"})
	_, _ = svc.RequestVerification(context.Background(), inspector(9), id, 7, "再次申请")
	_, err = svc.Verify(context.Background(), reviewer(), id, 8, "PASS", "证据充分，关闭")
	if err != nil {
		t.Fatalf("second verify failed: %v", err)
	}
	// 重开。
	iss, err = svc.Reopen(context.Background(), reviewer(), id, 9, "复查发现裂缝扩展")
	if err != nil {
		t.Fatalf("reopen failed: %v", err)
	}
	if iss.Status != StatusOpen {
		t.Fatalf("expected OPEN after reopen, got %s", iss.Status)
	}
}

func TestAdmitFromObservation(t *testing.T) {
	svc, _ := newTestSvc()
	// 不存在的记录。
	if _, err := svc.AdmitFromObservation(context.Background(), reviewer(), 999, AdmitInput{NewIssues: []NewIssueInput{{Title: "x", PriorityCode: PriorityWatch}}}); !errors.Is(err, observation.ErrNotFound) {
		t.Fatalf("expected not found, got %v", err)
	}
	// 受理：新建 + 关联。
	res, err := svc.AdmitFromObservation(context.Background(), reviewer(), 100, AdmitInput{
		NewIssues: []NewIssueInput{
			{Title: "南门段裂缝", LocationText: "南门段东侧", PriorityCode: PriorityProcess, PriorityReason: "现场确认"},
		},
		LinkIssueIDs:  []int64{},
		NoIssueReason: "",
	})
	if err != nil {
		t.Fatalf("admit failed: %v", err)
	}
	if len(res.CreatedIssueIDs) != 1 {
		t.Fatalf("expected 1 created issue, got %d", len(res.CreatedIssueIDs))
	}
	if res.LinkedIssueIDs == nil {
		t.Fatal("empty linked issue list must serialize as an array")
	}
	// 再次受理 → 幂等返回已有关联。
	res2, err := svc.AdmitFromObservation(context.Background(), reviewer(), 100, AdmitInput{
		NewIssues: []NewIssueInput{{Title: "重复", PriorityCode: PriorityWatch}},
	})
	if err != nil {
		t.Fatalf("second admit failed: %v", err)
	}
	if !res2.AlreadyAdmitted || len(res2.LinkedIssueIDs) != 1 {
		t.Fatalf("expected already admitted, got %+v", res2)
	}
	if res2.CreatedIssueIDs == nil {
		t.Fatal("empty created issue list must serialize as an array")
	}
	// 空受理被拒（未受理的记录）。
	if _, err := svc.AdmitFromObservation(context.Background(), reviewer(), 101, AdmitInput{}); !errors.Is(err, ErrAdmitEmpty) {
		t.Fatalf("expected admit empty error, got %v", err)
	}
	// 巡检员不能受理。
	if _, err := svc.AdmitFromObservation(context.Background(), inspector(3), 100, AdmitInput{NewIssues: []NewIssueInput{{Title: "x", PriorityCode: PriorityWatch}}}); !errors.Is(err, ErrForbidden) {
		t.Fatalf("expected forbidden, got %v", err)
	}
}

func TestAddActionImage(t *testing.T) {
	svc, repo := newTestSvc()
	id := mkIssue(repo)
	_, _ = svc.Assign(context.Background(), admin(), id, 1, 9, "")
	_, _ = svc.AddAction(context.Background(), inspector(9), id, 3, ActionInput{ActionType: ActionProgress, Content: "处理进展"})
	actions, _ := repo.ListActions(context.Background(), id)
	if len(actions) == 0 {
		t.Fatal("expected actions")
	}
	img, err := svc.AddActionImage(context.Background(), inspector(9), id, actions[0].ID, []byte("fake-png"), "image/png")
	if err != nil {
		t.Fatalf("add action image failed: %v", err)
	}
	if img.ID != 1000 || img.URL == "" {
		t.Fatalf("unexpected image: %+v", img)
	}
	// 不存在的动作。
	if _, err := svc.AddActionImage(context.Background(), inspector(9), id, 99999, []byte("x"), "image/png"); !errors.Is(err, ErrActionImageMiss) {
		t.Fatalf("expected action miss, got %v", err)
	}
}
