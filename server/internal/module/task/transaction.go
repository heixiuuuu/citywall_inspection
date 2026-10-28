package task

import (
	"context"

	"citywall/server/internal/middleware"
)

func (s *service) transaction(ctx context.Context, operation func(*service) (*Task, error)) (*Task, error) {
	var result *Task
	err := s.repo.Transaction(ctx, func(repo Repository) error {
		transactional := &service{repo: repo, obsStatus: s.obsStatus, pointGetter: s.pointGetter, audit: s.audit}
		var err error
		result, err = operation(transactional)
		return err
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (s *service) Create(ctx context.Context, actor *middleware.Actor, in CreateInput) (*Task, error) {
	return s.transaction(ctx, func(tx *service) (*Task, error) {
		return tx.create(ctx, actor, in)
	})
}

func (s *service) Dispatch(ctx context.Context, actor *middleware.Actor, taskID, revision int64) (*Task, error) {
	return s.transaction(ctx, func(tx *service) (*Task, error) {
		return tx.dispatch(ctx, actor, taskID, revision)
	})
}
