package service

import (
	"context"
	"fmt"
	"tutorgo/models"
	"tutorgo/repository"
)

type AttendanceService interface {
	Update(ctx context.Context, lessonID string, req models.UpdateAttendanceRequest, tutorID string) error
	GetByLesson(ctx context.Context, lessonID string, tutorID string) ([]models.LessonAttendance, error)
}

type attendanceService struct {
	repo       repository.AttendanceRepository
	lessonRepo repository.LessonRepository
	courseRepo repository.CourseRepository
}

func NewAttendanceService(repo repository.AttendanceRepository, lessonRepo repository.LessonRepository, courseRepo repository.CourseRepository) AttendanceService {
	return &attendanceService{repo: repo, lessonRepo: lessonRepo, courseRepo: courseRepo}
}

func (s *attendanceService) Update(ctx context.Context, lessonID string, req models.UpdateAttendanceRequest, tutorID string) error {
	lesson, err := s.lessonRepo.GetByIDForTutor(ctx, lessonID, tutorID)
	if err != nil {
		return fmt.Errorf("lesson: %w", ErrNotFound)
	}
	course, err := s.courseRepo.GetByID(ctx, lesson.CourseID, tutorID)
	if err != nil {
		return fmt.Errorf("course: %w", ErrNotFound)
	}
	if course.StudentID != nil {
		return fmt.Errorf("attendance for individual courses: %w", ErrForbidden)
	}

	return s.repo.Upsert(ctx, lessonID, req.Attendances)
}

func (s *attendanceService) GetByLesson(ctx context.Context, lessonID string, tutorID string) ([]models.LessonAttendance, error) {
	if _, err := s.lessonRepo.GetByIDForTutor(ctx, lessonID, tutorID); err != nil {
		return nil, fmt.Errorf("lesson: %w", ErrNotFound)
	}
	return s.repo.GetByLesson(ctx, lessonID)
}
