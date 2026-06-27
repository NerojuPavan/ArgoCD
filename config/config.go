package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	Logger             LoggerConfig
	Database           DatabaseConfig
	Redis              RedisConfig
	Auth               AuthConfig
	CORS               CORSConfig
	IsMetricsEnabled   bool
	Port               int
}

type CORSConfig struct {
	AllowedOrigins []string
}

type LoggerConfig struct {
	Level             string
	EnableHTTPLogging bool
}

type DatabaseConfig struct {
	Host     string
	Port     int
	User     string
	Password string
	DBName   string
}

func (d DatabaseConfig) DSN() string {
	return fmt.Sprintf(
		"postgres://%s:%s@%s:%d/%s?sslmode=disable",
		d.User, d.Password, d.Host, d.Port, d.DBName,
	)
}

type RedisConfig struct {
	Host string
	Port int
}

func (r RedisConfig) Addr() string {
	return fmt.Sprintf("%s:%d", r.Host, r.Port)
}

type AuthConfig struct {
	JWTSecret          string
	AccessTokenExpiry  int // minutes
	RefreshTokenExpiry int // hours
}

func Load() Config {
	cfg := DefaultConfig

	if v := os.Getenv("LOG_LEVEL"); v != "" {
		cfg.Logger.Level = v
	}
	if v := os.Getenv("ENABLE_HTTP_LOGGING"); v != "" {
		cfg.Logger.EnableHTTPLogging = v == "true" || v == "1"
	}
	if v := os.Getenv("DB_HOST"); v != "" {
		cfg.Database.Host = v
	}
	if v := os.Getenv("DB_PORT"); v != "" {
		if p, err := strconv.Atoi(v); err == nil {
			cfg.Database.Port = p
		}
	}
	if v := os.Getenv("DB_USER"); v != "" {
		cfg.Database.User = v
	}
	if v := os.Getenv("DB_PASSWORD"); v != "" {
		cfg.Database.Password = v
	}
	if v := os.Getenv("DB_NAME"); v != "" {
		cfg.Database.DBName = v
	}
	if v := os.Getenv("REDIS_HOST"); v != "" {
		cfg.Redis.Host = v
	}
	if v := os.Getenv("REDIS_PORT"); v != "" {
		if p, err := strconv.Atoi(v); err == nil {
			cfg.Redis.Port = p
		}
	}
	if v := os.Getenv("JWT_SECRET"); v != "" {
		cfg.Auth.JWTSecret = v
	}
	if v := os.Getenv("ACCESS_TOKEN_EXPIRY_MIN"); v != "" {
		if p, err := strconv.Atoi(v); err == nil {
			cfg.Auth.AccessTokenExpiry = p
		}
	}
	if v := os.Getenv("REFRESH_TOKEN_EXPIRY_HOURS"); v != "" {
		if p, err := strconv.Atoi(v); err == nil {
			cfg.Auth.RefreshTokenExpiry = p
		}
	}
	if v := os.Getenv("PORT"); v != "" {
		if p, err := strconv.Atoi(v); err == nil {
			cfg.Port = p
		}
	}
	if v := os.Getenv("METRICS_ENABLED"); v != "" {
		cfg.IsMetricsEnabled = v == "true" || v == "1"
	}
	if v := os.Getenv("CORS_ALLOWED_ORIGINS"); v != "" {
		cfg.CORS.AllowedOrigins = strings.Split(v, ",")
	}

	return cfg
}

func (c Config) Validate() error {
	if c.Auth.JWTSecret == "" {
		return fmt.Errorf("JWT_SECRET is required")
	}
	if len(c.Auth.JWTSecret) < 32 {
		return fmt.Errorf("JWT_SECRET must be at least 32 characters")
	}
	return nil
}

func (c Config) ListenAddr() string {
	return fmt.Sprintf(":%d", c.Port)
}

var DefaultConfig = Config{
	Logger: LoggerConfig{
		Level:             "info",
		EnableHTTPLogging: true,
	},
	Database: DatabaseConfig{
		Host:     "localhost",
		Port:     5432,
		User:     "postgres",
		Password: "postgres",
		DBName:   "api_gateway",
	},
	Redis: RedisConfig{
		Host: "localhost",
		Port: 6379,
	},
	Auth: AuthConfig{
		JWTSecret:          "bjhnhhSFHAjeABFCsnfcjBDSKVJCESDNFBbfhbjhvbcnbhcvjhbv,jdsbkjdsbcjdbc",
		AccessTokenExpiry:  15,
		RefreshTokenExpiry: 168, // 7 days
	},
	CORS: CORSConfig{
		AllowedOrigins: []string{"*"},
	},
	IsMetricsEnabled: true,
	Port:             8080,
}
