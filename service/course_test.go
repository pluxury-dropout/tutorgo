package service_test

import (
	"tutorgo/models"

	"github.com/stretchr/testify/mock"
)

// type CourseRepository interface {
// 	Create(req models.CreateCourseRequest, tutorID string) (models.Course, error)
// 	GetAll(tutorID string) ([]models.Course, error)
// 	GetByID(id string, tutorID string) (models.Course, error)
// 	Update(id string, tutorID string, req models.UpdateCourseRequest) (models.Course, error)
// 	Delete(id string, tutorID string) error
// }

type mockCourseRepo struct {
	mock.Mock
}

func (m *mockCourseRepo) Create(req models.CreateCourseRequest, tutorID string) (models.Course, error) {
	args := m.Called(req, tutorID)
	return args.Get(0).(models.Course), args.Error(1)
}
func (m *mockCourseRepo) GetAll(tutorID string) ([]models.Course, error) {
	args := m.Called(tutorID)
	return args.Get(0).([]models.Course), args.Error(1)

}

func (m *mockCourseRepo) GetByID(id string, tutorID string) (models.Course, error) {
	args := m.Called(id, tutorID)
	return args.Get(0).(models.Course), args.Error(1)
}

func (m *mockCourseRepo) Update(id string, tutorID string, req models.UpdateCourseRequest) (models.Course, error) {
	args := m.Called(id, tutorID, req)
	return args.Get(0).(models.Course), args.Error(1)

}

func (m *mockCourseRepo) Delete(id string, tutorID string) error {
	args := m.Called(id, tutorID)
	return args.Error(0)
}
