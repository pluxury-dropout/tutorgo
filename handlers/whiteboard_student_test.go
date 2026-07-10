package handlers_test

import (
	"errors"
	"net/http"
	"testing"
	"tutorgo/handlers"
	"tutorgo/models"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func newWhiteboardStudentRouter(svc *mockWhiteboardService, studentSvc *mockStudentService) *gin.Engine {
	r := gin.New()
	h := handlers.NewWhiteboardHandler(svc, nil, nil, nil, studentSvc)
	student := r.Group("/")
	student.Use(withStudentID(testStudentID))
	student.GET("/student/lessons/:id/board-token", h.StudentBoardToken)
	return r
}

func TestStudentBoardToken_NotEnrolled(t *testing.T) {
	svc := new(mockWhiteboardService)
	studentSvc := new(mockStudentService)
	r := newWhiteboardStudentRouter(svc, studentSvc)

	studentSvc.On("EnrolledInLesson", mock.Anything, testStudentID, testLessonID).Return(false, nil)

	w := makeRequest(t, r, http.MethodGet, "/student/lessons/"+testLessonID+"/board-token", nil)

	assert.Equal(t, http.StatusForbidden, w.Code)
	studentSvc.AssertExpectations(t)
	svc.AssertNotCalled(t, "GetOrCreateBoard", mock.Anything, mock.Anything, mock.Anything)
}

func TestStudentBoardToken_EnrollmentCheckError(t *testing.T) {
	svc := new(mockWhiteboardService)
	studentSvc := new(mockStudentService)
	r := newWhiteboardStudentRouter(svc, studentSvc)

	studentSvc.On("EnrolledInLesson", mock.Anything, testStudentID, testLessonID).Return(false, errors.New("db error"))

	w := makeRequest(t, r, http.MethodGet, "/student/lessons/"+testLessonID+"/board-token", nil)

	assert.Equal(t, http.StatusForbidden, w.Code)
	studentSvc.AssertExpectations(t)
}

func TestStudentBoardToken_Success(t *testing.T) {
	svc := new(mockWhiteboardService)
	studentSvc := new(mockStudentService)
	r := newWhiteboardStudentRouter(svc, studentSvc)

	const boardID = "66666666-6666-6666-6666-666666666666"
	const pageID = "77777777-7777-7777-7777-777777777777"
	const inviteID = "88888888-8888-8888-8888-888888888888"

	studentSvc.On("EnrolledInLesson", mock.Anything, testStudentID, testLessonID).Return(true, nil)
	studentSvc.On("CourseAndTutorForLesson", mock.Anything, testLessonID).Return(testCourseID, testTutorID, nil)
	svc.On("GetOrCreateBoard", mock.Anything, testCourseID, testTutorID).Return(models.BoardWithPages{
		Board: models.Board{ID: boardID, CourseID: testCourseID, TutorID: testTutorID},
		Pages: []models.BoardPage{{ID: pageID, BoardID: boardID}},
	}, nil)
	svc.On("CreateInvite", mock.Anything, boardID, testTutorID).Return(models.BoardInvite{ID: inviteID, BoardID: boardID}, nil)

	w := makeRequest(t, r, http.MethodGet, "/student/lessons/"+testLessonID+"/board-token", nil)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]string
	decodeJSON(t, w, &resp)
	assert.Equal(t, inviteID, resp["invite_token"])
	assert.Equal(t, pageID, resp["page_id"])
	studentSvc.AssertExpectations(t)
	svc.AssertExpectations(t)
}
