package service

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"time"

	"tutorgo/repository"
)

type StudentRefreshTokenService interface {
	Create(ctx context.Context, studentID string) (token string, err error)
	Validate(ctx context.Context, token string) (studentID string, err error)
	Revoke(ctx context.Context, token string) error
}

type studentRefreshTokenService struct {
	repo repository.StudentRefreshTokenRepository
}

func NewStudentRefreshTokenService(repo repository.StudentRefreshTokenRepository) StudentRefreshTokenService {
	return &studentRefreshTokenService{repo: repo}
}

func (s *studentRefreshTokenService) Create(ctx context.Context, studentID string) (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	token := base64.URLEncoding.EncodeToString(b)
	expiresAt := time.Now().Add(30 * 24 * time.Hour)
	if err := s.repo.Create(ctx, studentID, token, expiresAt); err != nil {
		return "", err
	}
	return token, nil
}

func (s *studentRefreshTokenService) Validate(ctx context.Context, token string) (string, error) {
	studentID, expiresAt, err := s.repo.GetByToken(ctx, token)
	if err != nil {
		return "", err
	}
	if time.Now().After(expiresAt) {
		return "", ErrTokenExpired
	}
	return studentID, nil
}

func (s *studentRefreshTokenService) Revoke(ctx context.Context, token string) error {
	return s.repo.DeleteByToken(ctx, token)
}
