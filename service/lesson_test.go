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

func (m *mockLessonRepo) StartRoom(ctx context.Context, lessonID string, tutorID string) error {
	return m.Called(ctx, lessonID, tutorID).Error(0)
}

func (m *mockLessonRepo) GetByCoursePaged(ctx context.Context, courseID string, p models.Pagination) ([]models.Lesson, int, error) {
	return nil, 0, nil
}

func (m *mockLessonRepo) GetByPeriod(ctx context.Context, courseID string, tutorID string, from string, to string) ([]models.Lesson, error) {
	args := m.Called(ctx, courseID, tutorID, from, to)
	return args.Get(0).([]models.Lesson), args.Error(1)
}

func (m *mockLessonRepo) EndRoom(ctx context.Context, lessonID string, tutorID string) error {
	return m.Called(ctx, lessonID, tutorID).Error(0)
}

func (m *mockLessonRepo) GetRoomStatus(ctx context.Context, lessonID string) (string, error) {
	args := m.Called(ctx, lessonID)
	return args.String(0), args.Error(1)
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

	assert.ErrorIs(t, err, service.ErrNotFound)
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

	assert.ErrorIs(t, err, service.ErrNotFound)
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

	assert.ErrorIs(t, err, service.ErrNotFound)
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

	assert.ErrorIs(t, err, service.ErrNotFound)
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

	assert.ErrorIs(t, err, service.ErrNotFound)
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

// ExistsPublic

func TestLessonExistsPublic_Success(t *testing.T) {
	lessonRepo := new(mockLessonRepo)
	courseRepo := new(mockCourseRepo)
	svc := newLessonSvc(lessonRepo, courseRepo)

	lessonRepo.On("ExistsPublic", mock.Anything, lessonID).Return(nil)

	err := svc.ExistsPublic(context.Background(), lessonID)

	assert.NoError(t, err)
	lessonRepo.AssertExpectations(t)
}

func TestLessonExistsPublic_Error(t *testing.T) {
	lessonRepo := new(mockLessonRepo)
	courseRepo := new(mockCourseRepo)
	svc := newLessonSvc(lessonRepo, courseRepo)

	lessonRepo.On("ExistsPublic", mock.Anything, lessonID).Return(errors.New("not found"))

	err := svc.ExistsPublic(context.Background(), lessonID)

	assert.Error(t, err)
	lessonRepo.AssertExpectations(t)
}

// StartRoom

func TestLessonStartRoom_Success(t *testing.T) {
	lessonRepo := new(mockLessonRepo)
	courseRepo := new(mockCourseRepo)
	svc := newLessonSvc(lessonRepo, courseRepo)

	lessonRepo.On("StartRoom", mock.Anything, lessonID, tutorID).Return(nil)

	err := svc.StartRoom(context.Background(), lessonID, tutorID)

	assert.NoError(t, err)
	lessonRepo.AssertExpectations(t)
}

func TestLessonStartRoom_NotFound(t *testing.T) {
	lessonRepo := new(mockLessonRepo)
	courseRepo := new(mockCourseRepo)
	svc := newLessonSvc(lessonRepo, courseRepo)

	lessonRepo.On("StartRoom", mock.Anything, lessonID, tutorID).Return(errors.New("lesson not found"))

	err := svc.StartRoom(context.Background(), lessonID, tutorID)

	assert.Error(t, err)
	lessonRepo.AssertExpectations(t)
}
func TestEndRoom_Success(t *testing.T) {
	repo := new(mockLessonRepo)
	svc := service.NewLessonService(repo, nil)

	repo.On("EndRoom", mock.Anything, lessonID, tutorID).Return(nil)

	err := svc.EndRoom(context.Background(), lessonID, tutorID)
	assert.NoError(t, err)
	repo.AssertExpectations(t)
}
func TestEndRoom_NotFound(t *testing.T) {
	repo := new(mockLessonRepo)
	svc := service.NewLessonService(repo, nil)

	repo.On("EndRoom", mock.Anything, lessonID, tutorID).Return(errors.New("lesson not found"))

	err := svc.EndRoom(context.Background(), lessonID, tutorID)
	assert.Error(t, err)
	repo.AssertExpectations(t)
}

func TestGetRoomStatus_Active(t *testing.T) {
	repo := new(mockLessonRepo)
	svc := service.NewLessonService(repo, nil)

	repo.On("GetRoomStatus", mock.Anything, lessonID).Return("active", nil)

	status, err := svc.GetRoomStatus(context.Background(), lessonID)
	assert.NoError(t, err)
	assert.Equal(t, "active", status)
	repo.AssertExpectations(t)
}

func TestGetRoomStatus_Ended(t *testing.T) {
	repo := new(mockLessonRepo)
	svc := service.NewLessonService(repo, nil)

	repo.On("GetRoomStatus", mock.Anything, lessonID).Return("ended", nil)

	status, err := svc.GetRoomStatus(context.Background(), lessonID)
	assert.NoError(t, err)
	assert.Equal(t, "ended", status)
	repo.AssertExpectations(t)
}

// GetByPeriod

func TestLessonGetByPeriod_Success(t *testing.T) {
	lessonRepo := new(mockLessonRepo)
	courseRepo := new(mockCourseRepo)
	svc := newLessonSvc(lessonRepo, courseRepo)

	from := "2026-05-19T00:00:00Z"
	to := "2026-05-26T00:00:00Z"

	courseRepo.On("GetByID", mock.Anything, courseID, tutorID).Return(expectedCourse, nil)
	lessonRepo.On("GetByPeriod", mock.Anything, courseID, tutorID, from, to).Return([]models.Lesson{expectedLesson}, nil)

	lessons, err := svc.GetByPeriod(context.Background(), courseID, tutorID, from, to)

	assert.NoError(t, err)
	assert.Len(t, lessons, 1)
	assert.Equal(t, expectedLesson, lessons[0])
	courseRepo.AssertExpectations(t)
	lessonRepo.AssertExpectations(t)
}

func TestLessonGetByPeriod_CourseNotFound(t *testing.T) {
	lessonRepo := new(mockLessonRepo)
	courseRepo := new(mockCourseRepo)
	svc := newLessonSvc(lessonRepo, courseRepo)

	from := "2026-05-19T00:00:00Z"
	to := "2026-05-26T00:00:00Z"

	courseRepo.On("GetByID", mock.Anything, courseID, tutorID).Return(models.Course{}, errors.New("not found"))

	lessons, err := svc.GetByPeriod(context.Background(), courseID, tutorID, from, to)

	assert.ErrorIs(t, err, service.ErrNotFound)
	assert.Nil(t, lessons)
	courseRepo.AssertExpectations(t)
	lessonRepo.AssertNotCalled(t, "GetByPeriod")
}
