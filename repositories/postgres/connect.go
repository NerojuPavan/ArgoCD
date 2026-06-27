package postgres

import (
	"context"

	"api-gateway/config"

	"github.com/jackc/pgx/v5/pgxpool"
)

func Connect(ctx context.Context, cfg config.DatabaseConfig) (*pgxpool.Pool, error) {
	return pgxpool.New(ctx, cfg.DSN())
}
