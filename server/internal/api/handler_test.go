package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"citywall/server/internal/api"
	"citywall/server/internal/authn"
	"citywall/server/internal/middleware"
	"citywall/server/internal/module/identity"
	"citywall/server/internal/module/observation"
	"citywall/server/internal/module/site"
	"citywall/server/internal/module/task"
	"citywall/server/internal/router"
)

// ---- fakes ----

func strp(s string) *string { return &s }

type fakeIdentity struct{}

func (fakeIdentity) Login(_ context.Context, username, _ string) (*identity.Session, error) {
	if username == "bad" {
		return nil, identity.ErrInvalidCredentials
	}
	return &identity.Session{
		Token: "stub-token",
		User:  identity.User{ID: 3, Username: strp(username), DisplayName: "巡检员"},
		Roles: []string{"INSPECTOR"},
	}, nil
}
func (fakeIdentity) WechatLogin(_ context.Context, code string) (*identity.Session, error) {
	if code == "bad" {
		return nil, identity.ErrWechatCodeInvalid
	}
	return &identity.Session{
		Token: "stub-token",
		User:  identity.User{ID: 3, Username: strp("inspector"), DisplayName: "巡检员"},
		Roles: []string{"INSPECTOR"},
	}, nil
}
func (fakeIdentity) GetUser(_ context.Context, id int64) (*identity.User, []string, error) {
	return &identity.User{ID: id, Username: strp("inspector"), DisplayName: "巡检员"}, []string{"INSPECTOR"}, nil
}
func (fakeIdentity) HashPassword(password string) (string, error) { return password, nil }

type fakeSite struct{}

func (fakeSite) ListPoints(_ context.Context, _ bool) ([]site.InspectionPoint, error) {
	return []site.InspectionPoint{{ID: 100, Code: "P-EAST-01", Name: "东城墙 1 号点", Latitude: 28.8, Longitude: 121.1}}, nil
}
func (fakeSite) ListSegments(_ context.Context, _ bool) ([]site.WallSegment, error) {
	return []site.WallSegment{{ID: 1, Code: "SEG_EAST", Name: "东城墙段"}}, nil
}
func (fakeSite) CreatePoint(_ context.Context, in site.CreatePointInput) (*site.InspectionPoint, error) {
	if in.Code == "" {
		return nil, errors.New("validation failed")
	}
	return &site.InspectionPoint{ID: 200, Code: in.Code, Name: in.Name}, nil
}

type fakeTask struct{ failWith error }

func hasRoleTest(a *middleware.Actor, role string) bool {
	for _, r := range a.Roles {
		if r == role {
			return true
		}
	}
	return false
}

func (f *fakeTask) Create(_ context.Context, actor *middleware.Actor, in task.CreateInput) (*task.Task, error) {
	if f.failWith != nil {
		return nil, f.failWith
	}
	if !hasRoleTest(actor, "ADMIN") {
		return nil, task.ErrForbidden
	}
	return &task.Task{ID: 10, Title: in.Title, AssigneeID: in.AssigneeID, Status: task.StatusDraft, Revision: 1}, nil
}
func (f *fakeTask) Dispatch(_ context.Context, _ *middleware.Actor, id, _ int64) (*task.Task, error) {
	if f.failWith != nil {
		return nil, f.failWith
	}
	return &task.Task{ID: id, Status: task.StatusAssigned, Revision: 2}, nil
}
func (f *fakeTask) List(_ context.Context, _ *middleware.Actor, _ task.ListInput) (*task.ListResult, error) {
	return &task.ListResult{Tasks: []task.Task{{ID: 10, Title: "t1", Status: task.StatusAssigned}}, Total: 1}, nil
}
func (f *fakeTask) Get(_ context.Context, _ *middleware.Actor, id int64) (*task.Task, []task.TaskPoint, error) {
	return &task.Task{ID: id, Title: "t1", Status: task.StatusAssigned}, []task.TaskPoint{{ID: 1, TaskID: id, PointID: 100}}, nil
}
func (f *fakeTask) SubmitTask(_ context.Context, _ *middleware.Actor, id, _ int64) (*task.Task, error) {
	return &task.Task{ID: id, Status: task.StatusPendingReview, Revision: 3}, nil
}
func (f *fakeTask) Complete(_ context.Context, _ *middleware.Actor, id, _ int64, _ string) (*task.Task, error) {
	return &task.Task{ID: id, Status: task.StatusCompleted, Revision: 4}, nil
}
func (f *fakeTask) Cancel(_ context.Context, _ *middleware.Actor, id, _ int64, _ string) (*task.Task, error) {
	return &task.Task{ID: id, Status: task.StatusCancelled, Revision: 5}, nil
}

type fakeObs struct{ failWith error }

func (f *fakeObs) Create(_ context.Context, _ *middleware.Actor, source string, tp *int64, _ string) (*observation.Observation, error) {
	if f.failWith != nil {
		return nil, f.failWith
	}
	return &observation.Observation{ID: 50, Source: source, TaskPointID: tp, Status: observation.ObsDraft, Revision: 1}, nil
}
func (f *fakeObs) Update(_ context.Context, _ *middleware.Actor, id, _ int64, _ observation.PatchInput) (*observation.Observation, error) {
	return &observation.Observation{ID: id, Status: observation.ObsDraft, Revision: 2}, nil
}
func (f *fakeObs) AddImage(_ context.Context, _ *middleware.Actor, _ int64, mediaType string, _ []byte, _ string, _ string) (*observation.Image, error) {
	return &observation.Image{ID: 60, MediaType: mediaType, ByteSize: 10}, nil
}
func (f *fakeObs) GetImageContent(_ context.Context, _ *middleware.Actor, _ int64) (*observation.Image, io.ReadCloser, error) {
	return &observation.Image{ID: 60, MediaType: "image/png", ByteSize: 4}, io.NopCloser(bytes.NewReader([]byte("png!"))), nil
}
func (f *fakeObs) Recognize(_ context.Context, _ *middleware.Actor, _ int64, _ string) (*observation.RecognitionAttempt, []observation.Detection, error) {
	return &observation.RecognitionAttempt{ID: 70, Status: observation.AttemptSucceeded}, []observation.Detection{{ID: 1, ClassCode: "CRACK"}}, nil
}
func (f *fakeObs) SaveConclusion(_ context.Context, _ *middleware.Actor, _ int64, _ int64, _ observation.ConclusionInput) (*observation.ConclusionRevision, error) {
	return &observation.ConclusionRevision{ID: 80, RevisionNo: 1, Outcome: observation.OutcomeNormal}, nil
}
func (f *fakeObs) Submit(_ context.Context, _ *middleware.Actor, id, _ int64) (*observation.Observation, error) {
	return &observation.Observation{ID: id, Status: observation.ObsSubmitted, Revision: 3}, nil
}
func (f *fakeObs) Review(_ context.Context, _ *middleware.Actor, id, _ int64, in observation.ReviewInput) (*observation.Observation, error) {
	if f.failWith != nil {
		return nil, f.failWith
	}
	return &observation.Observation{ID: id, Status: observation.ObsAccepted, Revision: 4}, nil
}
func (f *fakeObs) ListPendingReview(_ context.Context, _ *middleware.Actor) ([]observation.Observation, error) {
	return []observation.Observation{{ID: 50, Status: observation.ObsSubmitted}}, nil
}
func (f *fakeObs) ListImages(_ context.Context, _ *middleware.Actor, obsID int64) ([]observation.ImageMeta, error) {
	return []observation.ImageMeta{{ID: 60, Role: observation.ImageRoleOverview, URL: "/api/v1/images/60/content"}}, nil
}
func (f *fakeObs) List(_ context.Context, _ *middleware.Actor, _ observation.ListInput) (*observation.ListResult, error) {
	return &observation.ListResult{Items: []observation.Observation{{ID: 50, Status: observation.ObsSubmitted}}, Total: 1}, nil
}
func (f *fakeObs) TaskPointStatuses(_ context.Context, _ int64) (map[int64]string, error) {
	return map[int64]string{1: observation.ObsAccepted}, nil
}

// ---- helpers ----

func newTestServer(failTask, failObs error) (*httptest.Server, *authn.TokenManager) {
	gin.SetMode(gin.TestMode)
	tm := authn.NewTokenManager("test-secret", time.Hour)
	h := router.Handlers{
		Identity:    api.NewIdentityHandler(fakeIdentity{}),
		Site:        api.NewSiteHandler(fakeSite{}),
		Task:        api.NewTaskHandler(&fakeTask{failWith: failTask}),
		Observation: api.NewObservationHandler(&fakeObs{failWith: failObs}, "Idempotency-Key"),
	}
	engine := router.Router(h, tm, "X-Request-ID")
	srv := httptest.NewServer(engine)
	return srv, tm
}

func doJSON(t *testing.T, method, url string, token string, body any) (*http.Response, map[string]any) {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			t.Fatalf("encode: %v", err)
		}
	}
	req, _ := http.NewRequest(method, url, &buf)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do: %v", err)
	}
	defer resp.Body.Close()
	var out map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&out)
	return resp, out
}

// ---- tests ----

func TestHealthz(t *testing.T) {
	srv, _ := newTestServer(nil, nil)
	defer srv.Close()
	resp, _ := doJSON(t, http.MethodGet, srv.URL+"/healthz", "", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("healthz = %d", resp.StatusCode)
	}
}

func TestLogin(t *testing.T) {
	srv, _ := newTestServer(nil, nil)
	defer srv.Close()
	resp, body := doJSON(t, http.MethodPost, srv.URL+"/api/v1/auth/login", "", map[string]string{"username": "inspector", "password": "x"})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("login = %d: %v", resp.StatusCode, body)
	}
	if body["data"].(map[string]any)["token"] != "stub-token" {
		t.Fatalf("token missing: %v", body)
	}
}

func TestLoginInvalidCredentials(t *testing.T) {
	srv, _ := newTestServer(nil, nil)
	defer srv.Close()
	resp, body := doJSON(t, http.MethodPost, srv.URL+"/api/v1/auth/login", "", map[string]string{"username": "bad", "password": "x"})
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("login bad = %d: %v", resp.StatusCode, body)
	}
}

func TestAuthRequired(t *testing.T) {
	srv, tm := newTestServer(nil, nil)
	defer srv.Close()
	// 无 token → 401。
	resp, body := doJSON(t, http.MethodGet, srv.URL+"/api/v1/me", "", nil)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("me without token = %d", resp.StatusCode)
	}
	if body["error"].(map[string]any)["code"] != "UNAUTHORIZED" {
		t.Fatalf("error code = %v", body)
	}
	// 有效 token → 200。
	tok, _ := tm.Sign(3, []string{"INSPECTOR"})
	resp2, _ := doJSON(t, http.MethodGet, srv.URL+"/api/v1/me", tok, nil)
	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("me with token = %d", resp2.StatusCode)
	}
}

func TestTaskCreatePermissionAndSuccess(t *testing.T) {
	srv, tm := newTestServer(nil, nil)
	defer srv.Close()
	// 巡检员创建被拒绝（handler 层角色检查）。
	tokInsp, _ := tm.Sign(3, []string{"INSPECTOR"})
	resp, body := doJSON(t, http.MethodPost, srv.URL+"/api/v1/tasks", tokInsp, map[string]any{
		"title": "t", "assignee_id": 3, "point_ids": []int64{1},
	})
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("inspector create = %d: %v", resp.StatusCode, body)
	}
	// 管理员创建 → 201。
	tokAdmin, _ := tm.Sign(1, []string{"ADMIN"})
	resp2, body2 := doJSON(t, http.MethodPost, srv.URL+"/api/v1/tasks", tokAdmin, map[string]any{
		"title": "t", "assignee_id": 3, "point_ids": []int64{1},
	})
	if resp2.StatusCode != http.StatusCreated {
		t.Fatalf("admin create = %d: %v", resp2.StatusCode, body2)
	}
	// 缺字段 → 400。
	resp3, _ := doJSON(t, http.MethodPost, srv.URL+"/api/v1/tasks", tokAdmin, map[string]any{"title": "t"})
	if resp3.StatusCode != http.StatusBadRequest {
		t.Fatalf("invalid create = %d", resp3.StatusCode)
	}
}

func TestDomainErrorMapping(t *testing.T) {
	srv, tm := newTestServer(task.ErrForbidden, observation.ErrSourceConstraint)
	defer srv.Close()
	tok, _ := tm.Sign(1, []string{"ADMIN"})
	// task 模块权限错误 → 403。
	resp, body := doJSON(t, http.MethodPost, srv.URL+"/api/v1/tasks", tok, map[string]any{
		"title": "t", "assignee_id": 3, "point_ids": []int64{1},
	})
	if resp.StatusCode != http.StatusForbidden || body["error"].(map[string]any)["code"] != "FORBIDDEN" {
		t.Fatalf("task forbidden map = %d %v", resp.StatusCode, body)
	}
	// observation 来源约束 → 400。
	resp2, body2 := doJSON(t, http.MethodPost, srv.URL+"/api/v1/observations", tok, map[string]any{
		"source": "TASK", "location_text": "loc",
	})
	if resp2.StatusCode != http.StatusBadRequest || body2["error"].(map[string]any)["code"] != "VALIDATION_ERROR" {
		t.Fatalf("obs constraint map = %d %v", resp2.StatusCode, body2)
	}
}

func TestVerticalChainEndpoints(t *testing.T) {
	srv, tm := newTestServer(nil, nil)
	defer srv.Close()
	tok, _ := tm.Sign(3, []string{"INSPECTOR"})

	// 点位列表。
	resp, _ := doJSON(t, http.MethodGet, srv.URL+"/api/v1/inspection-points", tok, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("points = %d", resp.StatusCode)
	}
	// 任务列表。
	resp2, body2 := doJSON(t, http.MethodGet, srv.URL+"/api/v1/tasks", tok, nil)
	if resp2.StatusCode != http.StatusOK || len(body2["data"].(map[string]any)["tasks"].([]any)) == 0 {
		t.Fatalf("tasks = %d %v", resp2.StatusCode, body2)
	}
	// 创建记录 → 201。
	resp3, _ := doJSON(t, http.MethodPost, srv.URL+"/api/v1/observations", tok, map[string]any{
		"source": "VOLUNTARY", "location_text": "南门城楼下",
	})
	if resp3.StatusCode != http.StatusCreated {
		t.Fatalf("create obs = %d", resp3.StatusCode)
	}
	// 上传图片（multipart/form-data）。
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	part, _ := mw.CreateFormFile("image", "p.png")
	_, _ = part.Write([]byte("\x89PNG\r\n\x1a\nfake"))
	_ = mw.WriteField("image_role", "OVERVIEW")
	_ = mw.Close()
	req, _ := http.NewRequest(http.MethodPost, srv.URL+"/api/v1/observations/50/images", &buf)
	req.Header.Set("Authorization", "Bearer "+tok)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	resp4, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("upload: %v", err)
	}
	defer resp4.Body.Close()
	if resp4.StatusCode != http.StatusCreated {
		t.Fatalf("upload = %d", resp4.StatusCode)
	}
	// 提交任务。
	resp5, _ := doJSON(t, http.MethodPost, srv.URL+"/api/v1/tasks/10/submit", tok, map[string]any{"revision": 2})
	if resp5.StatusCode != http.StatusOK {
		t.Fatalf("submit task = %d", resp5.StatusCode)
	}
	// 审核通过（审核员）。
	tokReviewer, _ := tm.Sign(2, []string{"REVIEWER"})
	resp6, body6 := doJSON(t, http.MethodPost, srv.URL+"/api/v1/observations/50/review", tokReviewer, map[string]any{
		"revision": 3, "decision": "ACCEPT",
	})
	if resp6.StatusCode != http.StatusOK {
		t.Fatalf("review = %d %v", resp6.StatusCode, body6)
	}
	// 完成任务。
	tokAdmin, _ := tm.Sign(1, []string{"ADMIN"})
	resp7, _ := doJSON(t, http.MethodPost, srv.URL+"/api/v1/tasks/10/complete", tokAdmin, map[string]any{"revision": 3, "final_note": "ok"})
	if resp7.StatusCode != http.StatusOK {
		t.Fatalf("complete = %d", resp7.StatusCode)
	}
}
