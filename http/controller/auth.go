package controller

import (
	"encoding/json"
	"net/http"

	apperrors "api-gateway/errors"
	authsvc "api-gateway/services/auth"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type AuthController struct {
	svc *authsvc.Service
}

func NewAuthController(svc *authsvc.Service) *AuthController {
	return &AuthController{svc: svc}
}

func (c *AuthController) Register(w http.ResponseWriter, r *http.Request) (any, int, error) {
	var req authsvc.RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return nil, http.StatusBadRequest, apperrors.InvalidBodyErr(err)
	}

	res, err := c.svc.Register(r.Context(), req)
	if err != nil {
		return nil, statusFromError(err), err
	}
	return res, http.StatusCreated, nil
}

func (c *AuthController) Login(w http.ResponseWriter, r *http.Request) (any, int, error) {
	var req authsvc.LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return nil, http.StatusBadRequest, apperrors.InvalidBodyErr(err)
	}

	res, err := c.svc.Login(r.Context(), req)
	if err != nil {
		return nil, statusFromError(err), err
	}
	return res, http.StatusOK, nil
}

func (c *AuthController) Refresh(w http.ResponseWriter, r *http.Request) (any, int, error) {
	var req authsvc.RefreshRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return nil, http.StatusBadRequest, apperrors.InvalidBodyErr(err)
	}

	tokens, err := c.svc.Refresh(r.Context(), req)
	if err != nil {
		return nil, statusFromError(err), err
	}
	return tokens, http.StatusOK, nil
}

func (c *AuthController) DeleteUser(w http.ResponseWriter, r *http.Request) (any, int, error) {
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		return nil, http.StatusBadRequest, apperrors.InvalidErr(err)
	}

	if err := c.svc.DeleteUser(r.Context(), id); err != nil {
		return nil, statusFromError(err), err
	}
	return map[string]string{"message": "user deleted"}, http.StatusOK, nil
}

func statusFromError(err error) int {
	var appErr *apperrors.Error
	if apperrors.As(err, &appErr) {
		switch appErr.Kind {
		case apperrors.Invalid:
			return http.StatusBadRequest
		case apperrors.NotFound:
			return http.StatusNotFound
		case apperrors.Unauthorized:
			return http.StatusUnauthorized
		case apperrors.Forbidden:
			return http.StatusForbidden
		case apperrors.Conflict:
			return http.StatusConflict
		default:
			return http.StatusInternalServerError
		}
	}
	return http.StatusInternalServerError
}
