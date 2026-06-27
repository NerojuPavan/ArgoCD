package http

import (
	"context"
	"net/http"
	"time"

	configs "api-gateway/config"
	"api-gateway/errors"
	"api-gateway/http/controller"
	appmiddleware "api-gateway/http/middleware"
	response "api-gateway/http/response"
	"api-gateway/logger"
	authsvc "api-gateway/services/auth"
	"api-gateway/services/health"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type Server struct {
	config    *configs.Config
	healthSvc *health.Service
	authCtrl  *controller.AuthController
}

func NewServer(cfg *configs.Config, healthSvc *health.Service, authSvc *authsvc.Service) *Server {
	return &Server{
		config:    cfg,
		healthSvc: healthSvc,
		authCtrl:  controller.NewAuthController(authSvc),
	}
}

func (s *Server) Listen(ctx context.Context, addr string) error {
	r := chi.NewRouter()
	r.Use(middleware.Recoverer)
	r.Use(appmiddleware.LoggingMiddleware(logger.Logger, logger.IsHTTPLoggingEnabled()))
	r.Use(appmiddleware.PrometheusMetrics(s.config.IsMetricsEnabled))
	r.Use(appmiddleware.EnableCORS(s.config.CORS.AllowedOrigins))

	if s.config.IsMetricsEnabled {
		r.Get("/metrics", promhttp.Handler().ServeHTTP)
	}

	s.registerRoutes(r)

	server := &http.Server{Addr: addr, Handler: r}
	errCh := make(chan error, 1)

	go func() {
		errCh <- server.ListenAndServe()
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return server.Shutdown(shutdownCtx)
	}
}

func (s *Server) registerRoutes(r chi.Router) {
	r.Route("/api/v1", func(r chi.Router) {
		r.Get("/health", s.ToHttpHandlerFunc(s.healthHandler))

		r.Route("/auth", func(r chi.Router) {
			r.Post("/register", s.ToHttpHandlerFunc(s.authCtrl.Register))
			r.Post("/login", s.ToHttpHandlerFunc(s.authCtrl.Login))
			r.Post("/refresh", s.ToHttpHandlerFunc(s.authCtrl.Refresh))
		})

		r.Delete("/users/{id}", s.ToHttpHandlerFunc(s.authCtrl.DeleteUser))
	})
}

func (s *Server) healthHandler(_ http.ResponseWriter, r *http.Request) (any, int, error) {
	if !s.healthSvc.Health(r.Context()) {
		return nil, http.StatusServiceUnavailable, errors.E(errors.Internal, "health check failed")
	}
	return map[string]string{"status": "ok"}, http.StatusOK, nil
}

func (s *Server) ToHttpHandlerFunc(handler func(w http.ResponseWriter, r *http.Request) (any, int, error)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		res, status, err := handler(w, r)
		if err != nil {
			var appErr *errors.Error
			var valErr errors.ValidationErrors

			if errors.As(err, &valErr) {
				logger.LogValidationError(r, valErr)
				wrapped := errors.E(errors.Invalid, "validation failed", valErr)
				if errors.As(wrapped, &appErr) {
					response.RespondError(w, appErr)
				} else {
					response.RespondMessage(w, http.StatusBadRequest, "validation failed")
				}
				return
			}

			if errors.As(err, &appErr) {
				if appErr.WrappedErr != nil && errors.As(appErr.WrappedErr, &valErr) {
					logger.LogValidationError(r, valErr)
				} else {
					logger.LogErrorWithStatus(r, status, appErr)
				}
				response.RespondError(w, appErr)
				return
			}

			logger.LogErrorWithStatus(r, status, err)
			response.RespondMessage(w, http.StatusInternalServerError, err.Error())
			return
		}

		if res != nil {
			response.RespondJSON(w, status, res)
		}
	}
}
