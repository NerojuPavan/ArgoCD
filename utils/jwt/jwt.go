package jwt

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type Jwt struct {
	SecretKey     string
	SecretKeyByte []byte
}

func New(secretKey string) *Jwt {
	return &Jwt{
		SecretKey:     secretKey,
		SecretKeyByte: []byte(secretKey),
	}
}

func (j *Jwt) Decode(tokenStr string) (jwt.MapClaims, error) {
	token, err := jwt.Parse(tokenStr, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return j.SecretKeyByte, nil
	})
	if err != nil {
		return nil, err
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, jwt.ErrTokenInvalidClaims
	}

	if !token.Valid {
		return nil, jwt.ErrTokenInvalidId
	}

	return claims, nil
}

func (j *Jwt) GenerateToken(claims jwt.MapClaims) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS384, claims)
	return token.SignedString(j.SecretKeyByte)
}

func (j *Jwt) GenerateJwtToken(customerID string, duration time.Duration) (string, error) {
	claims := jwt.MapClaims{
		"user": customerID,
		"exp":  time.Now().Add(duration).Unix(),
		"iat":  time.Now().Unix(),
	}
	return j.GenerateToken(claims)
}

func FetchClaim(key string, claims jwt.MapClaims) string {
	if value, exists := claims[key].(string); exists && value != "" {
		return value
	}
	return ""
}
