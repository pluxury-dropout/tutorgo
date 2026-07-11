package service

import (
	"context"
	"fmt"
	"time"
	"tutorgo/models"
	"tutorgo/repository"
)

type StudentService interface {
	Create(ctx context.Context, req models.CreateStudentRequest, tutorID string) (models.Student, error)
	GetAll(ctx context.Context, tutorID string, p models.Pagination) ([]models.Student, int, error)
	GetByID(ctx context.Context, id string, tutorID string) (models.Student, error)
	Update(ctx context.Context, id string, tutorID string, req models.UpdateStudentRequest) (models.Student, error)
	Delete(ctx context.Context, id string, tutorID string) error
	SetInvite(ctx context.Context, studentID, token string, expiresAt time.Time) error
	GetByInviteToken(ctx context.Context, token string) (string, time.Time, error)
	ActivateAccount(ctx context.Context, studentID, username, passwordHash string) error
	GetCredentialsByLogin(ctx context.Context, identifier string) (string, string, error)
	EnrolledInLesson(ctx context.Context, studentID, lessonID string) (bool, error)
	CourseAndTutorForLesson(ctx context.Context, lessonID string) (string, string, error)
	GetProfile(ctx context.Context, studentID string) (models.StudentProfile, error)
	ListLessons(ctx context.Context, studentID string, past bool) ([]models.CalendarLesson, error)
}

type studentService struct {
	repo repository.StudentRepository
}

func NewStudentService(repo repository.StudentRepository) StudentService {
	return &studentService{repo: repo}
}

func (s *studentService) Create(ctx context.Context, req models.CreateStudentRequest, tutorID string) (models.Student, error) {
	return s.repo.Create(ctx, req, tutorID)
}

func (s *studentService) GetAll(ctx context.Context, tutorID string, p models.Pagination) ([]models.Student, int, error) {
	return s.repo.GetAll(ctx, tutorID, p)
}

func (s *studentService) GetByID(ctx context.Context, id string, tutorID string) (models.Student, error) {
	student, err := s.repo.GetByID(ctx, id, tutorID)
	if err != nil {
		return models.Student{}, fmt.Errorf("student: %w", ErrNotFound)
	}
	return student, nil
}

func (s *studentService) Update(ctx context.Context, id string, tutorID string, req models.UpdateStudentRequest) (models.Student, error) {
	if _, err := s.repo.GetByID(ctx, id, tutorID); err != nil {
		return models.Student{}, fmt.Errorf("student: %w", ErrNotFound)
	}
	return s.repo.Update(ctx, id, tutorID, req)
}

func (s *studentService) Delete(ctx context.Context, id string, tutorID string) error {
	if _, err := s.repo.GetByID(ctx, id, tutorID); err != nil {
		return fmt.Errorf("student: %w", ErrNotFound)
	}
	return s.repo.Delete(ctx, id, tutorID)
}

func (s *studentService) SetInvite(ctx context.Context, studentID, token string, expiresAt time.Time) error {
	return s.repo.SetInvite(ctx, studentID, token, expiresAt)
}

func (s *studentService) GetByInviteToken(ctx context.Context, token string) (string, time.Time, error) {
	return s.repo.GetByInviteToken(ctx, token)
}

func (s *studentService) ActivateAccount(ctx context.Context, studentID, username, passwordHash string) error {
	return s.repo.ActivateAccount(ctx, studentID, username, passwordHash)
}

func (s *studentService) GetCredentialsByLogin(ctx context.Context, identifier string) (string, string, error) {
	return s.repo.GetCredentialsByLogin(ctx, identifier)
}

func (s *studentService) EnrolledInLesson(ctx context.Context, studentID, lessonID string) (bool, error) {
	return s.repo.EnrolledInLesson(ctx, studentID, lessonID)
}

func (s *studentService) CourseAndTutorForLesson(ctx context.Context, lessonID string) (string, string, error) {
	return s.repo.CourseAndTutorForLesson(ctx, lessonID)
}

func (s *studentService) GetProfile(ctx context.Context, studentID string) (models.StudentProfile, error) {
	return s.repo.GetProfile(ctx, studentID)
}

func (s *studentService) ListLessons(ctx context.Context, studentID string, past bool) ([]models.CalendarLesson, error) {
	return s.repo.ListLessons(ctx, studentID, past)
}
