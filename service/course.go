package service

import (
	"errors"
	"tutorgo/models"
	"tutorgo/repository"
)

type CourseService interface {
	Create(req models.CreateCourseRequest, tutorID string) (models.Course, error)
	GetAll(tutorID string) ([]models.Course, error)
	GetByID(id string, tutorID string) (models.Course, error)
	Update(id string, tutorID string, req models.UpdateCourseRequest) (models.Course, error)
	Delete(id string, tutorID string) error
}

type courseService struct {
	repo        repository.CourseRepository
	studentRepo repository.StudentRepository
	lessonRepo  repository.LessonRepository
}

func NewCourseService(repo repository.CourseRepository, studentRepo repository.StudentRepository, lessonRepo repository.LessonRepository) CourseService {
	return &courseService{repo: repo, studentRepo: studentRepo, lessonRepo: lessonRepo}
}

func (s *courseService) Create(req models.CreateCourseRequest, tutorID string) (models.Course, error) {
	_, err := s.studentRepo.GetByID(req.StudentID, tutorID)
	if err != nil {
		return models.Course{}, errors.New("student not found or access denied")
	}
	return s.repo.Create(req, tutorID)
}

func (s *courseService) GetAll(tutorID string) ([]models.Course, error) {
	return s.repo.GetAll(tutorID)
}

func (s *courseService) GetByID(id string, tutorID string) (models.Course, error) {
	return s.repo.GetByID(id, tutorID)
}

func (s *courseService) Update(id string, tutorID string, req models.UpdateCourseRequest) (models.Course, error) {
	_, err := s.repo.GetByID(id, tutorID)
	if err != nil {
		return models.Course{}, errors.New("course not found or access denied")
	}
	return s.repo.Update(id, tutorID, req)
}
func (s *courseService) Delete(id string, tutorID string) error {
	lessons, err := s.lessonRepo.GetByCourse(id)
	if err != nil {
		return err
	}
	if len(lessons) > 0 {
		return errors.New("cannot delete a course with existing lessons")
	}
	return s.repo.Delete(id, tutorID)
}

// type Course struct {
// 	ID             string    `json:"id"`
// 	StudentID      string    `json:"student_id"`
// 	TutorID        string    `json:"tutor_id"`
// 	Subject        string    `json:"subject"`
// 	PricePerLesson float64   `json:"price_per_lesson"`
// 	StartedAt      time.Time `json:"started_at"`
// 	EndedAt        time.Time `json:"ended_at"`
// }
