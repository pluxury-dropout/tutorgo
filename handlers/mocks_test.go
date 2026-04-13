package handlers_test

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"testing"
	"time"
	"tutorgo/models"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/mock"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// --- Common test data ---

const (
	testTutorID   = "11111111-1111-1111-1111-111111111111"
	testStudentID = "22222222-2222-2222-2222-222222222222"
	testCourseID  = "33333333-3333-3333-3333-333333333333"
	testLessonID  = "44444444-4444-4444-4444-444444444444"
	testPaymentID = "55555555-5555-5555-5555-555555555555"
)

var (
	testTutor = models.Tutor{
		ID:        testTutorID,
		Email:     "tutor@example.com",
		FirstName: "Amir",
		LastName:  "Bekov",
	}

	testStudent = models.Student{
		ID:        testStudentID,
		TutorID:   testTutorID,
		FirstName: "Aiya",
		LastName:  "Bekova",
	}

	testCourse = models.Course{
		ID:             testCourseID,
		TutorID:        testTutorID,
		StudentID:      testStudentID,
		Subject:        "Mathematics",
		PricePerLesson: 5000,
		StartedAt:      time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC),
		EndedAt:        time.Date(2026, time.June, 1, 0, 0, 0, 0, time.UTC),
	}

	testLesson = models.Lesson{
		ID:              testLessonID,
		CourseID:        testCourseID,
		ScheduledAt:     time.Date(2026, time.May, 1, 10, 0, 0, 0, time.UTC),
		DurationMinutes: 60,
		Status:          "scheduled",
	}

	testPayment = models.Payment{
		ID:           testPaymentID,
		CourseID:     testCourseID,
		Amount:       5000,
		LessonsCount: 10,
		PaidAt:       time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC),
	}
)

// --- Helpers ---

func makeRequest(t *testing.T, router *gin.Engine, method, path string, body interface{}) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			t.Fatal(err)
		}
	}
	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}

func decodeJSON(t *testing.T, w *httptest.ResponseRecorder, v interface{}) {
	t.Helper()
	if err := json.NewDecoder(w.Body).Decode(v); err != nil {
		t.Fatalf("failed to decode response body: %v", err)
	}
}

func withTutorID(tutorID string) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Set("tutorID", tutorID)
		c.Next()
	}
}

// --- Mock: StudentService ---

type mockStudentService struct{ mock.Mock }

func (m *mockStudentService) GetAll(tutorID string) ([]models.Student, error) {
	args := m.Called(tutorID)
	return args.Get(0).([]models.Student), args.Error(1)
}
func (m *mockStudentService) Create(req models.CreateStudentRequest, tutorID string) (models.Student, error) {
	args := m.Called(req, tutorID)
	return args.Get(0).(models.Student), args.Error(1)
}
func (m *mockStudentService) GetByID(id string, tutorID string) (models.Student, error) {
	args := m.Called(id, tutorID)
	return args.Get(0).(models.Student), args.Error(1)
}
func (m *mockStudentService) Update(id string, tutorID string, req models.UpdateStudentRequest) (models.Student, error) {
	args := m.Called(id, tutorID, req)
	return args.Get(0).(models.Student), args.Error(1)
}
func (m *mockStudentService) Delete(id string, tutorID string) error {
	return m.Called(id, tutorID).Error(0)
}

// --- Mock: TutorService ---

type mockTutorService struct{ mock.Mock }

func (m *mockTutorService) Create(req models.CreateTutorRequest, passwordHash string) (models.Tutor, error) {
	args := m.Called(req, passwordHash)
	return args.Get(0).(models.Tutor), args.Error(1)
}
func (m *mockTutorService) GetAll() ([]models.Tutor, error) {
	args := m.Called()
	return args.Get(0).([]models.Tutor), args.Error(1)
}
func (m *mockTutorService) GetByID(id string) (models.Tutor, error) {
	args := m.Called(id)
	return args.Get(0).(models.Tutor), args.Error(1)
}
func (m *mockTutorService) GetByEmail(email string) (string, string, error) {
	args := m.Called(email)
	return args.String(0), args.String(1), args.Error(2)
}
func (m *mockTutorService) Update(id string, req models.UpdateTutorRequest) (models.Tutor, error) {
	args := m.Called(id, req)
	return args.Get(0).(models.Tutor), args.Error(1)
}
func (m *mockTutorService) Delete(id string) error {
	return m.Called(id).Error(0)
}

// --- Mock: CourseService ---

type mockCourseService struct{ mock.Mock }

func (m *mockCourseService) Create(req models.CreateCourseRequest, tutorID string) (models.Course, error) {
	args := m.Called(req, tutorID)
	return args.Get(0).(models.Course), args.Error(1)
}
func (m *mockCourseService) GetAll(tutorID string) ([]models.Course, error) {
	args := m.Called(tutorID)
	return args.Get(0).([]models.Course), args.Error(1)
}
func (m *mockCourseService) GetByID(id string, tutorID string) (models.Course, error) {
	args := m.Called(id, tutorID)
	return args.Get(0).(models.Course), args.Error(1)
}
func (m *mockCourseService) Update(id string, tutorID string, req models.UpdateCourseRequest) (models.Course, error) {
	args := m.Called(id, tutorID, req)
	return args.Get(0).(models.Course), args.Error(1)
}
func (m *mockCourseService) Delete(id string, tutorID string) error {
	return m.Called(id, tutorID).Error(0)
}

// --- Mock: PaymentService ---

type mockPaymentService struct{ mock.Mock }

func (m *mockPaymentService) Create(req models.CreatePaymentRequest, tutorID string) (models.Payment, error) {
	args := m.Called(req, tutorID)
	return args.Get(0).(models.Payment), args.Error(1)
}
func (m *mockPaymentService) GetByCourse(courseID string, tutorID string) ([]models.Payment, error) {
	args := m.Called(courseID, tutorID)
	return args.Get(0).([]models.Payment), args.Error(1)
}
func (m *mockPaymentService) GetBalance(courseID string, tutorID string) (models.CourseBalance, error) {
	args := m.Called(courseID, tutorID)
	return args.Get(0).(models.CourseBalance), args.Error(1)
}

// --- Mock: LessonService ---

type mockLessonService struct{ mock.Mock }

func (m *mockLessonService) Create(req models.CreateLessonRequest, tutorID string) (models.Lesson, error) {
	args := m.Called(req, tutorID)
	return args.Get(0).(models.Lesson), args.Error(1)
}
func (m *mockLessonService) GetByCourse(courseID string, tutorID string) ([]models.Lesson, error) {
	args := m.Called(courseID, tutorID)
	return args.Get(0).([]models.Lesson), args.Error(1)
}
func (m *mockLessonService) GetByID(id string, tutorID string) (models.Lesson, error) {
	args := m.Called(id, tutorID)
	return args.Get(0).(models.Lesson), args.Error(1)
}
func (m *mockLessonService) Update(id string, req models.UpdateLessonRequest, tutorID string) (models.Lesson, error) {
	args := m.Called(id, req, tutorID)
	return args.Get(0).(models.Lesson), args.Error(1)
}
func (m *mockLessonService) Delete(id string, tutorID string) error {
	return m.Called(id, tutorID).Error(0)
}
