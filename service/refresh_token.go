package service

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"time"

	"tutorgo/repository"
)

var ErrTokenExpired = errors.New("token expired")

type RefreshTokenService interface {
	Create(ctx context.Context, tutorID string) (token string, err error)
	Validate(ctx context.Context, token string) (tutorID string, err error)
	Revoke(ctx context.Context, token string) error
	RevokeAll(ctx context.Context, tutorID string) error
}

type refreshTokenService struct {
	repo repository.RefreshTokenRepository
}

func NewRefreshTokenService(repo repository.RefreshTokenRepository) RefreshTokenService {
	return &refreshTokenService{repo: repo}
}

func (s *refreshTokenService) Create(ctx context.Context, tutorID string) (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	token := base64.URLEncoding.EncodeToString(b)
	expiresAt := time.Now().Add(30 * 24 * time.Hour)
	if err := s.repo.Create(ctx, tutorID, token, expiresAt); err != nil {
		return "", err
	}
	return token, nil
}

func (s *refreshTokenService) Validate(ctx context.Context, token string) (string, error) {
	tutorID, expiresAt, err := s.repo.GetByToken(ctx, token)
	if err != nil {
		return "", err
	}
	if time.Now().After(expiresAt) {
		return "", ErrTokenExpired
	}
	return tutorID, nil
}

func (s *refreshTokenService) Revoke(ctx context.Context, token string) error {
	return s.repo.DeleteByToken(ctx, token)
}

func (s *refreshTokenService) RevokeAll(ctx context.Context, tutorID string) error {
	return s.repo.DeleteAllByTutorID(ctx, tutorID)
}
