package service

import (
	"context"
	"fmt"
	"time"
	"tutorgo/models"
	"tutorgo/repository"
)

// Заморозка складывает уже существующие действия — зависимости узкими
// интерфейсами, как в onboarding.go.

type pauseStudents interface {
	GetByID(ctx context.Context, id string, tutorID string) (models.Student, error)
}

type pauseMaterializer interface {
	Materialize(ctx context.Context, ruleID string, horizon time.Time) (int, error)
}

type PauseService interface {
	Create(ctx context.Context, studentID, tutorID string, req models.CreatePauseRequest) (models.StudentPause, error)
	List(ctx context.Context, studentID, tutorID string) ([]models.StudentPause, error)
	Delete(ctx context.Context, id, studentID, tutorID string) error
}

type pauseService struct {
	repo       repository.PauseRepository
	students   pauseStudents
	recurrence pauseMaterializer
}

func NewPauseService(repo repository.PauseRepository, students pauseStudents, recurrence pauseMaterializer) PauseService {
	return &pauseService{repo: repo, students: students, recurrence: recurrence}
}

// Create замораживает ученика (спека, п. 6.9). Пауза, отмена уроков и сдвиг
// хвоста правил — один оператор в репозитории; здесь — только материализация
// сдвинутых хвостов. Её сбой паузу не отменяет: репозиторий откатил
// materialized_until, и правило попадёт в ночную ExtendAll.
func (s *pauseService) Create(ctx context.Context, studentID, tutorID string, req models.CreatePauseRequest) (models.StudentPause, error) {
	if _, err := s.students.GetByID(ctx, studentID, tutorID); err != nil {
		return models.StudentPause{}, fmt.Errorf("student: %w", ErrNotFound)
	}
	if req.EndsOn.Before(req.StartsOn) {
		return models.StudentPause{}, fmt.Errorf("ends_on before starts_on: %w", ErrBadRequest)
	}
	pause, shifted, err := s.repo.Create(ctx, studentID, req)
	if err != nil {
		return models.StudentPause{}, err
	}
	// Кеш сбрасываем сразу после оператора: уроки уже отменены, и календарь
	// обязан это показать, даже если материализация ниже не удастся.
	globalCalendarCache.Invalidate(tutorID)
	horizon := time.Now().Add(RecurrenceHorizon)
	for _, ruleID := range shifted {
		_, _ = s.recurrence.Materialize(ctx, ruleID, horizon)
	}
	return pause, nil
}

func (s *pauseService) List(ctx context.Context, studentID, tutorID string) ([]models.StudentPause, error) {
	if _, err := s.students.GetByID(ctx, studentID, tutorID); err != nil {
		return nil, fmt.Errorf("student: %w", ErrNotFound)
	}
	return s.repo.ListByStudent(ctx, studentID)
}

// Delete — разморозка: уроки паузы возвращаются в счёт, отменённые и сдвинутый
// хвост остаются как есть (спека, п. 6.9).
func (s *pauseService) Delete(ctx context.Context, id, studentID, tutorID string) error {
	if _, err := s.students.GetByID(ctx, studentID, tutorID); err != nil {
		return fmt.Errorf("student: %w", ErrNotFound)
	}
	deleted, err := s.repo.Delete(ctx, id, studentID)
	if err != nil {
		return err
	}
	if !deleted {
		return fmt.Errorf("pause: %w", ErrNotFound)
	}
	globalCalendarCache.Invalidate(tutorID)
	return nil
}
