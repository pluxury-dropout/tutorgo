package service

import (
	"context"
	"fmt"
	"tutorgo/models"
	"tutorgo/repository"
)

type LessonService interface {
	Create(ctx context.Context, req models.CreateLessonRequest, tutorID string) (models.Lesson, error)
	CreateBulk(ctx context.Context, req models.CreateBulkLessonRequest, tutorID string) ([]models.Lesson, error)
	GetByCourse(ctx context.Context, courseID string, tutorID string) ([]models.Lesson, error)
	GetByID(ctx context.Context, id string, tutorID string) (models.Lesson, error)
	Update(ctx context.Context, id string, req models.UpdateLessonRequest, tutorID string) (models.Lesson, error)
	Delete(ctx context.Context, id string, tutorID string) error
	DeleteByCourse(ctx context.Context, courseID string, tutorID string) error
	DeleteSeries(ctx context.Context, seriesID string, tutorID string, fromDate *string) error
	UpdateSeries(ctx context.Context, seriesID string, tutorID string, req models.UpdateSeriesRequest) error
	GetCalendar(ctx context.Context, tutorID string, from string, to string) ([]models.CalendarLesson, error)
	ExistsPublic(ctx context.Context, id string) error
}

type lessonService struct {
	repo       repository.LessonRepository
	courseRepo repository.CourseRepository
}

func NewLessonService(repo repository.LessonRepository, courseRepo repository.CourseRepository) LessonService {
	return &lessonService{repo: repo, courseRepo: courseRepo}
}

func (s *lessonService) Create(ctx context.Context, req models.CreateLessonRequest, tutorID string) (models.Lesson, error) {
	_, err := s.courseRepo.GetByID(ctx, req.CourseID, tutorID)
	if err != nil {
		return models.Lesson{}, fmt.Errorf("course: %w", ErrNotFound)
	}
	return s.repo.Create(ctx, req)
}

func (s *lessonService) CreateBulk(ctx context.Context, req models.CreateBulkLessonRequest, tutorID string) ([]models.Lesson, error) {
	_, err := s.courseRepo.GetByID(ctx, req.CourseID, tutorID)
	if err != nil {
		return nil, fmt.Errorf("course: %w", ErrNotFound)
	}
	return s.repo.CreateBulk(ctx, req)
}

func (s *lessonService) GetByCourse(ctx context.Context, courseID string, tutorID string) ([]models.Lesson, error) {
	_, err := s.courseRepo.GetByID(ctx, courseID, tutorID)
	if err != nil {
		return nil, fmt.Errorf("course: %w", ErrNotFound)
	}
	return s.repo.GetByCourse(ctx, courseID)
}

func (s *lessonService) GetByID(ctx context.Context, id string, tutorID string) (models.Lesson, error) {
	lesson, err := s.repo.GetByIDForTutor(ctx, id, tutorID)
	if err != nil {
		return models.Lesson{}, fmt.Errorf("lesson: %w", ErrNotFound)
	}
	return lesson, nil
}

func (s *lessonService) Update(ctx context.Context, id string, req models.UpdateLessonRequest, tutorID string) (models.Lesson, error) {
	_, err := s.repo.GetByIDForTutor(ctx, id, tutorID)
	if err != nil {
		return models.Lesson{}, fmt.Errorf("lesson: %w", ErrNotFound)
	}
	return s.repo.Update(ctx, id, req)
}

func (s *lessonService) Delete(ctx context.Context, id string, tutorID string) error {
	_, err := s.repo.GetByIDForTutor(ctx, id, tutorID)
	if err != nil {
		return fmt.Errorf("lesson: %w", ErrNotFound)
	}
	return s.repo.Delete(ctx, id)
}

func (s *lessonService) DeleteByCourse(ctx context.Context, courseID string, tutorID string) error {
	_, err := s.courseRepo.GetByID(ctx, courseID, tutorID)
	if err != nil {
		return fmt.Errorf("course: %w", ErrNotFound)
	}
	return s.repo.DeleteByCourse(ctx, courseID, tutorID)
}

func (s *lessonService) DeleteSeries(ctx context.Context, seriesID string, tutorID string, fromDate *string) error {
	return s.repo.DeleteSeries(ctx, seriesID, tutorID, fromDate)
}

func (s *lessonService) UpdateSeries(ctx context.Context, seriesID string, tutorID string, req models.UpdateSeriesRequest) error {
	if req.NewTime == nil && req.DurationMinutes == nil && req.Notes == nil {
		return fmt.Errorf("update requires at least one field: %w", ErrBadRequest)
	}
	return s.repo.UpdateSeries(ctx, seriesID, tutorID, req)
}

func (s *lessonService) GetCalendar(ctx context.Context, tutorID string, from string, to string) ([]models.CalendarLesson, error) {
	return s.repo.GetCalendar(ctx, tutorID, from, to)
}

func (s *lessonService) ExistsPublic(ctx context.Context, id string) error {
	return s.repo.ExistsPublic(ctx, id)
}
