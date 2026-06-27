package auth

import (
	"context"

	"api-gateway/models"

	"github.com/google/uuid"
)

type Repository interface {
	CreateUser(ctx context.Context, email, passwordHash string) (*models.User, error)
	GetUserByEmail(ctx context.Context, email string) (*models.User, error)
	GetUserByID(ctx context.Context, id uuid.UUID) (*models.User, error)
	DeleteUser(ctx context.Context, id uuid.UUID) error
}

type TokenStore interface {
	StoreRefreshToken(ctx context.Context, userID uuid.UUID, tokenID string, ttlSeconds int) error
	GetRefreshTokenUserID(ctx context.Context, tokenID string) (uuid.UUID, error)
	RevokeRefreshToken(ctx context.Context, tokenID string) error
	RevokeAllUserTokens(ctx context.Context, userID uuid.UUID) error
}

