package scheduler

import (
	"context"
	"time"

	"go.uber.org/zap"
)

type Service struct {
	logger *zap.Logger
}

func NewService(logger *zap.Logger) *Service {
	return &Service{logger: logger}
}

func (s *Service) LogEvery5Seconds(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			s.logger.Info("5-second logger stopped")
			return
		case t := <-ticker.C:
			s.logger.Info("scheduler tick", zap.Int("interval_seconds", 5), zap.Time("time", t))
		}
	}
}

func (s *Service) LogEvery10Seconds(ctx context.Context) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			s.logger.Info("10-second logger stopped")
			return
		case t := <-ticker.C:
			s.logger.Info("scheduler tick", zap.Int("interval_seconds", 10), zap.Time("time", t))
		}
	}
}

func (s *Service) Start(ctx context.Context) {
	go s.LogEvery5Seconds(ctx)
	go s.LogEvery10Seconds(ctx)
}
