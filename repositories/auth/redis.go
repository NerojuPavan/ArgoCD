package auth

import (
	"context"
	"fmt"
	"time"

	apperrors "api-gateway/errors"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

const refreshTokenPrefix = "refresh_token:"

type RedisTokenStore struct {
	client *redis.Client
}

func NewRedisTokenStore(client *redis.Client) *RedisTokenStore {
	return &RedisTokenStore{client: client}
}

func (s *RedisTokenStore) StoreRefreshToken(ctx context.Context, userID uuid.UUID, tokenID string, ttlSeconds int) error {
	key := refreshTokenPrefix + tokenID
	err := s.client.Set(ctx, key, userID.String(), time.Duration(ttlSeconds)*time.Second).Err()
	if err != nil {
		return apperrors.E(apperrors.Internal, "failed to store refresh token", err)
	}
	return nil
}

func (s *RedisTokenStore) GetRefreshTokenUserID(ctx context.Context, tokenID string) (uuid.UUID, error) {
	key := refreshTokenPrefix + tokenID
	val, err := s.client.Get(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			return uuid.Nil, apperrors.UnauthorizedErr(fmt.Errorf("invalid or expired refresh token"))
		}
		return uuid.Nil, apperrors.E(apperrors.Internal, "failed to get refresh token", err)
	}
	userID, err := uuid.Parse(val)
	if err != nil {
		return uuid.Nil, apperrors.E(apperrors.Internal, "invalid token data", err)
	}
	return userID, nil
}

func (s *RedisTokenStore) RevokeRefreshToken(ctx context.Context, tokenID string) error {
	key := refreshTokenPrefix + tokenID
	if err := s.client.Del(ctx, key).Err(); err != nil {
		return apperrors.E(apperrors.Internal, "failed to revoke refresh token", err)
	}
	return nil
}

func (s *RedisTokenStore) RevokeAllUserTokens(ctx context.Context, userID uuid.UUID) error {
	pattern := refreshTokenPrefix + "*"
	iter := s.client.Scan(ctx, 0, pattern, 100).Iterator()
	for iter.Next(ctx) {
		key := iter.Val()
		val, err := s.client.Get(ctx, key).Result()
		if err != nil {
			continue
		}
		if val == userID.String() {
			_ = s.client.Del(ctx, key).Err()
		}
	}
	if err := iter.Err(); err != nil {
		return apperrors.E(apperrors.Internal, "failed to revoke user tokens", err)
	}
	return nil
}
