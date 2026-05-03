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

// mock

type mockLessonRepo struct {
	mock.Mock
}

func (m *mockLessonRepo) Create(ctx context.Context, req models.CreateLessonRequest) (models.Lesson, error) {
	args := m.Called(ctx, req)
	return args.Get(0).(models.Lesson), args.Error(1)
}

func (m *mockLessonRepo) GetByCourse(ctx context.Context, courseID string) ([]models.Lesson, error) {
	args := m.Called(ctx, courseID)
	return args.Get(0).([]models.Lesson), args.Error(1)
}

func (m *mockLessonRepo) GetByID(ctx context.Context, id string) (models.Lesson, error) {
	args := m.Called(ctx, id)
	return args.Get(0).(models.Lesson), args.Error(1)
}

func (m *mockLessonRepo) GetByIDForTutor(ctx context.Context, id string, tutorID string) (models.Lesson, error) {
	args := m.Called(ctx, id, tutorID)
	return args.Get(0).(models.Lesson), args.Error(1)
}

func (m *mockLessonRepo) Update(ctx context.Context, id string, req models.UpdateLessonRequest) (models.Lesson, error) {
	args := m.Called(ctx, id, req)
	return args.Get(0).(models.Lesson), args.Error(1)
}

func (m *mockLessonRepo) Delete(ctx context.Context, id string) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *mockLessonRepo) GetCalendar(ctx context.Context, tutorID string, from string, to string) ([]models.CalendarLesson, error) {
	args := m.Called(ctx, tutorID, from, to)
	return args.Get(0).([]models.CalendarLesson), args.Error(1)
}

func (m *mockLessonRepo) CreateBulk(ctx context.Context, req models.CreateBulkLessonRequest) ([]models.Lesson, error) {
	args := m.Called(ctx, req)
	return args.Get(0).([]models.Lesson), args.Error(1)
}

func (m *mockLessonRepo) AutoComplete(ctx context.Context) (int64, error) {
	args := m.Called(ctx)
	return args.Get(0).(int64), args.Error(1)
}

func (m *mockLessonRepo) DeleteByCourse(ctx context.Context, courseID string, tutorID string) error {
	return m.Called(ctx, courseID, tutorID).Error(0)
}

func (m *mockLessonRepo) DeleteSeries(ctx context.Context, seriesID string, tutorID string, fromDate *string) error {
	return m.Called(ctx, seriesID, tutorID, fromDate).Error(0)
}

func (m *mockLessonRepo) UpdateSeries(ctx context.Context, seriesID string, tutorID string, req models.UpdateSeriesRequest) error {
	return m.Called(ctx, seriesID, tutorID, req).Error(0)
}

func (m *mockLessonRepo) ExistsPublic(ctx context.Context, id string) error {
	return m.Called(ctx, id).Error(0)
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

	courseRepo.On("GetByID", mock.Anything, courseID, tutorID).Return(expectedCourse, nil)
	lessonRepo.On("Create", mock.Anything, createLessonReq).Return(expectedLesson, nil)

	lesson, err := svc.Create(context.Background(), createLessonReq, tutorID)

	assert.NoError(t, err)
	assert.Equal(t, expectedLesson, lesson)
	courseRepo.AssertExpectations(t)
	lessonRepo.AssertExpectations(t)
}

func TestLessonCreate_CourseNotFound(t *testing.T) {
	lessonRepo := new(mockLessonRepo)
	courseRepo := new(mockCourseRepo)
	svc := newLessonSvc(lessonRepo, courseRepo)

	courseRepo.On("GetByID", mock.Anything, courseID, tutorID).Return(models.Course{}, errors.New("not found"))

	lesson, err := svc.Create(context.Background(), createLessonReq, tutorID)

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

	courseRepo.On("GetByID", mock.Anything, courseID, tutorID).Return(expectedCourse, nil)
	lessonRepo.On("Create", mock.Anything, createLessonReq).Return(models.Lesson{}, errors.New("db error"))

	lesson, err := svc.Create(context.Background(), createLessonReq, tutorID)

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
	courseRepo.On("GetByID", mock.Anything, courseID, tutorID).Return(expectedCourse, nil)
	lessonRepo.On("GetByCourse", mock.Anything, courseID).Return(expected, nil)

	lessons, err := svc.GetByCourse(context.Background(), courseID, tutorID)

	assert.NoError(t, err)
	assert.Equal(t, expected, lessons)
	courseRepo.AssertExpectations(t)
	lessonRepo.AssertExpectations(t)
}

func TestLessonGetByCourse_CourseNotFound(t *testing.T) {
	lessonRepo := new(mockLessonRepo)
	courseRepo := new(mockCourseRepo)
	svc := newLessonSvc(lessonRepo, courseRepo)

	courseRepo.On("GetByID", mock.Anything, courseID, tutorID).Return(models.Course{}, errors.New("not found"))

	lessons, err := svc.GetByCourse(context.Background(), courseID, tutorID)

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

	lessonRepo.On("GetByIDForTutor", mock.Anything, lessonID, tutorID).Return(expectedLesson, nil)

	lesson, err := svc.GetByID(context.Background(), lessonID, tutorID)

	assert.NoError(t, err)
	assert.Equal(t, expectedLesson, lesson)
	lessonRepo.AssertExpectations(t)
}

func TestLessonGetByID_NotFound(t *testing.T) {
	lessonRepo := new(mockLessonRepo)
	courseRepo := new(mockCourseRepo)
	svc := newLessonSvc(lessonRepo, courseRepo)

	lessonRepo.On("GetByIDForTutor", mock.Anything, lessonID, tutorID).Return(models.Lesson{}, errors.New("not found"))

	lesson, err := svc.GetByID(context.Background(), lessonID, tutorID)

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

	lessonRepo.On("GetByIDForTutor", mock.Anything, lessonID, tutorID).Return(expectedLesson, nil)
	lessonRepo.On("Update", mock.Anything, lessonID, updateLessonReq).Return(updated, nil)

	lesson, err := svc.Update(context.Background(), lessonID, updateLessonReq, tutorID)

	assert.NoError(t, err)
	assert.Equal(t, updated, lesson)
	lessonRepo.AssertExpectations(t)
}

func TestLessonUpdate_NotFound(t *testing.T) {
	lessonRepo := new(mockLessonRepo)
	courseRepo := new(mockCourseRepo)
	svc := newLessonSvc(lessonRepo, courseRepo)

	lessonRepo.On("GetByIDForTutor", mock.Anything, lessonID, tutorID).Return(models.Lesson{}, errors.New("not found"))

	lesson, err := svc.Update(context.Background(), lessonID, updateLessonReq, tutorID)

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

	lessonRepo.On("GetByIDForTutor", mock.Anything, lessonID, tutorID).Return(expectedLesson, nil)
	lessonRepo.On("Update", mock.Anything, lessonID, updateLessonReq).Return(models.Lesson{}, errors.New("db error"))

	lesson, err := svc.Update(context.Background(), lessonID, updateLessonReq, tutorID)

	assert.Error(t, err)
	assert.Empty(t, lesson)
	lessonRepo.AssertExpectations(t)
}

// Delete

func TestLessonDelete_Success(t *testing.T) {
	lessonRepo := new(mockLessonRepo)
	courseRepo := new(mockCourseRepo)
	svc := newLessonSvc(lessonRepo, courseRepo)

	lessonRepo.On("GetByIDForTutor", mock.Anything, lessonID, tutorID).Return(expectedLesson, nil)
	lessonRepo.On("Delete", mock.Anything, lessonID).Return(nil)

	err := svc.Delete(context.Background(), lessonID, tutorID)

	assert.NoError(t, err)
	lessonRepo.AssertExpectations(t)
}

func TestLessonDelete_NotFound(t *testing.T) {
	lessonRepo := new(mockLessonRepo)
	courseRepo := new(mockCourseRepo)
	svc := newLessonSvc(lessonRepo, courseRepo)

	lessonRepo.On("GetByIDForTutor", mock.Anything, lessonID, tutorID).Return(models.Lesson{}, errors.New("not found"))

	err := svc.Delete(context.Background(), lessonID, tutorID)

	assert.Error(t, err)
	assert.EqualError(t, err, "lesson not found or access denied")
	lessonRepo.AssertNotCalled(t, "Delete")
	lessonRepo.AssertExpectations(t)
}

func TestLessonDelete_RepoError(t *testing.T) {
	lessonRepo := new(mockLessonRepo)
	courseRepo := new(mockCourseRepo)
	svc := newLessonSvc(lessonRepo, courseRepo)

	lessonRepo.On("GetByIDForTutor", mock.Anything, lessonID, tutorID).Return(expectedLesson, nil)
	lessonRepo.On("Delete", mock.Anything, lessonID).Return(errors.New("db error"))

	err := svc.Delete(context.Background(), lessonID, tutorID)

	assert.Error(t, err)
	lessonRepo.AssertExpectations(t)
}
