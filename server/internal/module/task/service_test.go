package task

import (
	"context"
	"errors"
	"testing"

	"citywall/server/internal/middleware"
	"citywall/server/internal/module/audit"
)

// ---- fakes ----

type fakeRepo struct {
	tasks   map[int64]*Task
	points  map[int64][]TaskPoint
	nextID  int64
	pointID int64
	status  string // 用于模拟非法访问
	refused error
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{tasks: map[int64]*Task{}, points: map[int64][]TaskPoint{}, nextID: 1, pointID: 1}
}

func (f *fakeRepo) InsertTask(_ context.Context, t *Task) error {
	if f.refused != nil {
		return f.refused
	}
	t.ID = f.nextID
	f.nextID++
	f.tasks[t.ID] = t
	return nil
}

func (f *fakeRepo) InsertTaskPoints(_ context.Context, taskID int64, pointIDs []int64) error {
	if f.refused != nil {
		return f.refused
	}
	for i, pid := range pointIDs {
		f.points[taskID] = append(f.points[taskID], TaskPoint{
			ID:         f.pointID,
			TaskID:     taskID,
			PointID:    pid,
			SequenceNo: i + 1,
		})
		f.pointID++
	}
	return nil
}

func (f *fakeRepo) GetByID(_ context.Context, id int64) (*Task, error) {
	t, ok := f.tasks[id]
	if !ok {
		return nil, errors.New("not found")
	}
	return t, nil
}

func (f *fakeRepo) ListByActor(_ context.Context, actorID int64, isAdmin bool) ([]Task, error) {
	var out []Task
	for _, t := range f.tasks {
		if isAdmin || t.AssigneeID == actorID {
			out = append(out, *t)
		}
	}
	return out, nil
}

func (f *fakeRepo) UpdateStatus(_ context.Context, id int64, fromRevision int64, status string) (int64, error) {
	t, ok := f.tasks[id]
	if !ok || t.Revision != fromRevision {
		return 0, nil
	}
	// 模拟 SQL 语义：不修改内存 revision（由 service 自行 t.Revision++）。
	return 1, nil
}

func (f *fakeRepo) ListPoints(_ context.Context, taskID int64) ([]TaskPoint, error) {
	return f.points[taskID], nil
}

func (f *fakeRepo) FillPointSnapshots(_ context.Context, taskID int64, snaps []PointSnapshot) error {
	for i := range f.points[taskID] {
		for _, s := range snaps {
			if f.points[taskID][i].PointID == s.PointID {
				name, seg, lat, lng, r, hint := s.Name, s.SegmentID, s.Latitude, s.Longitude, s.RadiusM, s.Hint
				f.points[taskID][i].SnapshotName = &name
				f.points[taskID][i].SnapshotSegmentID = &seg
				f.points[taskID][i].SnapshotLatitude = &lat
				f.points[taskID][i].SnapshotLongitude = &lng
				f.points[taskID][i].SnapshotRadiusM = &r
				f.points[taskID][i].SnapshotLocationHint = &hint
			}
		}
	}
	return nil
}

func (f *fakeRepo) MarkInProgressIfAssigned(_ context.Context, taskID int64) error {
	t, ok := f.tasks[taskID]
	if !ok {
		return errors.New("not found")
	}
	if t.Status == StatusAssigned {
		t.Status = StatusInProgress
	}
	return nil
}

func (f *fakeRepo) GetTaskPoint(_ context.Context, id int64) (*TaskPoint, error) {
	for _, pts := range f.points {
		for _, p := range pts {
			if p.ID == id {
				return &p, nil
			}
		}
	}
	return nil, errors.New("not found")
}

type fakeObsStatus struct {
	m map[int64]string
}

func (f *fakeObsStatus) TaskPointStatuses(_ context.Context, _ int64) (map[int64]string, error) {
	return f.m, nil
}

// ---- helpers ----

func admin() *middleware.Actor { return &middleware.Actor{UserID: 1, Roles: []string{"ADMIN"}} }
func reviewer() *middleware.Actor {
	return &middleware.Actor{UserID: 2, Roles: []string{"REVIEWER"}}
}
func inspector() *middleware.Actor {
	return &middleware.Actor{UserID: 3, Roles: []string{"INSPECTOR"}}
}

func newService(f *fakeRepo, obs map[int64]string) Service {
	return NewService(f, &fakeObsStatus{m: obs}, func(_ context.Context, id int64) (PointSnapshot, error) {
		return PointSnapshot{PointID: id, Name: "P1", SegmentID: 1, Latitude: 1, Longitude: 1, RadiusM: 50, Hint: "hint"}, nil
	}, audit.NewRecorder(nil))
}

func createTask(t *testing.T, svc Service, actor *middleware.Actor) int64 {
	t.Helper()
	tk, err := svc.Create(context.Background(), actor, CreateInput{
		Title:      "t1",
		AssigneeID: 3,
		PointIDs:   []int64{11, 12},
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	return tk.ID
}

// ---- tests ----

func TestCreateRequiresAdmin(t *testing.T) {
	svc := newService(newFakeRepo(), nil)
	if _, err := svc.Create(context.Background(), inspector(), CreateInput{Title: "t", AssigneeID: 3, PointIDs: []int64{1}}); err == nil {
		t.Fatal("expected forbidden for non-admin")
	}
}

func TestCreateDuplicatePointRejected(t *testing.T) {
	svc := newService(newFakeRepo(), nil)
	_, err := svc.Create(context.Background(), admin(), CreateInput{Title: "t", AssigneeID: 3, PointIDs: []int64{1, 1}})
	if !errors.Is(err, ErrDuplicatePoint) {
		t.Fatalf("expected ErrDuplicatePoint, got %v", err)
	}
}

func TestDispatchTransitions(t *testing.T) {
	f := newFakeRepo()
	svc := newService(f, nil)
	id := createTask(t, svc, admin())

	// 巡检员无权派发。
	if _, err := svc.Dispatch(context.Background(), inspector(), id, 1); !errors.Is(err, ErrForbidden) {
		t.Fatalf("expected forbidden, got %v", err)
	}
	// revision 不匹配。
	if _, err := svc.Dispatch(context.Background(), admin(), id, 99); !errors.Is(err, ErrStatusConflict) {
		t.Fatalf("expected conflict, got %v", err)
	}
	// 正常派发。
	tk, err := svc.Dispatch(context.Background(), admin(), id, 1)
	if err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	if tk.Status != StatusAssigned {
		t.Fatalf("status = %s", tk.Status)
	}
	// 重复派发拒绝。
	if _, err := svc.Dispatch(context.Background(), admin(), id, 2); !errors.Is(err, ErrInvalidState) {
		t.Fatalf("expected invalid state, got %v", err)
	}
}

func TestSubmitTaskRequiresAllPointsSubmitted(t *testing.T) {
	f := newFakeRepo()
	svc := newService(f, map[int64]string{1: "SUBMITTED", 2: "DRAFT"})
	id := createTask(t, svc, admin())
	_, _ = svc.Dispatch(context.Background(), admin(), id, 1)
	if _, err := svc.SubmitTask(context.Background(), inspector(), id, 2); !errors.Is(err, ErrUnfinishedPoints) {
		t.Fatalf("expected unfinished points, got %v", err)
	}
}

func TestSubmitTaskAllDone(t *testing.T) {
	f := newFakeRepo()
	svc := newService(f, map[int64]string{1: "SUBMITTED", 2: "ACCEPTED"})
	id := createTask(t, svc, admin())
	_, _ = svc.Dispatch(context.Background(), admin(), id, 1)
	tk, err := svc.SubmitTask(context.Background(), inspector(), id, 2)
	if err != nil {
		t.Fatalf("submit task: %v", err)
	}
	if tk.Status != StatusPendingReview {
		t.Fatalf("status = %s", tk.Status)
	}
}

func TestCompleteRequiresAllAccepted(t *testing.T) {
	f := newFakeRepo()
	svc := newService(f, map[int64]string{1: "ACCEPTED", 2: "SUBMITTED"})
	id := createTask(t, svc, admin())
	_, _ = svc.Dispatch(context.Background(), admin(), id, 1)
	_, _ = svc.SubmitTask(context.Background(), inspector(), id, 2)
	if _, err := svc.Complete(context.Background(), reviewer(), id, 3, ""); !errors.Is(err, ErrUnfinishedPoints) {
		t.Fatalf("expected unfinished points, got %v", err)
	}
}

func TestCompleteLifecycle(t *testing.T) {
	f := newFakeRepo()
	svc := newService(f, map[int64]string{1: "ACCEPTED", 2: "ACCEPTED"})
	id := createTask(t, svc, admin())
	_, _ = svc.Dispatch(context.Background(), admin(), id, 1)
	_, _ = svc.SubmitTask(context.Background(), inspector(), id, 2)
	tk, err := svc.Complete(context.Background(), reviewer(), id, 3, "ok")
	if err != nil {
		t.Fatalf("complete: %v", err)
	}
	if tk.Status != StatusCompleted {
		t.Fatalf("status = %s", tk.Status)
	}
}
