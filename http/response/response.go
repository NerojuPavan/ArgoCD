package response

import (
	"encoding/json"
	"net/http"

	"api-gateway/errors"
)

type errorResponse struct {
	Kind    string `json:"kind,omitempty"`
	Message string `json:"message"`
}

func RespondJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if data != nil {
		_ = json.NewEncoder(w).Encode(data)
	}
}

func RespondMessage(w http.ResponseWriter, status int, message string) {
	RespondJSON(w, status, map[string]string{"message": message})
}

func RespondError(w http.ResponseWriter, err *errors.Error) {
	status := http.StatusInternalServerError
	switch err.Kind {
	case errors.Invalid:
		status = http.StatusBadRequest
	case errors.NotFound:
		status = http.StatusNotFound
	case errors.Unauthorized:
		status = http.StatusUnauthorized
	case errors.Forbidden:
		status = http.StatusForbidden
	case errors.Conflict:
		status = http.StatusConflict
	}

	msg := err.Message
	if msg == "" && err.WrappedErr != nil {
		msg = err.WrappedErr.Error()
	}

	RespondJSON(w, status, errorResponse{
		Kind:    err.Kind.String(),
		Message: msg,
	})
}
