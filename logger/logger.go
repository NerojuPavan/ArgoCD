package logger

import (
	"os"
	"strings"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// enableHTTPLogging stores whether HTTP request logging is enabled.
// This is set during logger initialization and can be retrieved via IsHTTPLoggingEnabled().
var Logger *zap.Logger
var enableHTTPLogging bool

// InitLogger initializes the global logger and configures its settings.
//
// Parameters:
//   - logLevel: Optional log level string ("debug", "info", "warn", "error"). If empty, uses APP_ENV or defaults to "info"
//   - enableHTTPLogging: Whether to enable HTTP request logging (default: true)
//
// Configuration:
//   - Log Level: Determined by logLevel parameter, APP_ENV, or defaults to InfoLevel
//   - "debug", "dev", or "development" → DebugLevel (logs everything)
//   - Otherwise → InfoLevel (logs info, warn, error, fatal only)
//   - Output Format:
//   - Development mode → Console encoder (human-readable, colored output)
//   - Production mode → JSON encoder (structured, suitable for log aggregation systems like ELK, Datadog, Loki)
//
// The logger is configured with:
//   - RFC3339 timestamp format
//   - Lowercase log levels for JSON compatibility
//   - Duration encoding in milliseconds
//   - Caller information (file and line number)
//   - Stack traces for error-level logs and above
func InitLogger(logLevel string, enableHTTPLoggingParam ...bool) {
	// Get APP_ENV environment variable and normalize to lowercase for comparison
	appEnv := strings.ToLower(os.Getenv("APP_ENV"))

	// Determine log level: use parameter if provided, otherwise use APP_ENV, otherwise default to info
	var levelStr string
	if logLevel != "" {
		levelStr = strings.ToLower(logLevel)
	} else if appEnv != "" {
		levelStr = appEnv
	} else {
		levelStr = "info"
	}

	// Set HTTP logging flag: use parameter if provided, otherwise default to true
	if len(enableHTTPLoggingParam) > 0 {
		enableHTTPLogging = enableHTTPLoggingParam[0]
	} else {
		enableHTTPLogging = true // Default to enabled
	}

	// Determine log level based on level string
	// In development, use DebugLevel to see all logs including debug messages
	// In production, use InfoLevel to reduce noise and improve performance
	var zapLogLevel zapcore.Level
	switch levelStr {
	case "debug", "dev", "development":
		zapLogLevel = zapcore.DebugLevel
	case "warn", "warning":
		zapLogLevel = zapcore.WarnLevel
	case "error":
		zapLogLevel = zapcore.ErrorLevel
	default:
		zapLogLevel = zapcore.InfoLevel
	}

	// Configure the encoder with custom field names and formats
	// This ensures consistent structure across all log entries
	encoderConfig := zapcore.EncoderConfig{
		TimeKey:        "timestamp",                               // Field name for timestamp
		LevelKey:       "level",                                   // Field name for log level
		NameKey:        "logger",                                  // Field name for logger name
		MessageKey:     "message",                                 // Field name for log message
		StacktraceKey:  "stacktrace",                              // Field name for stack traces
		LineEnding:     zapcore.DefaultLineEnding,                 // Use default line ending
		EncodeLevel:    zapcore.LowercaseLevelEncoder,             // Lowercase log levels (info, error, etc.) for JSON compatibility
		EncodeTime:     zapcore.TimeEncoderOfLayout(time.RFC3339), // RFC3339 format: 2006-01-02T15:04:05Z07:00
		EncodeDuration: zapcore.MillisDurationEncoder,             // Duration in milliseconds (e.g., "150ms")
		EncodeCaller:   zapcore.ShortCallerEncoder,                // Short caller format: file.go:line
	}

	// Choose encoder based on environment
	// Console encoder is more readable for development
	// JSON encoder is required for production log aggregation systems
	var encoder zapcore.Encoder
	if levelStr == "debug" || levelStr == "dev" || levelStr == "development" || appEnv == "debug" || appEnv == "dev" || appEnv == "development" {
		// Use console encoder for development (more readable, colored output)
		encoder = zapcore.NewConsoleEncoder(encoderConfig)
	} else {
		// Use JSON encoder for production (structured, machine-readable)
		encoder = zapcore.NewJSONEncoder(encoderConfig)
	}

	// Write logs to stdout (standard output)
	// This allows log aggregation systems to capture logs from container stdout
	writer := zapcore.AddSync(os.Stdout)

	// Create the core logger with encoder, writer, and log level
	core := zapcore.NewCore(encoder, writer, zapLogLevel)

	// Build logger options
	// These options enhance the logger with additional functionality
	options := []zap.Option{
		zap.AddCaller(),                       // Include caller information (file:line) in logs
		zap.AddCallerSkip(1),                  // Skip one caller frame (the wrapper function)
		zap.AddStacktrace(zapcore.ErrorLevel), // Include stack traces for error-level logs and above
	}

	// Add development mode only for debug/dev environments
	// Development mode provides more detailed error messages and stack traces
	if levelStr == "debug" || levelStr == "dev" || levelStr == "development" || appEnv == "debug" || appEnv == "dev" || appEnv == "development" {
		options = append(options, zap.Development())
	}

	// Create and assign the global logger instance
	Logger = zap.New(core, options...)
}

// Info logs a message at InfoLevel with optional fields.
// Use this for general informational messages about application flow.
func Info(msg string, args ...interface{}) {
	Logger.Info(msg, ConvertArgsToFields(args...)...)
}

// Error logs a message at ErrorLevel with optional fields.
// Use this for error conditions that should be investigated.
func Error(msg string, args ...interface{}) {
	Logger.Error(msg, ConvertArgsToFields(args...)...)
}

// Debug logs a message at DebugLevel with optional fields.
// Use this for detailed debugging information (only visible in debug mode).
func Debug(msg string, args ...interface{}) {
	Logger.Debug(msg, ConvertArgsToFields(args...)...)
}

// Fatal logs a message at FatalLevel and then calls os.Exit(1).
// Use this for critical errors that require immediate application termination.
func Fatal(msg string, args ...interface{}) {
	Logger.Fatal(msg, ConvertArgsToFields(args...)...)
}

// Warn logs a message at WarnLevel with optional fields.
// Use this for warning conditions that may need attention but don't stop execution.
func Warn(msg string, args ...interface{}) {
	Logger.Warn(msg, ConvertArgsToFields(args...)...)
}

// Panic logs a message at PanicLevel and then panics.
// Use this for conditions that should not occur and indicate a programming error.
func Panic(msg string, args ...interface{}) {
	Logger.Panic(msg, ConvertArgsToFields(args...)...)
}

// IsHTTPLoggingEnabled returns whether HTTP request logging is enabled.
// This value is set during logger initialization via InitLogger().
func IsHTTPLoggingEnabled() bool {
	return enableHTTPLogging
}

// ConvertArgsToFields converts a variadic list of arguments to zap.Field slice.
// This allows the logger wrapper functions to accept any type of arguments
// and automatically convert them to structured zap fields.
func ConvertArgsToFields(args ...interface{}) []zap.Field {
	fields := make([]zap.Field, len(args))
	for i, arg := range args {
		fields[i] = convertToField(arg)
	}
	return fields
}

// convertToField converts an argument to a zap.Field based on its type.
// This function performs type assertion to determine the appropriate zap field type,
// ensuring type-safe logging with optimal performance.
func convertToField(arg interface{}) zap.Field {
	switch v := arg.(type) {
	case string:
		return zap.String("string", v)
	case int:
		return zap.Int("int", v)
	case int64:
		return zap.Int64("int64", v)
	case float64:
		return zap.Float64("float64", v)
	case bool:
		return zap.Bool("bool", v)
	case error:
		return zap.Error(v) // Special handling for errors (includes stack trace if available)
	case rune:
		return zap.String("rune", string(v))
	default:
		// Fallback for any other type - uses reflection (slower but flexible)
		return zap.Any("any", v)
	}
}
