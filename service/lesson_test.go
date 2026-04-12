package service_test

import (
	"errors"
	"testing"
	"time"
	"tutorgo/models"
	"tutorgo/service"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// mock

type mockLessonRepo struct {
	mock.Mock
}

func (m *mockLessonRepo) Create(req models.CreateLessonRequest) (models.Lesson, error) {
	args := m.Called(req)
	return args.Get(0).(models.Lesson), args.Error(1)
}

func (m *mockLessonRepo) GetByCourse(courseID string) ([]models.Lesson, error) {
	args := m.Called(courseID)
	return args.Get(0).([]models.Lesson), args.Error(1)
}

func (m *mockLessonRepo) GetByID(id string) (models.Lesson, error) {
	args := m.Called(id)
	return args.Get(0).(models.Lesson), args.Error(1)
}

func (m *mockLessonRepo) GetByIDForTutor(id string, tutorID string) (models.Lesson, error) {
	args := m.Called(id, tutorID)
	return args.Get(0).(models.Lesson), args.Error(1)
}

func (m *mockLessonRepo) Update(id string, req models.UpdateLessonRequest) (models.Lesson, error) {
	args := m.Called(id, req)
	return args.Get(0).(models.Lesson), args.Error(1)
}

func (m *mockLessonRepo) Delete(id string) error {
	args := m.Called(id)
	return args.Error(0)
}

// fixtures

var (
	lessonID = "lesson-uuid-1"

	scheduledAt = time.Date(2026, time.May, 1, 10, 0, 0, 0, time.UTC)

	createLessonReq = models.CreateLessonRequest{
		CourseID:        courseID,
		ScheduledAt:     scheduledAt,
		DurationMinutes: 60,
		Notes:           "first lesson",
	}

	updateLessonReq = models.UpdateLessonRequest{
		ScheduledAt:     scheduledAt,
		DurationMinutes: 90,
		Status:          "completed",
		Notes:           "done",
	}

	expectedLesson = models.Lesson{
		ID:              lessonID,
		CourseID:        courseID,
		ScheduledAt:     scheduledAt,
		DurationMinutes: 60,
		Status:          "scheduled",
		Notes:           "first lesson",
	}
)

func newLessonSvc(lessonRepo *mockLessonRepo, courseRepo *mockCourseRepo) service.LessonService {
	return service.NewLessonService(lessonRepo, courseRepo)
}

// Create

func TestLessonCreate_Success(t *testing.T) {
	lessonRepo := new(mockLessonRepo)
	courseRepo := new(mockCourseRepo)
	svc := newLessonSvc(lessonRepo, courseRepo)

	courseRepo.On("GetByID", courseID, tutorID).Return(expectedCourse, nil)
	lessonRepo.On("Create", createLessonReq).Return(expectedLesson, nil)

	lesson, err := svc.Create(createLessonReq, tutorID)

	assert.NoError(t, err)
	assert.Equal(t, expectedLesson, lesson)
	courseRepo.AssertExpectations(t)
	lessonRepo.AssertExpectations(t)
}

func TestLessonCreate_CourseNotFound(t *testing.T) {
	lessonRepo := new(mockLessonRepo)
	courseRepo := new(mockCourseRepo)
	svc := newLessonSvc(lessonRepo, courseRepo)

	courseRepo.On("GetByID", courseID, tutorID).Return(models.Course{}, errors.New("not found"))

	lesson, err := svc.Create(createLessonReq, tutorID)

	assert.Error(t, err)
	assert.EqualError(t, err, "course not found or access denied")
	assert.Empty(t, lesson)
	lessonRepo.AssertNotCalled(t, "Create")
	courseRepo.AssertExpectations(t)
}

func TestLessonCreate_RepoError(t *testing.T) {
	lessonRepo := new(mockLessonRepo)
	courseRepo := new(mockCourseRepo)
	svc := newLessonSvc(lessonRepo, courseRepo)

	courseRepo.On("GetByID", courseID, tutorID).Return(expectedCourse, nil)
	lessonRepo.On("Create", createLessonReq).Return(models.Lesson{}, errors.New("db error"))

	lesson, err := svc.Create(createLessonReq, tutorID)

	assert.Error(t, err)
	assert.Empty(t, lesson)
	courseRepo.AssertExpectations(t)
	lessonRepo.AssertExpectations(t)
}

// GetByCourse

func TestLessonGetByCourse_Success(t *testing.T) {
	lessonRepo := new(mockLessonRepo)
	courseRepo := new(mockCourseRepo)
	svc := newLessonSvc(lessonRepo, courseRepo)

	expected := []models.Lesson{expectedLesson}
	courseRepo.On("GetByID", courseID, tutorID).Return(expectedCourse, nil)
	lessonRepo.On("GetByCourse", courseID).Return(expected, nil)

	lessons, err := svc.GetByCourse(courseID, tutorID)

	assert.NoError(t, err)
	assert.Equal(t, expected, lessons)
	courseRepo.AssertExpectations(t)
	lessonRepo.AssertExpectations(t)
}

func TestLessonGetByCourse_CourseNotFound(t *testing.T) {
	lessonRepo := new(mockLessonRepo)
	courseRepo := new(mockCourseRepo)
	svc := newLessonSvc(lessonRepo, courseRepo)

	courseRepo.On("GetByID", courseID, tutorID).Return(models.Course{}, errors.New("not found"))

	lessons, err := svc.GetByCourse(courseID, tutorID)

	assert.Error(t, err)
	assert.EqualError(t, err, "course not found or access denied")
	assert.Nil(t, lessons)
	lessonRepo.AssertNotCalled(t, "GetByCourse")
	courseRepo.AssertExpectations(t)
}

// GetByID

func TestLessonGetByID_Success(t *testing.T) {
	lessonRepo := new(mockLessonRepo)
	courseRepo := new(mockCourseRepo)
	svc := newLessonSvc(lessonRepo, courseRepo)

	lessonRepo.On("GetByIDForTutor", lessonID, tutorID).Return(expectedLesson, nil)

	lesson, err := svc.GetByID(lessonID, tutorID)

	assert.NoError(t, err)
	assert.Equal(t, expectedLesson, lesson)
	lessonRepo.AssertExpectations(t)
}

func TestLessonGetByID_NotFound(t *testing.T) {
	lessonRepo := new(mockLessonRepo)
	courseRepo := new(mockCourseRepo)
	svc := newLessonSvc(lessonRepo, courseRepo)

	lessonRepo.On("GetByIDForTutor", lessonID, tutorID).Return(models.Lesson{}, errors.New("not found"))

	lesson, err := svc.GetByID(lessonID, tutorID)

	assert.Error(t, err)
	assert.EqualError(t, err, "lesson not found or access denied")
	assert.Empty(t, lesson)
	courseRepo.AssertNotCalled(t, "GetByID")
	lessonRepo.AssertExpectations(t)
}

// Update

func TestLessonUpdate_Success(t *testing.T) {
	lessonRepo := new(mockLessonRepo)
	courseRepo := new(mockCourseRepo)
	svc := newLessonSvc(lessonRepo, courseRepo)

	updated := models.Lesson{
		ID:              lessonID,
		CourseID:        courseID,
		ScheduledAt:     scheduledAt,
		DurationMinutes: 90,
		Status:          "completed",
		Notes:           "done",
	}

	lessonRepo.On("GetByIDForTutor", lessonID, tutorID).Return(expectedLesson, nil)
	lessonRepo.On("Update", lessonID, updateLessonReq).Return(updated, nil)

	lesson, err := svc.Update(lessonID, updateLessonReq, tutorID)

	assert.NoError(t, err)
	assert.Equal(t, updated, lesson)
	lessonRepo.AssertExpectations(t)
}

func TestLessonUpdate_NotFound(t *testing.T) {
	lessonRepo := new(mockLessonRepo)
	courseRepo := new(mockCourseRepo)
	svc := newLessonSvc(lessonRepo, courseRepo)

	lessonRepo.On("GetByIDForTutor", lessonID, tutorID).Return(models.Lesson{}, errors.New("not found"))

	lesson, err := svc.Update(lessonID, updateLessonReq, tutorID)

	assert.Error(t, err)
	assert.EqualError(t, err, "lesson not found or access denied")
	assert.Empty(t, lesson)
	lessonRepo.AssertNotCalled(t, "Update")
	lessonRepo.AssertExpectations(t)
}

func TestLessonUpdate_RepoError(t *testing.T) {
	lessonRepo := new(mockLessonRepo)
	courseRepo := new(mockCourseRepo)
	svc := newLessonSvc(lessonRepo, courseRepo)

	lessonRepo.On("GetByIDForTutor", lessonID, tutorID).Return(expectedLesson, nil)
	lessonRepo.On("Update", lessonID, updateLessonReq).Return(models.Lesson{}, errors.New("db error"))

	lesson, err := svc.Update(lessonID, updateLessonReq, tutorID)

	assert.Error(t, err)
	assert.Empty(t, lesson)
	lessonRepo.AssertExpectations(t)
}

// Delete

func TestLessonDelete_Success(t *testing.T) {
	lessonRepo := new(mockLessonRepo)
	courseRepo := new(mockCourseRepo)
	svc := newLessonSvc(lessonRepo, courseRepo)

	lessonRepo.On("GetByIDForTutor", lessonID, tutorID).Return(expectedLesson, nil)
	lessonRepo.On("Delete", lessonID).Return(nil)

	err := svc.Delete(lessonID, tutorID)

	assert.NoError(t, err)
	lessonRepo.AssertExpectations(t)
}

func TestLessonDelete_NotFound(t *testing.T) {
	lessonRepo := new(mockLessonRepo)
	courseRepo := new(mockCourseRepo)
	svc := newLessonSvc(lessonRepo, courseRepo)

	lessonRepo.On("GetByIDForTutor", lessonID, tutorID).Return(models.Lesson{}, errors.New("not found"))

	err := svc.Delete(lessonID, tutorID)

	assert.Error(t, err)
	assert.EqualError(t, err, "lesson not found or access denied")
	lessonRepo.AssertNotCalled(t, "Delete")
	lessonRepo.AssertExpectations(t)
}

func TestLessonDelete_RepoError(t *testing.T) {
	lessonRepo := new(mockLessonRepo)
	courseRepo := new(mockCourseRepo)
	svc := newLessonSvc(lessonRepo, courseRepo)

	lessonRepo.On("GetByIDForTutor", lessonID, tutorID).Return(expectedLesson, nil)
	lessonRepo.On("Delete", lessonID).Return(errors.New("db error"))

	err := svc.Delete(lessonID, tutorID)

	assert.Error(t, err)
	lessonRepo.AssertExpectations(t)
}
