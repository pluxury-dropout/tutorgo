package handlers_test

import (
	"context"
	"log/slog"
	"net/http"
	"testing"

	"tutorgo/handlers"
	"tutorgo/models"
	"tutorgo/service"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// --- Mock: LessonTaskService ---

type mockLessonTaskService struct{ mock.Mock }

func (m *mockLessonTaskService) ListForTutor(ctx context.Context, lessonID, tutorID string) ([]models.LessonTask, error) {
	args := m.Called(ctx, lessonID, tutorID)
	return args.Get(0).([]models.LessonTask), args.Error(1)
}
func (m *mockLessonTaskService) Create(ctx context.Context, lessonID, tutorID string, req models.CreateLessonTaskRequest) (models.LessonTask, error) {
	args := m.Called(ctx, lessonID, tutorID, req)
	return args.Get(0).(models.LessonTask), args.Error(1)
}
func (m *mockLessonTaskService) Update(ctx context.Context, taskID, tutorID string, req models.UpdateLessonTaskRequest) (models.LessonTask, error) {
	args := m.Called(ctx, taskID, tutorID, req)
	return args.Get(0).(models.LessonTask), args.Error(1)
}
func (m *mockLessonTaskService) Delete(ctx context.Context, taskID, tutorID string) error {
	return m.Called(ctx, taskID, tutorID).Error(0)
}
func (m *mockLessonTaskService) ListForStudent(ctx context.Context, lessonID, studentID string) ([]models.LessonTask, error) {
	args := m.Called(ctx, lessonID, studentID)
	return args.Get(0).([]models.LessonTask), args.Error(1)
}
func (m *mockLessonTaskService) SetDone(ctx context.Context, taskID, studentID string, done bool) error {
	return m.Called(ctx, taskID, studentID, done).Error(0)
}

func newLessonTaskRouter(svc *mockLessonTaskService, mws ...gin.HandlerFunc) *gin.Engine {
	r := gin.New()
	h := handlers.NewLessonTaskHandler(svc, slog.Default())
	grp := r.Group("/")
	for _, mw := range mws {
		grp.Use(mw)
	}
	grp.GET("/lessons/:id/tasks", h.ListForTutor)
	grp.POST("/lessons/:id/tasks", h.Create)
	grp.PUT("/lesson-tasks/:id", h.Update)
	grp.DELETE("/lesson-tasks/:id", h.Delete)
	grp.GET("/student/lessons/:id/tasks", h.ListForStudent)
	grp.PATCH("/student/lesson-tasks/:id", h.StudentSetDone)
	return r
}

func TestLessonTaskStudentSetDone_NoStudentID(t *testing.T) {
	svc := new(mockLessonTaskService)
	r := newLessonTaskRouter(svc) // no studentID middleware

	w := makeRequest(t, r, http.MethodPatch, "/student/lesson-tasks/task-1", models.SetLessonTaskDoneRequest{Done: true})

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	svc.AssertNotCalled(t, "SetDone")
}

func TestLessonTaskStudentSetDone_Forbidden(t *testing.T) {
	svc := new(mockLessonTaskService)
	r := newLessonTaskRouter(svc, withStudentID(testStudentID))

	svc.On("SetDone", mock.Anything, "task-1", testStudentID, true).
		Return(service.ErrForbidden)

	w := makeRequest(t, r, http.MethodPatch, "/student/lesson-tasks/task-1", models.SetLessonTaskDoneRequest{Done: true})

	assert.Equal(t, http.StatusForbidden, w.Code)
	svc.AssertExpectations(t)
}

func TestLessonTaskStudentSetDone_Success(t *testing.T) {
	svc := new(mockLessonTaskService)
	r := newLessonTaskRouter(svc, withStudentID(testStudentID))

	svc.On("SetDone", mock.Anything, "task-1", testStudentID, true).Return(nil)

	w := makeRequest(t, r, http.MethodPatch, "/student/lesson-tasks/task-1", models.SetLessonTaskDoneRequest{Done: true})

	assert.Equal(t, http.StatusNoContent, w.Code)
	svc.AssertExpectations(t)
}

func TestLessonTaskTutorCreate_InvalidBody(t *testing.T) {
	svc := new(mockLessonTaskService)
	r := newLessonTaskRouter(svc, withTutorID(testTutorID))

	// empty title fails validation (required)
	w := makeRequest(t, r, http.MethodPost, "/lessons/lesson-1/tasks", models.CreateLessonTaskRequest{Title: ""})

	assert.Equal(t, http.StatusBadRequest, w.Code)
	svc.AssertNotCalled(t, "Create")
}
