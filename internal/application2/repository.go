package application2

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

const applicationTTL = 24 * time.Hour

type Repository struct {
	redis *redis.Client
}

func NewRepository(redis *redis.Client) *Repository {
	return &Repository{
		redis: redis,
	}
}

func (r *Repository) Save(ctx context.Context, app Application) error {
	key := KeyApplication(app.ChatID, app.UserID)

	data, err := json.Marshal(app)
	if err != nil {
		return err
	}

	return r.redis.Set(ctx, key, data, applicationTTL).Err()
}

func (r *Repository) Get(ctx context.Context, chatID, userID int64) (*Application, error) {
	key := KeyApplication(chatID, userID)

	data, err := r.redis.Get(ctx, key).Bytes()
	if errors.Is(err, redis.Nil) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get application2: %w", err)
	}

	var app Application
	if err := json.Unmarshal(data, &app); err != nil {
		return nil, fmt.Errorf("unmarshal application2: %w", err)
	}

	return &app, nil
}

func (r *Repository) Delete(ctx context.Context, chatID, userID int64) error {
	if err := r.redis.Del(ctx, KeyApplication(chatID, userID)).Err(); err != nil {
		return fmt.Errorf("delete application2: %w", err)
	}

	return nil
}

func KeyApplication(chatID, userID int64) string {
	return fmt.Sprintf("application2:%d:%d", chatID, userID)
}
