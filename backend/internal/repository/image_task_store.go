package repository

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/redis/go-redis/v9"
)

const (
	imageTaskKeyPrefix        = "image_task:"
	imageTaskRecoverableIndex = "image_task:recoverable"
)

type imageTaskStore struct {
	rdb *redis.Client
}

func NewImageTaskStore(rdb *redis.Client) service.ImageTaskStore {
	return &imageTaskStore{rdb: rdb}
}

func (s *imageTaskStore) Save(ctx context.Context, task *service.ImageTaskRecord, ttl time.Duration) error {
	data, err := json.Marshal(task)
	if err != nil {
		return err
	}
	_, err = s.rdb.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
		pipe.Set(ctx, imageTaskKey(task.ID), data, ttl)
		updateImageTaskRecoveryIndex(ctx, pipe, task)
		return nil
	})
	return err
}

func (s *imageTaskStore) Get(ctx context.Context, id string) (*service.ImageTaskRecord, error) {
	data, err := s.rdb.Get(ctx, imageTaskKey(id)).Bytes()
	if err != nil {
		if err == redis.Nil {
			return nil, service.ErrImageTaskNotFound
		}
		return nil, err
	}
	var task service.ImageTaskRecord
	if err := json.Unmarshal(data, &task); err != nil {
		return nil, err
	}
	return &task, nil
}

func (s *imageTaskStore) Update(ctx context.Context, id string, ttl time.Duration, mutate func(*service.ImageTaskRecord) error) error {
	key := imageTaskKey(id)
	for attempts := 0; attempts < 5; attempts++ {
		err := s.rdb.Watch(ctx, func(tx *redis.Tx) error {
			data, err := tx.Get(ctx, key).Bytes()
			if err != nil {
				if errors.Is(err, redis.Nil) {
					return service.ErrImageTaskNotFound
				}
				return err
			}
			var task service.ImageTaskRecord
			if err := json.Unmarshal(data, &task); err != nil {
				return err
			}
			if err := mutate(&task); err != nil {
				return err
			}
			updated, err := json.Marshal(&task)
			if err != nil {
				return err
			}
			_, err = tx.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
				pipe.Set(ctx, key, updated, ttl)
				updateImageTaskRecoveryIndex(ctx, pipe, &task)
				return nil
			})
			return err
		}, key)
		if !errors.Is(err, redis.TxFailedErr) {
			return err
		}
	}
	return redis.TxFailedErr
}

func (s *imageTaskStore) ListRecoverable(ctx context.Context, before time.Time, limit int64) ([]*service.ImageTaskRecord, error) {
	if limit <= 0 {
		limit = 100
	}
	ids, err := s.rdb.ZRangeByScore(ctx, imageTaskRecoverableIndex, &redis.ZRangeBy{
		Min: "-inf", Max: strconv.FormatInt(before.Unix(), 10), Offset: 0, Count: limit,
	}).Result()
	if err != nil || len(ids) == 0 {
		return nil, err
	}
	tasks := make([]*service.ImageTaskRecord, 0, len(ids))
	staleIDs := make([]any, 0)
	for _, id := range ids {
		task, getErr := s.Get(ctx, id)
		if errors.Is(getErr, service.ErrImageTaskNotFound) {
			staleIDs = append(staleIDs, id)
			continue
		}
		if getErr != nil {
			return nil, getErr
		}
		recoverable := task.Status == service.ImageTaskStatusProcessing ||
			task.Budget != nil && task.Budget.Status == service.ImageTaskBudgetStatusNeedsReview
		if !recoverable || task.RecoverAfter <= 0 {
			staleIDs = append(staleIDs, id)
			continue
		}
		tasks = append(tasks, task)
	}
	if len(staleIDs) > 0 {
		if err := s.rdb.ZRem(ctx, imageTaskRecoverableIndex, staleIDs...).Err(); err != nil {
			return nil, err
		}
	}
	return tasks, nil
}

func updateImageTaskRecoveryIndex(ctx context.Context, pipe redis.Pipeliner, task *service.ImageTaskRecord) {
	recoverable := task != nil && (task.Status == service.ImageTaskStatusProcessing ||
		task.Budget != nil && task.Budget.Status == service.ImageTaskBudgetStatusNeedsReview)
	if recoverable && task.RecoverAfter > 0 {
		pipe.ZAdd(ctx, imageTaskRecoverableIndex, redis.Z{Score: float64(task.RecoverAfter), Member: task.ID})
		return
	}
	if task != nil {
		pipe.ZRem(ctx, imageTaskRecoverableIndex, task.ID)
	}
}

func imageTaskKey(id string) string {
	return imageTaskKeyPrefix + strings.TrimSpace(id)
}
