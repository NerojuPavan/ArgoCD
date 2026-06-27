package logger

import (
	"net/http"

	"api-gateway/errors"

	"go.uber.org/zap"
)

// LogError logs an error with structured fields including request context.
// This function provides consistent error logging across the application
// with proper context for debugging and monitoring.
//
// Parameters:
//   - r: HTTP request (optional, can be nil) - used to extract request context
//   - err: The error to log
//   - additionalFields: Optional additional zap fields for context
//
// The function automatically extracts:
//   - Request ID from request context (if available)
//   - Request path and method (if request is provided)
//   - Error kind and message (for *errors.Error types)
//   - Wrapped error details (if available)
//
// Example usage:
//
//	logger.LogError(r, err, zap.String("operation", "user_creation"))
func LogError(r *http.Request, err error, additionalFields ...zap.Field) {
	if err == nil {
		return
	}

	// Build base fields
	fields := []zap.Field{}

	// Extract request context if available
	if r != nil {
		// Extract request ID from context (set by LoggingMiddleware)
		// Using string key to avoid import cycle
		const requestIDKey = "request_id"
		if requestID, ok := r.Context().Value(requestIDKey).(string); ok && requestID != "" {
			fields = append(fields, zap.String("request_id", requestID))
		}
		fields = append(fields,
			zap.String("path", r.URL.Path),
			zap.String("method", r.Method),
		)
	}

	// Handle structured errors
	if appErr, ok := err.(*errors.Error); ok {
		// Log structured application error with kind and message
		fields = append(fields,
			zap.String("error_kind", appErr.Kind.String()),
			zap.String("error_message", appErr.Message),
		)

		// Include wrapped error if present
		if appErr.WrappedErr != nil {
			fields = append(fields, zap.Error(appErr.WrappedErr))
		}
	} else {
		// Log generic error
		fields = append(fields, zap.Error(err))
	}

	// Add any additional fields provided by the caller
	fields = append(fields, additionalFields...)

	// Log at error level
	Logger.Error("Error occurred", fields...)
}

// LogErrorWithStatus logs an error with HTTP status code context.
// This is useful when logging errors that occur during HTTP request handling.
//
// Parameters:
//   - r: HTTP request (optional, can be nil)
//   - statusCode: HTTP status code associated with the error
//   - err: The error to log
//   - additionalFields: Optional additional zap fields for context
//
// Example usage:
//
//	logger.LogErrorWithStatus(r, http.StatusInternalServerError, err)
func LogErrorWithStatus(r *http.Request, statusCode int, err error, additionalFields ...zap.Field) {
	fields := []zap.Field{
		zap.Int("status_code", statusCode),
	}
	fields = append(fields, additionalFields...)
	LogError(r, err, fields...)
}

// LogValidationError logs validation errors with structured fields.
// This provides specialized logging for validation failures with field-level details.
//
// Parameters:
//   - r: HTTP request (optional, can be nil)
//   - validationErr: Validation errors to log
//   - additionalFields: Optional additional zap fields for context
//
// Example usage:
//
//	logger.LogValidationError(r, validationErrors)
func LogValidationError(r *http.Request, validationErr errors.ValidationErrors, additionalFields ...zap.Field) {
	fields := []zap.Field{
		zap.String("error_kind", "validation_error"),
		zap.Any("validation_errors", validationErr),
	}
	fields = append(fields, additionalFields...)
	LogError(r, validationErr, fields...)
}
