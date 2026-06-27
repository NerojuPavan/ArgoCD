package auth

import (
	"context"
	"errors"
	"time"

	"api-gateway/config"
	apperrors "api-gateway/errors"
	"api-gateway/models"
	authrepo "api-gateway/repositories/auth"
	jwtutil "api-gateway/utils/jwt"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

type Service struct {
	repo       authrepo.Repository
	tokenStore authrepo.TokenStore
	cfg        config.AuthConfig
	jwt        *jwtutil.Jwt
}

func NewService(repo authrepo.Repository, tokenStore authrepo.TokenStore, cfg config.AuthConfig) *Service {
	return &Service{
		repo:       repo,
		tokenStore: tokenStore,
		cfg:        cfg,
		jwt:        jwtutil.New(cfg.JWTSecret),
	}
}

type TokenPair struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
}

type AuthResponse struct {
	User   UserResponse `json:"user"`
	Tokens TokenPair    `json:"tokens"`
}

type UserResponse struct {
	ID        uuid.UUID `json:"id"`
	Email     string    `json:"email"`
	CreatedAt time.Time `json:"created_at"`
}

type RegisterRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type RefreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

func (s *Service) Register(ctx context.Context, req RegisterRequest) (*AuthResponse, error) {
	if err := validateCredentials(req.Email, req.Password); err != nil {
		return nil, err
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, apperrors.E(apperrors.Internal, "failed to hash password", err)
	}

	user, err := s.repo.CreateUser(ctx, req.Email, string(hash))
	if err != nil {
		return nil, err
	}

	tokens, err := s.issueTokenPair(ctx, user)
	if err != nil {
		return nil, err
	}

	return &AuthResponse{
		User:   toUserResponse(user),
		Tokens: *tokens,
	}, nil
}

func (s *Service) Login(ctx context.Context, req LoginRequest) (*AuthResponse, error) {
	if req.Email == "" || req.Password == "" {
		return nil, apperrors.InvalidErr(errors.New("email and password are required"))
	}

	user, err := s.repo.GetUserByEmail(ctx, req.Email)
	if err != nil {
		return nil, apperrors.UnauthorizedErr(errors.New("invalid credentials"))
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		return nil, apperrors.UnauthorizedErr(errors.New("invalid credentials"))
	}

	tokens, err := s.issueTokenPair(ctx, user)
	if err != nil {
		return nil, err
	}

	return &AuthResponse{
		User:   toUserResponse(user),
		Tokens: *tokens,
	}, nil
}

func (s *Service) Refresh(ctx context.Context, req RefreshRequest) (*TokenPair, error) {
	if req.RefreshToken == "" {
		return nil, apperrors.InvalidErr(errors.New("refresh_token is required"))
	}

	tokenID, userIDStr, err := s.parseRefreshToken(req.RefreshToken)
	if err != nil {
		return nil, err
	}

	storedUserID, err := s.tokenStore.GetRefreshTokenUserID(ctx, tokenID)
	if err != nil {
		return nil, err
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil || storedUserID != userID {
		return nil, apperrors.UnauthorizedErr(errors.New("invalid refresh token"))
	}

	user, err := s.repo.GetUserByID(ctx, userID)
	if err != nil {
		return nil, err
	}

	if err := s.tokenStore.RevokeRefreshToken(ctx, tokenID); err != nil {
		return nil, err
	}

	return s.issueTokenPair(ctx, user)
}

func (s *Service) DeleteUser(ctx context.Context, id uuid.UUID) error {
	if err := s.tokenStore.RevokeAllUserTokens(ctx, id); err != nil {
		return err
	}
	return s.repo.DeleteUser(ctx, id)
}

func (s *Service) issueTokenPair(ctx context.Context, user *models.User) (*TokenPair, error) {
	accessToken, expiresIn, err := s.generateAccessToken(user)
	if err != nil {
		return nil, err
	}

	refreshToken, tokenID, err := s.generateRefreshToken(user.ID)
	if err != nil {
		return nil, err
	}

	ttlSeconds := s.cfg.RefreshTokenExpiry * 3600
	if err := s.tokenStore.StoreRefreshToken(ctx, user.ID, tokenID, ttlSeconds); err != nil {
		return nil, err
	}

	return &TokenPair{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    expiresIn,
	}, nil
}

func (s *Service) generateAccessToken(user *models.User) (string, int, error) {
	expiresIn := s.cfg.AccessTokenExpiry * 60
	now := time.Now()
	claims := jwt.MapClaims{
		"sub":   user.ID.String(),
		"email": user.Email,
		"typ":   "access",
		"iat":   now.Unix(),
		"exp":   now.Add(time.Duration(s.cfg.AccessTokenExpiry) * time.Minute).Unix(),
	}

	signed, err := s.jwt.GenerateToken(claims)
	if err != nil {
		return "", 0, apperrors.E(apperrors.Internal, "failed to sign access token", err)
	}
	return signed, expiresIn, nil
}

func (s *Service) generateRefreshToken(userID uuid.UUID) (string, string, error) {
	tokenID := uuid.New().String()
	now := time.Now()
	claims := jwt.MapClaims{
		"sub": userID.String(),
		"jti": tokenID,
		"typ": "refresh",
		"iat": now.Unix(),
		"exp": now.Add(time.Duration(s.cfg.RefreshTokenExpiry) * time.Hour).Unix(),
	}

	signed, err := s.jwt.GenerateToken(claims)
	if err != nil {
		return "", "", apperrors.E(apperrors.Internal, "failed to sign refresh token", err)
	}
	return signed, tokenID, nil
}

func (s *Service) parseRefreshToken(tokenStr string) (tokenID, userID string, err error) {
	claims, err := s.jwt.Decode(tokenStr)
	if err != nil {
		return "", "", apperrors.UnauthorizedErr(errors.New("invalid refresh token"))
	}

	if jwtutil.FetchClaim("typ", claims) != "refresh" {
		return "", "", apperrors.UnauthorizedErr(errors.New("invalid token type"))
	}

	tokenID = jwtutil.FetchClaim("jti", claims)
	userID = jwtutil.FetchClaim("sub", claims)
	if tokenID == "" || userID == "" {
		return "", "", apperrors.UnauthorizedErr(errors.New("invalid refresh token"))
	}

	return tokenID, userID, nil
}

func validateCredentials(email, password string) error {
	ve := apperrors.ValidationErrs()
	if email == "" {
		ve.Add("email", "email is required")
	}
	if password == "" {
		ve.Add("password", "password is required")
	} else if len(password) < 8 {
		ve.Add("password", "password must be at least 8 characters")
	}
	return ve.Err()
}

func toUserResponse(user *models.User) UserResponse {
	return UserResponse{
		ID:        user.ID,
		Email:     user.Email,
		CreatedAt: user.CreatedAt,
	}
}
