package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	configs "api-gateway/config"
	"api-gateway/http"
	"api-gateway/logger"
	authrepo "api-gateway/repositories/auth"
	"api-gateway/repositories/postgres"
	redisrepo "api-gateway/repositories/redis"
	authsvc "api-gateway/services/auth"
	"api-gateway/services/health"
	"api-gateway/services/scheduler"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	cfg := configs.Load()
	if err := cfg.Validate(); err != nil {
		log.Fatal("invalid configuration: ", err)
	}

	logger.InitLogger(cfg.Logger.Level, cfg.Logger.EnableHTTPLogging)

	dbPool, redisClient, err := connectDependencies(ctx, cfg)
	if err != nil {
		logger.Fatal("failed to connect dependencies", err)
	}
	defer dbPool.Close()
	defer redisClient.Close()

	server, err := buildServer(ctx, dbPool, redisClient, cfg)
	if err != nil {
		logger.Fatal("failed to build server", err)
	}

	schedulerService := scheduler.NewService(logger.Logger)
	schedulerService.Start(ctx)

	logger.Info("server listening", cfg.Port)
	if err := server.Listen(ctx, cfg.ListenAddr()); err != nil {
		logger.Fatal("server stopped", err)
	}
}

func connectDependencies(ctx context.Context, cfg configs.Config) (*pgxpool.Pool, *redis.Client, error) {
	dbPool, err := postgres.Connect(ctx, cfg.Database)
	if err != nil {
		return nil, nil, err
	}

	redisClient, err := redisrepo.Connect(ctx, cfg.Redis)
	if err != nil {
		dbPool.Close()
		return nil, nil, err
	}

	return dbPool, redisClient, nil
}

func buildServer(ctx context.Context, dbPool *pgxpool.Pool, redisClient *redis.Client, cfg configs.Config) (*http.Server, error) {
	userRepo := authrepo.NewPostgresRepository(dbPool)
	if err := userRepo.Migrate(ctx); err != nil {
		return nil, err
	}

	tokenStore := authrepo.NewRedisTokenStore(redisClient)
	authService := authsvc.NewService(userRepo, tokenStore, cfg.Auth)
	healthService := health.NewService(logger.Logger)

	return http.NewServer(&cfg, healthService, authService), nil
}
