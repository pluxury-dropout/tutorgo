package service

import (
	"context"
	"fmt"
	"time"
	"tutorgo/models"
	"tutorgo/repository"
)

type CourseService interface {
	Create(ctx context.Context, req models.CreateCourseRequest, tutorID string) (models.Course, error)
	GetSubjects(ctx context.Context, tutorID string) ([]string, error)
	GetAll(ctx context.Context, tutorID string, p models.Pagination) ([]models.Course, int, error)
	GetByID(ctx context.Context, id string, tutorID string) (models.Course, error)
	GetByStudent(ctx context.Context, studentID string, tutorID string) ([]models.Course, error)
	Update(ctx context.Context, id string, tutorID string, req models.UpdateCourseRequest) (models.Course, error)
	Delete(ctx context.Context, id string, tutorID string) error
	GetArchived(ctx context.Context, tutorID string, p models.Pagination) ([]models.Course, int, error)
	Restore(ctx context.Context, id string, tutorID string) error
	GetHomework(ctx context.Context, id string, tutorID string) (string, error)
	SetHomework(ctx context.Context, id string, tutorID string, homework string) error
}

// courseSchedule — узкий выход к расписанию: courseService собран из
// courseRepo и studentRepo и до уроков с правилами сам не достаёт.
type courseSchedule interface {
	ArchiveCourseSchedule(ctx context.Context, courseID, tutorID string) error
}

type courseService struct {
	repo        repository.CourseRepository
	studentRepo repository.StudentRepository
	schedule    courseSchedule
}

func NewCourseService(repo repository.CourseRepository, studentRepo repository.StudentRepository, schedule courseSchedule) CourseService {
	return &courseService{repo: repo, studentRepo: studentRepo, schedule: schedule}
}

func (s *courseService) Create(ctx context.Context, req models.CreateCourseRequest, tutorID string) (models.Course, error) {
	if req.StudentID != nil {
		_, err := s.studentRepo.GetByID(ctx, *req.StudentID, tutorID)
		if err != nil {
			return models.Course{}, fmt.Errorf("student: %w", ErrNotFound)
		}
	}
	if req.StartedAt.IsZero() {
		req.StartedAt = time.Now()
	}
	return s.repo.Create(ctx, req, tutorID)
}

func (s *courseService) GetSubjects(ctx context.Context, tutorID string) ([]string, error) {
	return s.repo.GetSubjects(ctx, tutorID)
}

func (s *courseService) GetAll(ctx context.Context, tutorID string, p models.Pagination) ([]models.Course, int, error) {
	return s.repo.GetAll(ctx, tutorID, p)
}

func (s *courseService) GetByID(ctx context.Context, id string, tutorID string) (models.Course, error) {
	course, err := s.repo.GetByID(ctx, id, tutorID)
	if err != nil {
		return models.Course{}, fmt.Errorf("course: %w", ErrNotFound)
	}
	return course, nil
}

func (s *courseService) GetByStudent(ctx context.Context, studentID string, tutorID string) ([]models.Course, error) {
	if _, err := s.studentRepo.GetByID(ctx, studentID, tutorID); err != nil {
		return nil, fmt.Errorf("student: %w", ErrNotFound)
	}
	return s.repo.GetByStudent(ctx, studentID, tutorID)
}

func (s *courseService) Update(ctx context.Context, id string, tutorID string, req models.UpdateCourseRequest) (models.Course, error) {
	_, err := s.repo.GetByID(ctx, id, tutorID)
	if err != nil {
		return models.Course{}, fmt.Errorf("course: %w", ErrNotFound)
	}
	return s.repo.Update(ctx, id, tutorID, req)
}

func (s *courseService) Delete(ctx context.Context, id string, tutorID string) error {
	_, err := s.repo.GetByID(ctx, id, tutorID)
	if err != nil {
		return fmt.Errorf("course: %w", ErrNotFound)
	}
	// Расписание закрываем ДО is_active = false: GetByStudent (и через него
	// studentService.Archive) отдаёт только активные курсы, так что оборванный
	// вызов между шагами обязан оставить курс активным — тогда повтор (в том
	// числе из архивации ученика) увидит курс снова и доведёт закрытие
	// расписания до конца. Перевернуть порядок — значит после сбоя закрытия
	// расписания курс станет неактивным, выпадет из GetByStudent, и правило
	// с будущими уроками останется висеть открытым навсегда.
	if err := s.schedule.ArchiveCourseSchedule(ctx, id, tutorID); err != nil {
		return err
	}
	return s.repo.Delete(ctx, id, tutorID)
}

func (s *courseService) GetArchived(ctx context.Context, tutorID string, p models.Pagination) ([]models.Course, int, error) {
	return s.repo.GetAllArchived(ctx, tutorID, p)
}

func (s *courseService) Restore(ctx context.Context, id string, tutorID string) error {
	_, err := s.repo.GetByID(ctx, id, tutorID)
	if err != nil {
		return fmt.Errorf("course: %w", ErrNotFound)
	}
	return s.repo.Restore(ctx, id, tutorID)
}

func (s *courseService) GetHomework(ctx context.Context, id string, tutorID string) (string, error) {
	hw, err := s.repo.GetHomework(ctx, id, tutorID)
	if err != nil {
		return "", fmt.Errorf("course: %w", ErrNotFound)
	}
	return hw, nil
}

func (s *courseService) SetHomework(ctx context.Context, id string, tutorID string, homework string) error {
	n, err := s.repo.SetHomework(ctx, id, tutorID, homework)
	if err != nil {
		return err
	}
	if n == 0 {
		return fmt.Errorf("course: %w", ErrNotFound)
	}
	return nil
}
