package idempotency

import (
	"context"
	"time"
)

// Service 幂等检查与记录。
type Service interface {
	// Check 幂等检查（只查询不写入）：同 actor+operation+key 已有 COMPLETED 记录 → (true, rec)。
	// 记录为 CONFLICT/失败态时不视为已执行（返回 false,nil，允许重试）。
	Check(ctx context.Context, actorID int64, operation, idemKey string) (bool, *Record, error)
	// Record 执行成功后补写记录；并发冲突（另一请求已写）时忽略，不报错。
	Record(ctx context.Context, actorID int64, operation, idemKey, requestHash string) error
}

// Options 幂等记录过期策略。
const defaultTTL = 30 * time.Minute

type service struct {
	repo Repository
}

// NewService 构造幂等服务。
func NewService(repo Repository) Service {
	return &service{repo: repo}
}

// CheckIdempotent 结构性实现 observation.IdempotencyStore 查询部分。
func (s *service) CheckIdempotent(ctx context.Context, actorID int64, operation, key string) (bool, error) {
	done, _, err := s.Check(ctx, actorID, operation, key)
	return done, err
}

// RecordIdempotent 结构性实现 observation.IdempotencyStore 记录部分。
func (s *service) RecordIdempotent(ctx context.Context, actorID int64, operation, key, requestHash string) error {
	return s.Record(ctx, actorID, operation, key, requestHash)
}

func (s *service) Check(ctx context.Context, actorID int64, operation, idemKey string) (bool, *Record, error) {
	if idemKey == "" {
		return false, nil, ErrKeyRequired
	}
	existing, err := s.repo.Get(ctx, actorID, operation, idemKey)
	if err != nil {
		return false, nil, err
	}
	if existing != nil {
		if existing.Status == StatusCompleted {
			return true, existing, nil
		}
		// 失败/冲突记录：允许同键重试（下次执行成功后覆盖）。
		return false, nil, nil
	}
	return false, nil, nil
}

func (s *service) Record(ctx context.Context, actorID int64, operation, idemKey, requestHash string) error {
	if idemKey == "" {
		return ErrKeyRequired
	}
	now := time.Now().UTC()
	expires := now.Add(defaultTTL)
	rec := &Record{
		ActorID:     actorID,
		Operation:   operation,
		IdemKey:     idemKey,
		RequestHash: requestHash,
		Status:      StatusCompleted,
		ExpiresAt:   &expires,
		CreatedAt:   now,
	}
	if err := s.repo.Save(ctx, rec); err != nil {
		if err == ErrRecordExists {
			// 并发另一请求已写入：忽略（其执行成功）。
			return nil
		}
		return err
	}
	return nil
}
