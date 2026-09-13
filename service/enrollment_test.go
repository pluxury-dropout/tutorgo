package service_test

import (
	"context"
	"errors"
	"testing"
	"tutorgo/models"
	"tutorgo/service"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type mockEnrollmentRepo struct{ mock.Mock }

func (m *mockEnrollmentRepo) Add(ctx context.Context, courseID string, studentID string) (models.CourseEnrollment, error) {
	args := m.Called(ctx, courseID, studentID)
	return args.Get(0).(models.CourseEnrollment), args.Error(1)
}
func (m *mockEnrollmentRepo) AddBulk(ctx context.Context, courseID string, studentIDs []string, tutorID string) ([]models.CourseEnrollment, error) {
	args := m.Called(ctx, courseID, studentIDs, tutorID)
	return args.Get(0).([]models.CourseEnrollment), args.Error(1)
}
func (m *mockEnrollmentRepo) Remove(ctx context.Context, courseID string, studentID string) error {
	return m.Called(ctx, courseID, studentID).Error(0)
}
func (m *mockEnrollmentRepo) GetByCourse(ctx context.Context, courseID string) ([]models.CourseEnrollment, error) {
	args := m.Called(ctx, courseID)
	return args.Get(0).([]models.CourseEnrollment), args.Error(1)
}
func (m *mockEnrollmentRepo) LeaveAllByStudent(ctx context.Context, studentID string) error {
	return m.Called(ctx, studentID).Error(0)
}

var groupCourse = models.Course{ID: courseID, TutorID: tutorID, StudentID: nil, IsActive: true}

// Группа собирается одним сабмитом: по одному ученику за запрос — это диалог
// «добавить ученика» N раз вместо одной формы.
func TestEnrollmentAddBulk_Success(t *testing.T) {
	repo := new(mockEnrollmentRepo)
	courseRepo := new(mockCourseRepo)
	svc := service.NewEnrollmentService(repo, courseRepo, new(mockStudentRepo))

	ids := []string{"student-uuid-1", "student-uuid-2"}
	courseRepo.On("GetByID", mock.Anything, courseID, tutorID).Return(groupCourse, nil)
	repo.On("AddBulk", mock.Anything, courseID, ids, tutorID).
		Return([]models.CourseEnrollment{{ID: "e1"}, {ID: "e2"}}, nil)

	result, err := svc.AddBulk(context.Background(), courseID, models.EnrollStudentsBulkRequest{StudentIDs: ids}, tutorID)

	assert.NoError(t, err)
	assert.Len(t, result, 2)
	repo.AssertExpectations(t)
	courseRepo.AssertExpectations(t)
}

func TestEnrollmentAddBulk_IndividualCourse(t *testing.T) {
	repo := new(mockEnrollmentRepo)
	courseRepo := new(mockCourseRepo)
	svc := service.NewEnrollmentService(repo, courseRepo, new(mockStudentRepo))

	individual := models.Course{ID: courseID, TutorID: tutorID, StudentID: studentUUID, IsActive: true}
	courseRepo.On("GetByID", mock.Anything, courseID, tutorID).Return(individual, nil)

	result, err := svc.AddBulk(context.Background(), courseID, models.EnrollStudentsBulkRequest{StudentIDs: []string{"student-uuid-1"}}, tutorID)

	assert.ErrorIs(t, err, service.ErrForbidden)
	assert.Empty(t, result)
	repo.AssertNotCalled(t, "AddBulk")
}

func TestEnrollmentAddBulk_CourseNotFound(t *testing.T) {
	repo := new(mockEnrollmentRepo)
	courseRepo := new(mockCourseRepo)
	svc := service.NewEnrollmentService(repo, courseRepo, new(mockStudentRepo))

	courseRepo.On("GetByID", mock.Anything, courseID, tutorID).Return(models.Course{}, errors.New("not found"))

	result, err := svc.AddBulk(context.Background(), courseID, models.EnrollStudentsBulkRequest{StudentIDs: []string{"student-uuid-1"}}, tutorID)

	assert.ErrorIs(t, err, service.ErrNotFound)
	assert.Empty(t, result)
	repo.AssertNotCalled(t, "AddBulk")
}
