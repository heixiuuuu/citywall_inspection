package audit

import (
	"context"
	"errors"
	"testing"
)

type fakeRepo struct {
	events []Event
	err    error
}

func (f *fakeRepo) Insert(_ context.Context, e *Event) error { return nil }

func (f *fakeRepo) ListEvents(_ context.Context, fl EventFilter, limit, offset int) ([]Event, int64, error) {
	if f.err != nil {
		return nil, 0, f.err
	}
	var out []Event
	for _, e := range f.events {
		if fl.EntityType != "" && e.EntityType != fl.EntityType {
			continue
		}
		if fl.EntityID != nil && e.EntityID != *fl.EntityID {
			continue
		}
		if fl.ActorID != nil && e.ActorID != *fl.ActorID {
			continue
		}
		out = append(out, e)
	}
	// 与 GORM 实现一致：id 倒序。
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	start := offset
	if start > len(out) {
		start = len(out)
	}
	end := start + limit
	if end > len(out) {
		end = len(out)
	}
	return out[start:end], int64(len(out)), nil
}

func TestQueryService_List_Forbidden(t *testing.T) {
	s := NewQueryService(&fakeRepo{})
	if _, err := s.List(context.Background(), EventFilter{}, 1, 50, false); !errors.Is(err, ErrForbidden) {
		t.Fatalf("expect ErrForbidden, got %v", err)
	}
}

func TestQueryService_List_DBUnavailable(t *testing.T) {
	s := NewQueryService(nil)
	if _, err := s.List(context.Background(), EventFilter{}, 1, 50, true); !errors.Is(err, ErrDBUnavailable) {
		t.Fatalf("expect ErrDBUnavailable, got %v", err)
	}
}

func TestQueryService_List_FilterAndPage(t *testing.T) {
	repo := &fakeRepo{events: []Event{
		{ID: 1, EntityType: "task", EntityID: 9, ActorID: 2, Action: "dispatch"},
		{ID: 2, EntityType: "task", EntityID: 9, ActorID: 2, Action: "complete"},
		{ID: 3, EntityType: "observation", EntityID: 5, ActorID: 3, Action: "submit"},
		{ID: 4, EntityType: "task", EntityID: 8, ActorID: 2, Action: "dispatch"},
	}}
	s := NewQueryService(repo)
	eid := int64(9)
	p, err := s.List(context.Background(), EventFilter{EntityType: "task", EntityID: &eid}, 1, 1, true)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if p.Total != 2 || len(p.Items) != 1 || p.Items[0].ID != 2 {
		t.Fatalf("unexpected page: total=%d items=%v", p.Total, p.Items)
	}
	// 第二页。
	p2, err := s.List(context.Background(), EventFilter{EntityType: "task", EntityID: &eid}, 2, 1, true)
	if err != nil {
		t.Fatalf("List page2: %v", err)
	}
	if len(p2.Items) != 1 || p2.Items[0].ID != 1 {
		t.Fatalf("unexpected page2: %v", p2.Items)
	}
}
