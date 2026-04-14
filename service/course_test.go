package service_test

import (
	"context"
	"errors"
	"testing"
	"time"
	"tutorgo/models"
	"tutorgo/service"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// mockCourseRepo is shared across service tests (payment_test.go, lesson_test.go)
type mockCourseRepo struct{ mock.Mock }

func (m *mockCourseRepo) Create(ctx context.Context, req models.CreateCourseRequest, tutorID string) (models.Course, error) {
	args := m.Called(ctx, req, tutorID)
	return args.Get(0).(models.Course), args.Error(1)
}
func (m *mockCourseRepo) GetAll(ctx context.Context, tutorID string) ([]models.Course, error) {
	args := m.Called(ctx, tutorID)
	return args.Get(0).([]models.Course), args.Error(1)
}
func (m *mockCourseRepo) GetByID(ctx context.Context, id string, tutorID string) (models.Course, error) {
	args := m.Called(ctx, id, tutorID)
	return args.Get(0).(models.Course), args.Error(1)
}
func (m *mockCourseRepo) Update(ctx context.Context, id string, tutorID string, req models.UpdateCourseRequest) (models.Course, error) {
	args := m.Called(ctx, id, tutorID, req)
	return args.Get(0).(models.Course), args.Error(1)
}
func (m *mockCourseRepo) Delete(ctx context.Context, id string, tutorID string) error {
	return m.Called(ctx, id, tutorID).Error(0)
}

var (
	endedAt = func() *time.Time { t := time.Date(2026, time.June, 1, 0, 0, 0, 0, time.UTC); return &t }()

	courseReq = models.CreateCourseRequest{
		StudentID:      "student-uuid-1",
		Subject:        "Mathematics",
		PricePerLesson: 5000,
		StartedAt:      time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC),
		EndedAt:        endedAt,
	}

	updateCourseReq = models.UpdateCourseRequest{
		Subject:        "Physics",
		PricePerLesson: 6000,
		StartedAt:      time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC),
		EndedAt:        endedAt,
	}

	expectedStudent = models.Student{ID: "student-uuid-1", TutorID: tutorID}
)

func newCourseSvc(courseRepo *mockCourseRepo, studentRepo *mockStudentRepo, lessonRepo *mockLessonRepo) service.CourseService {
	return service.NewCourseService(courseRepo, studentRepo, lessonRepo)
}

// Create

func TestCourseCreate_Success(t *testing.T) {
	courseRepo := new(mockCourseRepo)
	studentRepo := new(mockStudentRepo)
	lessonRepo := new(mockLessonRepo)
	svc := newCourseSvc(courseRepo, studentRepo, lessonRepo)

	studentRepo.On("GetByID", mock.Anything, courseReq.StudentID, tutorID).Return(expectedStudent, nil)
	courseRepo.On("Create", mock.Anything, courseReq, tutorID).Return(expectedCourse, nil)

	course, err := svc.Create(context.Background(), courseReq, tutorID)

	assert.NoError(t, err)
	assert.Equal(t, expectedCourse, course)
	studentRepo.AssertExpectations(t)
	courseRepo.AssertExpectations(t)
}

func TestCourseCreate_StudentNotFound(t *testing.T) {
	courseRepo := new(mockCourseRepo)
	studentRepo := new(mockStudentRepo)
	lessonRepo := new(mockLessonRepo)
	svc := newCourseSvc(courseRepo, studentRepo, lessonRepo)

	studentRepo.On("GetByID", mock.Anything, courseReq.StudentID, tutorID).Return(models.Student{}, errors.New("not found"))

	course, err := svc.Create(context.Background(), courseReq, tutorID)

	assert.Error(t, err)
	assert.EqualError(t, err, "student not found or access denied")
	assert.Empty(t, course)
	courseRepo.AssertNotCalled(t, "Create")
	studentRepo.AssertExpectations(t)
}

func TestCourseCreate_RepoError(t *testing.T) {
	courseRepo := new(mockCourseRepo)
	studentRepo := new(mockStudentRepo)
	lessonRepo := new(mockLessonRepo)
	svc := newCourseSvc(courseRepo, studentRepo, lessonRepo)

	studentRepo.On("GetByID", mock.Anything, courseReq.StudentID, tutorID).Return(expectedStudent, nil)
	courseRepo.On("Create", mock.Anything, courseReq, tutorID).Return(models.Course{}, errors.New("db error"))

	course, err := svc.Create(context.Background(), courseReq, tutorID)

	assert.Error(t, err)
	assert.Empty(t, course)
	studentRepo.AssertExpectations(t)
	courseRepo.AssertExpectations(t)
}

// GetAll

func TestCourseGetAll_Success(t *testing.T) {
	courseRepo := new(mockCourseRepo)
	studentRepo := new(mockStudentRepo)
	lessonRepo := new(mockLessonRepo)
	svc := newCourseSvc(courseRepo, studentRepo, lessonRepo)

	expected := []models.Course{expectedCourse}
	courseRepo.On("GetAll", mock.Anything, tutorID).Return(expected, nil)

	courses, err := svc.GetAll(context.Background(), tutorID)

	assert.NoError(t, err)
	assert.Equal(t, expected, courses)
	courseRepo.AssertExpectations(t)
}

func TestCourseGetAll_Error(t *testing.T) {
	courseRepo := new(mockCourseRepo)
	studentRepo := new(mockStudentRepo)
	lessonRepo := new(mockLessonRepo)
	svc := newCourseSvc(courseRepo, studentRepo, lessonRepo)

	courseRepo.On("GetAll", mock.Anything, tutorID).Return([]models.Course{}, errors.New("db error"))

	courses, err := svc.GetAll(context.Background(), tutorID)

	assert.Error(t, err)
	assert.Empty(t, courses)
	courseRepo.AssertExpectations(t)
}

// GetByID

func TestCourseGetByID_Success(t *testing.T) {
	courseRepo := new(mockCourseRepo)
	studentRepo := new(mockStudentRepo)
	lessonRepo := new(mockLessonRepo)
	svc := newCourseSvc(courseRepo, studentRepo, lessonRepo)

	courseRepo.On("GetByID", mock.Anything, courseID, tutorID).Return(expectedCourse, nil)

	course, err := svc.GetByID(context.Background(), courseID, tutorID)

	assert.NoError(t, err)
	assert.Equal(t, expectedCourse, course)
	courseRepo.AssertExpectations(t)
}

func TestCourseGetByID_NotFound(t *testing.T) {
	courseRepo := new(mockCourseRepo)
	studentRepo := new(mockStudentRepo)
	lessonRepo := new(mockLessonRepo)
	svc := newCourseSvc(courseRepo, studentRepo, lessonRepo)

	courseRepo.On("GetByID", mock.Anything, courseID, tutorID).Return(models.Course{}, errors.New("not found"))

	course, err := svc.GetByID(context.Background(), courseID, tutorID)

	assert.Error(t, err)
	assert.Empty(t, course)
	courseRepo.AssertExpectations(t)
}

// Update

func TestCourseUpdate_Success(t *testing.T) {
	courseRepo := new(mockCourseRepo)
	studentRepo := new(mockStudentRepo)
	lessonRepo := new(mockLessonRepo)
	svc := newCourseSvc(courseRepo, studentRepo, lessonRepo)

	updated := models.Course{ID: courseID, TutorID: tutorID, Subject: "Physics"}
	courseRepo.On("GetByID", mock.Anything, courseID, tutorID).Return(expectedCourse, nil)
	courseRepo.On("Update", mock.Anything, courseID, tutorID, updateCourseReq).Return(updated, nil)

	course, err := svc.Update(context.Background(), courseID, tutorID, updateCourseReq)

	assert.NoError(t, err)
	assert.Equal(t, updated, course)
	courseRepo.AssertExpectations(t)
}

func TestCourseUpdate_NotFound(t *testing.T) {
	courseRepo := new(mockCourseRepo)
	studentRepo := new(mockStudentRepo)
	lessonRepo := new(mockLessonRepo)
	svc := newCourseSvc(courseRepo, studentRepo, lessonRepo)

	courseRepo.On("GetByID", mock.Anything, courseID, tutorID).Return(models.Course{}, errors.New("not found"))

	course, err := svc.Update(context.Background(), courseID, tutorID, updateCourseReq)

	assert.Error(t, err)
	assert.EqualError(t, err, "course not found or access denied")
	assert.Empty(t, course)
	courseRepo.AssertNotCalled(t, "Update")
	courseRepo.AssertExpectations(t)
}

func TestCourseUpdate_RepoError(t *testing.T) {
	courseRepo := new(mockCourseRepo)
	studentRepo := new(mockStudentRepo)
	lessonRepo := new(mockLessonRepo)
	svc := newCourseSvc(courseRepo, studentRepo, lessonRepo)

	courseRepo.On("GetByID", mock.Anything, courseID, tutorID).Return(expectedCourse, nil)
	courseRepo.On("Update", mock.Anything, courseID, tutorID, updateCourseReq).Return(models.Course{}, errors.New("db error"))

	course, err := svc.Update(context.Background(), courseID, tutorID, updateCourseReq)

	assert.Error(t, err)
	assert.Empty(t, course)
	courseRepo.AssertExpectations(t)
}

// Delete

func TestCourseDelete_Success(t *testing.T) {
	courseRepo := new(mockCourseRepo)
	studentRepo := new(mockStudentRepo)
	lessonRepo := new(mockLessonRepo)
	svc := newCourseSvc(courseRepo, studentRepo, lessonRepo)

	courseRepo.On("GetByID", mock.Anything, courseID, tutorID).Return(expectedCourse, nil)
	lessonRepo.On("GetByCourse", mock.Anything, courseID).Return([]models.Lesson{}, nil)
	courseRepo.On("Delete", mock.Anything, courseID, tutorID).Return(nil)

	err := svc.Delete(context.Background(), courseID, tutorID)

	assert.NoError(t, err)
	courseRepo.AssertExpectations(t)
	lessonRepo.AssertExpectations(t)
}

func TestCourseDelete_CourseNotFound(t *testing.T) {
	courseRepo := new(mockCourseRepo)
	studentRepo := new(mockStudentRepo)
	lessonRepo := new(mockLessonRepo)
	svc := newCourseSvc(courseRepo, studentRepo, lessonRepo)

	courseRepo.On("GetByID", mock.Anything, courseID, tutorID).Return(models.Course{}, errors.New("not found"))

	err := svc.Delete(context.Background(), courseID, tutorID)

	assert.Error(t, err)
	assert.EqualError(t, err, "course not found or access denied")
	lessonRepo.AssertNotCalled(t, "GetByCourse")
	courseRepo.AssertNotCalled(t, "Delete")
	courseRepo.AssertExpectations(t)
}

func TestCourseDelete_HasLessons(t *testing.T) {
	courseRepo := new(mockCourseRepo)
	studentRepo := new(mockStudentRepo)
	lessonRepo := new(mockLessonRepo)
	svc := newCourseSvc(courseRepo, studentRepo, lessonRepo)

	courseRepo.On("GetByID", mock.Anything, courseID, tutorID).Return(expectedCourse, nil)
	lessonRepo.On("GetByCourse", mock.Anything, courseID).Return([]models.Lesson{expectedLesson}, nil)

	err := svc.Delete(context.Background(), courseID, tutorID)

	assert.Error(t, err)
	assert.EqualError(t, err, "cannot delete a course with existing lessons")
	courseRepo.AssertNotCalled(t, "Delete")
	courseRepo.AssertExpectations(t)
	lessonRepo.AssertExpectations(t)
}

func TestCourseDelete_RepoError(t *testing.T) {
	courseRepo := new(mockCourseRepo)
	studentRepo := new(mockStudentRepo)
	lessonRepo := new(mockLessonRepo)
	svc := newCourseSvc(courseRepo, studentRepo, lessonRepo)

	courseRepo.On("GetByID", mock.Anything, courseID, tutorID).Return(expectedCourse, nil)
	lessonRepo.On("GetByCourse", mock.Anything, courseID).Return([]models.Lesson{}, nil)
	courseRepo.On("Delete", mock.Anything, courseID, tutorID).Return(errors.New("db error"))

	err := svc.Delete(context.Background(), courseID, tutorID)

	assert.Error(t, err)
	courseRepo.AssertExpectations(t)
	lessonRepo.AssertExpectations(t)
}
