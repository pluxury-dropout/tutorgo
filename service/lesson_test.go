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

func (m *mockLessonRepo) AutoComplete(ctx context.Context) (int64, error) {
	args := m.Called(ctx)
	return args.Get(0).(int64), args.Error(1)
}

func (m *mockLessonRepo) DeleteByCourse(ctx context.Context, courseID string, tutorID string) error {
	return m.Called(ctx, courseID, tutorID).Error(0)
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

func (m *mockLessonRepo) EndRoomByID(ctx context.Context, lessonID string) error {
	return m.Called(ctx, lessonID).Error(0)
}

func (m *mockLessonRepo) GetRoomStatus(ctx context.Context, lessonID string) (string, error) {
	args := m.Called(ctx, lessonID)
	return args.String(0), args.Error(1)
}

func (m *mockLessonRepo) GetRanksForCourses(ctx context.Context, courseIDs []string) (map[string]map[string]int, error) {
	args := m.Called(ctx, courseIDs)
	return args.Get(0).(map[string]map[string]int), args.Error(1)
}

func (m *mockLessonRepo) GetAllLessonsForCycles(ctx context.Context, tutorID string) ([]models.CalendarLesson, error) {
	args := m.Called(ctx, tutorID)
	return args.Get(0).([]models.CalendarLesson), args.Error(1)
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
	return service.NewLessonService(lessonRepo, courseRepo, new(mockPaymentRepo), nil)
}

func newLessonSvcWithPayment(lessonRepo *mockLessonRepo, courseRepo *mockCourseRepo, paymentRepo *mockPaymentRepo) service.LessonService {
	return service.NewLessonService(lessonRepo, courseRepo, paymentRepo, nil)
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

func TestLessonCreate_ArchivedCourse(t *testing.T) {
	lessonRepo := new(mockLessonRepo)
	courseRepo := new(mockCourseRepo)
	payRepo := new(mockPaymentRepo)
	svc := service.NewLessonService(lessonRepo, courseRepo, payRepo, nil)

	archivedCourse := models.Course{ID: courseID, TutorID: tutorID, IsActive: false}
	courseRepo.On("GetByID", mock.Anything, createLessonReq.CourseID, tutorID).Return(archivedCourse, nil)

	lesson, err := svc.Create(context.Background(), createLessonReq, tutorID)

	assert.ErrorIs(t, err, service.ErrConflict)
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

	lesson, err := svc.Update(context.Background(), lessonID, updateLessonReq, tutorID, "one")

	assert.NoError(t, err)
	assert.Equal(t, updated, lesson)
	lessonRepo.AssertExpectations(t)
}

func TestLessonUpdate_NotFound(t *testing.T) {
	lessonRepo := new(mockLessonRepo)
	courseRepo := new(mockCourseRepo)
	svc := newLessonSvc(lessonRepo, courseRepo)

	lessonRepo.On("GetByIDForTutor", mock.Anything, lessonID, tutorID).Return(models.Lesson{}, errors.New("not found"))

	lesson, err := svc.Update(context.Background(), lessonID, updateLessonReq, tutorID, "one")

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

	lesson, err := svc.Update(context.Background(), lessonID, updateLessonReq, tutorID, "one")

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

	err := svc.Delete(context.Background(), lessonID, tutorID, "one")

	assert.NoError(t, err)
	lessonRepo.AssertExpectations(t)
}

func TestLessonDelete_NotFound(t *testing.T) {
	lessonRepo := new(mockLessonRepo)
	courseRepo := new(mockCourseRepo)
	svc := newLessonSvc(lessonRepo, courseRepo)

	lessonRepo.On("GetByIDForTutor", mock.Anything, lessonID, tutorID).Return(models.Lesson{}, errors.New("not found"))

	err := svc.Delete(context.Background(), lessonID, tutorID, "one")

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

	err := svc.Delete(context.Background(), lessonID, tutorID, "one")

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
	svc := service.NewLessonService(repo, nil, nil, nil)

	repo.On("EndRoom", mock.Anything, lessonID, tutorID).Return(nil)

	err := svc.EndRoom(context.Background(), lessonID, tutorID)
	assert.NoError(t, err)
	repo.AssertExpectations(t)
}
func TestEndRoom_NotFound(t *testing.T) {
	repo := new(mockLessonRepo)
	svc := service.NewLessonService(repo, nil, nil, nil)

	repo.On("EndRoom", mock.Anything, lessonID, tutorID).Return(errors.New("lesson not found"))

	err := svc.EndRoom(context.Background(), lessonID, tutorID)
	assert.Error(t, err)
	repo.AssertExpectations(t)
}

func TestGetRoomStatus_Active(t *testing.T) {
	repo := new(mockLessonRepo)
	svc := service.NewLessonService(repo, nil, nil, nil)

	repo.On("GetRoomStatus", mock.Anything, lessonID).Return("active", nil)

	status, err := svc.GetRoomStatus(context.Background(), lessonID)
	assert.NoError(t, err)
	assert.Equal(t, "active", status)
	repo.AssertExpectations(t)
}

func TestGetRoomStatus_Ended(t *testing.T) {
	repo := new(mockLessonRepo)
	svc := service.NewLessonService(repo, nil, nil, nil)

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
	paymentRepo := new(mockPaymentRepo)
	svc := newLessonSvcWithPayment(lessonRepo, courseRepo, paymentRepo)

	from := "2026-05-19T00:00:00Z"
	to := "2026-05-26T00:00:00Z"

	courseRepo.On("GetByID", mock.Anything, courseID, tutorID).Return(expectedCourse, nil)
	lessonRepo.On("GetByPeriod", mock.Anything, courseID, tutorID, from, to).Return([]models.Lesson{expectedLesson}, nil)
	lessonRepo.On("GetRanksForCourses", mock.Anything, []string{courseID}).Return(map[string]map[string]int{}, nil)
	paymentRepo.On("GetByCoursesBatch", mock.Anything, []string{courseID}).Return(map[string][]models.Payment{}, nil)

	lessons, err := svc.GetByPeriod(context.Background(), courseID, tutorID, from, to)

	assert.NoError(t, err)
	assert.Len(t, lessons, 1)
	assert.Equal(t, expectedLesson, lessons[0])
	courseRepo.AssertExpectations(t)
	lessonRepo.AssertExpectations(t)
	paymentRepo.AssertExpectations(t)
}

func TestLessonGetByPeriod_CourseNotFound(t *testing.T) {
	lessonRepo := new(mockLessonRepo)
	courseRepo := new(mockCourseRepo)
	paymentRepo := new(mockPaymentRepo)
	svc := newLessonSvcWithPayment(lessonRepo, courseRepo, paymentRepo)

	from := "2026-05-19T00:00:00Z"
	to := "2026-05-26T00:00:00Z"

	courseRepo.On("GetByID", mock.Anything, courseID, tutorID).Return(models.Course{}, errors.New("not found"))

	lessons, err := svc.GetByPeriod(context.Background(), courseID, tutorID, from, to)

	assert.ErrorIs(t, err, service.ErrNotFound)
	assert.Nil(t, lessons)
	courseRepo.AssertExpectations(t)
	lessonRepo.AssertNotCalled(t, "GetByPeriod")
}

func TestGetCurrentCycles_ReturnsActiveCycle(t *testing.T) {
	lessonRepo := new(mockLessonRepo)
	paymentRepo := new(mockPaymentRepo)
	svc := newLessonSvcWithPayment(lessonRepo, new(mockCourseRepo), paymentRepo)

	r1, r2, r3, r4 := 1, 2, 3, 4
	base := time.Date(2026, 6, 1, 10, 0, 0, 0, time.UTC)
	studentName := "Азиз"
	lessons := []models.CalendarLesson{
		{ID: "l1", CourseID: "c1", Status: "completed", Subject: "Математика", StudentName: &studentName, Rank: &r1, ScheduledAt: base},
		{ID: "l2", CourseID: "c1", Status: "completed", Subject: "Математика", StudentName: &studentName, Rank: &r2, ScheduledAt: base.AddDate(0, 0, 7)},
		{ID: "l3", CourseID: "c1", Status: "scheduled", Subject: "Математика", StudentName: &studentName, Rank: &r3, ScheduledAt: base.AddDate(0, 0, 14)},
		{ID: "l4", CourseID: "c1", Status: "scheduled", Subject: "Математика", StudentName: &studentName, Rank: &r4, ScheduledAt: base.AddDate(0, 0, 21)},
	}
	lessonRepo.On("GetAllLessonsForCycles", mock.Anything, "tutor-1").Return(lessons, nil)
	paymentRepo.On("GetByCoursesBatch", mock.Anything, mock.Anything).Return(map[string][]models.Payment{
		"c1": {{LessonsCount: 4}},
	}, nil)

	result, err := svc.GetCurrentCycles(context.Background(), "tutor-1")

	assert.NoError(t, err)
	assert.Len(t, result, 1)
	assert.Equal(t, "c1", result[0].CourseID)
	assert.Equal(t, "Математика", result[0].Subject)
	assert.Equal(t, &studentName, result[0].StudentName)
	assert.Equal(t, 2, result[0].Progress)
	assert.Equal(t, 4, result[0].CycleSize)
	assert.Equal(t, base.AddDate(0, 0, 21), result[0].LastAt)
}

func TestGetCurrentCycles_SkipsCourseWithNoScheduled(t *testing.T) {
	lessonRepo := new(mockLessonRepo)
	paymentRepo := new(mockPaymentRepo)
	svc := newLessonSvcWithPayment(lessonRepo, new(mockCourseRepo), paymentRepo)

	r1, r2 := 1, 2
	base := time.Date(2026, 6, 1, 10, 0, 0, 0, time.UTC)
	lessons := []models.CalendarLesson{
		{ID: "l1", CourseID: "c1", Status: "completed", Subject: "Физика", Rank: &r1, ScheduledAt: base},
		{ID: "l2", CourseID: "c1", Status: "completed", Subject: "Физика", Rank: &r2, ScheduledAt: base.AddDate(0, 0, 7)},
	}
	lessonRepo.On("GetAllLessonsForCycles", mock.Anything, "tutor-1").Return(lessons, nil)
	paymentRepo.On("GetByCoursesBatch", mock.Anything, mock.Anything).Return(map[string][]models.Payment{
		"c1": {{LessonsCount: 2}},
	}, nil)

	result, err := svc.GetCurrentCycles(context.Background(), "tutor-1")

	assert.NoError(t, err)
	assert.Empty(t, result)
}

func TestGetCurrentCycles_EmptyLessons(t *testing.T) {
	lessonRepo := new(mockLessonRepo)
	paymentRepo := new(mockPaymentRepo)
	svc := newLessonSvcWithPayment(lessonRepo, new(mockCourseRepo), paymentRepo)

	lessonRepo.On("GetAllLessonsForCycles", mock.Anything, "tutor-1").Return([]models.CalendarLesson{}, nil)

	result, err := svc.GetCurrentCycles(context.Background(), "tutor-1")

	assert.NoError(t, err)
	assert.Empty(t, result)
}

func TestGetCurrentCycles_MultiCycleBoundary(t *testing.T) {
	lessonRepo := new(mockLessonRepo)
	paymentRepo := new(mockPaymentRepo)
	svc := newLessonSvcWithPayment(lessonRepo, new(mockCourseRepo), paymentRepo)

	r1, r2, r3, r4 := 1, 2, 3, 4
	base := time.Date(2026, 6, 1, 10, 0, 0, 0, time.UTC)
	lessons := []models.CalendarLesson{
		{ID: "l1", CourseID: "c1", Status: "completed", Subject: "Химия", Rank: &r1, ScheduledAt: base},
		{ID: "l2", CourseID: "c1", Status: "completed", Subject: "Химия", Rank: &r2, ScheduledAt: base.AddDate(0, 0, 7)},
		{ID: "l3", CourseID: "c1", Status: "scheduled", Subject: "Химия", Rank: &r3, ScheduledAt: base.AddDate(0, 0, 14)},
		{ID: "l4", CourseID: "c1", Status: "scheduled", Subject: "Химия", Rank: &r4, ScheduledAt: base.AddDate(0, 0, 21)},
	}
	lessonRepo.On("GetAllLessonsForCycles", mock.Anything, "tutor-1").Return(lessons, nil)
	// two payments of 2 lessons each: ranks 1-2 in bucket 0, ranks 3-4 in bucket 1
	paymentRepo.On("GetByCoursesBatch", mock.Anything, mock.Anything).Return(map[string][]models.Payment{
		"c1": {{LessonsCount: 2}, {LessonsCount: 2}},
	}, nil)

	result, err := svc.GetCurrentCycles(context.Background(), "tutor-1")

	assert.NoError(t, err)
	assert.Len(t, result, 1)
	// current cycle is bucket 1 (ranks 3-4): 0 completed, cycle_size=2
	assert.Equal(t, 0, result[0].Progress)
	assert.Equal(t, 2, result[0].CycleSize)
	assert.Equal(t, base.AddDate(0, 0, 21), result[0].LastAt)
}

func TestGetCurrentCycles_SortsByLastAtDesc(t *testing.T) {
	lessonRepo := new(mockLessonRepo)
	paymentRepo := new(mockPaymentRepo)
	svc := newLessonSvcWithPayment(lessonRepo, new(mockCourseRepo), paymentRepo)

	r1, r2 := 1, 1
	base := time.Date(2026, 6, 1, 10, 0, 0, 0, time.UTC)
	lessons := []models.CalendarLesson{
		{ID: "l1", CourseID: "c1", Status: "scheduled", Subject: "Математика", Rank: &r1, ScheduledAt: base},
		{ID: "l2", CourseID: "c2", Status: "scheduled", Subject: "Физика", Rank: &r2, ScheduledAt: base.AddDate(0, 0, 7)},
	}
	lessonRepo.On("GetAllLessonsForCycles", mock.Anything, "tutor-1").Return(lessons, nil)
	paymentRepo.On("GetByCoursesBatch", mock.Anything, mock.Anything).Return(map[string][]models.Payment{
		"c1": {{LessonsCount: 1}},
		"c2": {{LessonsCount: 1}},
	}, nil)

	result, err := svc.GetCurrentCycles(context.Background(), "tutor-1")

	assert.NoError(t, err)
	assert.Len(t, result, 2)
	// c2 has later LastAt so it must come first
	assert.Equal(t, "c2", result[0].CourseID)
	assert.Equal(t, "c1", result[1].CourseID)
}

func TestGetCurrentCycles_SkipsCourseWithNoPayments(t *testing.T) {
	lessonRepo := new(mockLessonRepo)
	paymentRepo := new(mockPaymentRepo)
	svc := newLessonSvcWithPayment(lessonRepo, new(mockCourseRepo), paymentRepo)

	r1 := 1
	base := time.Date(2026, 6, 1, 10, 0, 0, 0, time.UTC)
	lessons := []models.CalendarLesson{
		{ID: "l1", CourseID: "c1", Status: "scheduled", Subject: "Биология", Rank: &r1, ScheduledAt: base},
	}
	lessonRepo.On("GetAllLessonsForCycles", mock.Anything, "tutor-1").Return(lessons, nil)
	// no payments for c1
	paymentRepo.On("GetByCoursesBatch", mock.Anything, mock.Anything).Return(map[string][]models.Payment{}, nil)

	result, err := svc.GetCurrentCycles(context.Background(), "tutor-1")

	assert.NoError(t, err)
	assert.Empty(t, result)
}

// Неявное создание курса: урок ставится по паре «ученик + предмет»,
// course_id клиент не знает и не присылает.

var implicitStudentID = "student-uuid-1"

func TestLessonCreate_ImplicitCourse(t *testing.T) {
	lessonRepo := new(mockLessonRepo)
	courseRepo := new(mockCourseRepo)
	svc := newLessonSvc(lessonRepo, courseRepo)

	req := models.CreateLessonRequest{
		StudentID:       implicitStudentID,
		Subject:         "Математика",
		ScheduledAt:     scheduledAt,
		DurationMinutes: 60,
		Notes:           "first lesson",
	}
	implicit := models.Course{ID: courseID, TutorID: tutorID, Subject: "Математика", IsActive: true}

	courseRepo.On("GetOrCreateIndividual", mock.Anything, tutorID, implicitStudentID, "Математика", scheduledAt).
		Return(implicit, nil)
	lessonRepo.On("Create", mock.Anything, mock.MatchedBy(func(r models.CreateLessonRequest) bool {
		return r.CourseID == courseID
	})).Return(expectedLesson, nil)

	lesson, err := svc.Create(context.Background(), req, tutorID)

	assert.NoError(t, err)
	assert.Equal(t, expectedLesson, lesson)
	courseRepo.AssertNotCalled(t, "GetByID")
	courseRepo.AssertExpectations(t)
	lessonRepo.AssertExpectations(t)
}

func TestLessonCreate_ForeignStudent(t *testing.T) {
	lessonRepo := new(mockLessonRepo)
	courseRepo := new(mockCourseRepo)
	svc := newLessonSvc(lessonRepo, courseRepo)

	req := models.CreateLessonRequest{
		StudentID:       implicitStudentID,
		Subject:         "Математика",
		ScheduledAt:     scheduledAt,
		DurationMinutes: 60,
	}
	courseRepo.On("GetOrCreateIndividual", mock.Anything, tutorID, implicitStudentID, "Математика", scheduledAt).
		Return(models.Course{}, errors.New("no rows in result set"))

	lesson, err := svc.Create(context.Background(), req, tutorID)

	assert.ErrorIs(t, err, service.ErrNotFound)
	assert.Empty(t, lesson)
	lessonRepo.AssertNotCalled(t, "Create")
	courseRepo.AssertExpectations(t)
}

func TestLessonCreate_NeitherCourseNorStudent(t *testing.T) {
	lessonRepo := new(mockLessonRepo)
	courseRepo := new(mockCourseRepo)
	svc := newLessonSvc(lessonRepo, courseRepo)

	req := models.CreateLessonRequest{ScheduledAt: scheduledAt, DurationMinutes: 60}

	lesson, err := svc.Create(context.Background(), req, tutorID)

	assert.ErrorIs(t, err, service.ErrBadRequest)
	assert.Empty(t, lesson)
	lessonRepo.AssertNotCalled(t, "Create")
	courseRepo.AssertNotCalled(t, "GetOrCreateIndividual")
	courseRepo.AssertNotCalled(t, "GetByID")
}

