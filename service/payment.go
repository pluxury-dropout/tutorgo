package service

import (
	"context"
	"fmt"
	"tutorgo/models"
	"tutorgo/repository"
)

type PaymentService interface {
	Create(ctx context.Context, req models.CreatePaymentRequest, tutorID string) (models.Payment, error)
	GetByCourse(ctx context.Context, courseID string, tutorID string, p models.Pagination) ([]models.Payment, int, error)
	GetAllByTutor(ctx context.Context, tutorID string, limit int) ([]models.Payment, error)
	GetAllByTutorPaged(ctx context.Context, tutorID string, p models.Pagination) ([]models.Payment, int, error)
	GetBalance(ctx context.Context, courseID string, tutorID string) (models.CourseBalance, error)
	GetMonthlyIncome(ctx context.Context, tutorID string) (float64, error)
	GetMonthlyExpected(ctx context.Context, tutorID string) (float64, error)
	Update(ctx context.Context, id string, tutorID string, req models.UpdatePaymentRequest) (models.Payment, error)
	Delete(ctx context.Context, id string, tutorID string) error
}

type paymentService struct {
	repo           repository.PaymentRepository
	courseRepo     repository.CourseRepository
	enrollmentRepo repository.EnrollmentRepository
}

func NewPaymentService(repo repository.PaymentRepository, courseRepo repository.CourseRepository, enrollmentRepo repository.EnrollmentRepository) PaymentService {
	return &paymentService{repo: repo, courseRepo: courseRepo, enrollmentRepo: enrollmentRepo}
}

// studentOnCourse проверяет, что платёж адресован ученику этого курса (спека,
// п. 6.3). Индивидуальный курс — ученик обязан совпасть с course.student_id,
// групповой — иметь запись, пусть и закрытую уходом. Иначе платёж уедет в
// баланс, который нигде не показывается.
func (s *paymentService) studentOnCourse(ctx context.Context, course models.Course, studentID string) error {
	if course.StudentID != nil {
		if *course.StudentID != studentID {
			return fmt.Errorf("student is not on course: %w", ErrBadRequest)
		}
		return nil
	}
	enrolled, err := s.enrollmentRepo.IsEnrolled(ctx, course.ID, studentID)
	if err != nil {
		return err
	}
	if !enrolled {
		return fmt.Errorf("student is not on course: %w", ErrBadRequest)
	}
	return nil
}

func (s *paymentService) Create(ctx context.Context, req models.CreatePaymentRequest, tutorID string) (models.Payment, error) {
	course, err := s.courseRepo.GetByID(ctx, req.CourseID, tutorID)
	if err != nil {
		return models.Payment{}, fmt.Errorf("course: %w", ErrNotFound)
	}
	if err := s.studentOnCourse(ctx, course, req.StudentID); err != nil {
		return models.Payment{}, err
	}
	payment, err := s.repo.Create(ctx, req)
	if err == nil {
		globalCalendarCache.Invalidate(tutorID)
	}
	return payment, err
}

func (s *paymentService) GetByCourse(ctx context.Context, courseID string, tutorID string, p models.Pagination) ([]models.Payment, int, error) {
	if _, err := s.courseRepo.GetByID(ctx, courseID, tutorID); err != nil {
		return nil, 0, fmt.Errorf("course: %w", ErrNotFound)
	}
	return s.repo.GetByCourse(ctx, courseID, p)
}

func (s *paymentService) GetAllByTutor(ctx context.Context, tutorID string, limit int) ([]models.Payment, error) {
	return s.repo.GetAllByTutor(ctx, tutorID, limit)
}

func (s *paymentService) GetAllByTutorPaged(ctx context.Context, tutorID string, p models.Pagination) ([]models.Payment, int, error) {
	return s.repo.GetAllByTutorPaged(ctx, tutorID, p)
}

func (s *paymentService) GetBalance(ctx context.Context, courseID string, tutorID string) (models.CourseBalance, error) {
	if _, err := s.courseRepo.GetByID(ctx, courseID, tutorID); err != nil {
		return models.CourseBalance{}, fmt.Errorf("course: %w", ErrNotFound)
	}
	return s.repo.GetBalance(ctx, courseID)
}

func (s *paymentService) GetMonthlyIncome(ctx context.Context, tutorID string) (float64, error) {
	return s.repo.GetMonthlyIncome(ctx, tutorID)
}

func (s *paymentService) GetMonthlyExpected(ctx context.Context, tutorID string) (float64, error) {
	return s.repo.GetMonthlyExpected(ctx, tutorID)
}

func (s *paymentService) Update(ctx context.Context, id string, tutorID string, req models.UpdatePaymentRequest) (models.Payment, error) {
	existing, err := s.repo.GetByID(ctx, id, tutorID)
	if err != nil {
		return models.Payment{}, fmt.Errorf("payment: %w", ErrNotFound)
	}
	course, err := s.courseRepo.GetByID(ctx, existing.CourseID, tutorID)
	if err != nil {
		return models.Payment{}, fmt.Errorf("course: %w", ErrNotFound)
	}
	if err := s.studentOnCourse(ctx, course, req.StudentID); err != nil {
		return models.Payment{}, err
	}
	payment, err := s.repo.Update(ctx, id, tutorID, req)
	if err != nil {
		return models.Payment{}, fmt.Errorf("payment: %w", ErrNotFound)
	}
	globalCalendarCache.Invalidate(tutorID)
	return payment, nil
}

func (s *paymentService) Delete(ctx context.Context, id string, tutorID string) error {
	if err := s.repo.Delete(ctx, id, tutorID); err != nil {
		return fmt.Errorf("payment: %w", ErrNotFound)
	}
	globalCalendarCache.Invalidate(tutorID)
	return nil
}
