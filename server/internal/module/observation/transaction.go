package observation

import (
	"citywall/server/internal/middleware"
	"context"
)

// 状态校验、版本校验、快照及状态写入共享同一事务和记录锁。
func transactionValue[T any](ctx context.Context, s *service, operation func(*service) (T, error)) (T, error) {
	var result T
	err := s.repo.Transaction(ctx, func(repo Repository) error {
		transactional := &service{
			repo: repo, storage: s.storage, recognizer: s.recognizer,
			taskPoints: s.taskPoints, taskPointLister: s.taskPointLister,
			audit: s.audit, idemStore: s.idemStore,
		}
		var err error
		result, err = operation(transactional)
		return err
	})
	if err != nil {
		var empty T
		return empty, err
	}
	return result, nil
}

func (s *service) Create(ctx context.Context, actor *middleware.Actor, source string, taskPointID *int64, locationText string) (*Observation, error) {
	return transactionValue(ctx, s, func(tx *service) (*Observation, error) {
		return tx.create(ctx, actor, source, taskPointID, locationText)
	})
}

func (s *service) Update(ctx context.Context, actor *middleware.Actor, id, revision int64, input PatchInput) (*Observation, error) {
	return transactionValue(ctx, s, func(tx *service) (*Observation, error) {
		return tx.update(ctx, actor, id, revision, input)
	})
}
func (s *service) SaveConclusion(ctx context.Context, actor *middleware.Actor, id, revision int64, input ConclusionInput) (*ConclusionRevision, error) {
	return transactionValue(ctx, s, func(tx *service) (*ConclusionRevision, error) {
		return tx.saveConclusion(ctx, actor, id, revision, input)
	})
}
func (s *service) Submit(ctx context.Context, actor *middleware.Actor, id, revision int64) (*Observation, error) {
	return transactionValue(ctx, s, func(tx *service) (*Observation, error) {
		return tx.submit(ctx, actor, id, revision)
	})
}
func (s *service) Review(ctx context.Context, actor *middleware.Actor, id, revision int64, input ReviewInput) (*Observation, error) {
	return transactionValue(ctx, s, func(tx *service) (*Observation, error) {
		return tx.review(ctx, actor, id, revision, input)
	})
}
func (s *service) RemoveImage(ctx context.Context, actor *middleware.Actor, observationID, imageID int64) error {
	_, err := transactionValue(ctx, s, func(tx *service) (bool, error) {
		return true, tx.removeImage(ctx, actor, observationID, imageID)
	})
	return err
}
