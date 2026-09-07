package handlers_test

import (
	"bytes"
	"context"
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

	testStudentIDPtr = func() *string { s := testStudentID; return &s }()

	testCourse = models.Course{
		ID:              testCourseID,
		TutorID:         testTutorID,
		StudentID:       testStudentIDPtr,
		Subject:         "Mathematics",
		PricePerCycle:   20000,
		LessonsPerCycle: 4,
		StartedAt:       time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC),
		EndedAt:         func() *time.Time { t := time.Date(2026, time.June, 1, 0, 0, 0, 0, time.UTC); return &t }(),
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

func withStudentID(studentID string) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Set("studentID", studentID)
		c.Next()
	}
}

// --- Mock: StudentService ---

type mockStudentService struct{ mock.Mock }

func (m *mockStudentService) GetAll(ctx context.Context, tutorID string, p models.Pagination) ([]models.Student, int, error) {
	args := m.Called(ctx, tutorID, p)
	return args.Get(0).([]models.Student), args.Int(1), args.Error(2)
}
func (m *mockStudentService) Create(ctx context.Context, req models.CreateStudentRequest, tutorID string) (models.Student, error) {
	args := m.Called(ctx, req, tutorID)
	return args.Get(0).(models.Student), args.Error(1)
}
func (m *mockStudentService) GetByID(ctx context.Context, id string, tutorID string) (models.Student, error) {
	args := m.Called(ctx, id, tutorID)
	return args.Get(0).(models.Student), args.Error(1)
}
func (m *mockStudentService) Update(ctx context.Context, id string, tutorID string, req models.UpdateStudentRequest) (models.Student, error) {
	args := m.Called(ctx, id, tutorID, req)
	return args.Get(0).(models.Student), args.Error(1)
}
func (m *mockStudentService) Delete(ctx context.Context, id string, tutorID string) error {
	return m.Called(ctx, id, tutorID).Error(0)
}
func (m *mockStudentService) SetInvite(ctx context.Context, studentID, token string, expiresAt time.Time) error {
	args := m.Called(ctx, studentID, token, expiresAt)
	return args.Error(0)
}
func (m *mockStudentService) GetByInviteToken(ctx context.Context, token string) (string, time.Time, error) {
	args := m.Called(ctx, token)
	return args.String(0), args.Get(1).(time.Time), args.Error(2)
}
func (m *mockStudentService) ActivateAccount(ctx context.Context, studentID, username, passwordHash string) error {
	args := m.Called(ctx, studentID, username, passwordHash)
	return args.Error(0)
}
func (m *mockStudentService) GetCredentialsByLogin(ctx context.Context, identifier string) (string, string, error) {
	args := m.Called(ctx, identifier)
	return args.String(0), args.String(1), args.Error(2)
}
func (m *mockStudentService) EnrolledInLesson(ctx context.Context, studentID, lessonID string) (bool, error) {
	args := m.Called(ctx, studentID, lessonID)
	return args.Bool(0), args.Error(1)
}
func (m *mockStudentService) CourseAndTutorForLesson(ctx context.Context, lessonID string) (string, string, error) {
	args := m.Called(ctx, lessonID)
	return args.String(0), args.String(1), args.Error(2)
}
func (m *mockStudentService) GetProfile(ctx context.Context, studentID string) (models.StudentProfile, error) {
	args := m.Called(ctx, studentID)
	return args.Get(0).(models.StudentProfile), args.Error(1)
}
func (m *mockStudentService) ListLessons(ctx context.Context, studentID string, past bool) ([]models.CalendarLesson, error) {
	args := m.Called(ctx, studentID, past)
	return args.Get(0).([]models.CalendarLesson), args.Error(1)
}
func (m *mockStudentService) GetPasswordHash(ctx context.Context, studentID string) (string, error) {
	args := m.Called(ctx, studentID)
	return args.String(0), args.Error(1)
}
func (m *mockStudentService) UpdatePassword(ctx context.Context, studentID, hash string) error {
	return m.Called(ctx, studentID, hash).Error(0)
}
func (m *mockStudentService) ListHomework(ctx context.Context, studentID string) ([]models.StudentHomework, error) {
	args := m.Called(ctx, studentID)
	return args.Get(0).([]models.StudentHomework), args.Error(1)
}
func (m *mockStudentService) ListCourses(ctx context.Context, studentID string) ([]models.StudentCourse, error) {
	args := m.Called(ctx, studentID)
	return args.Get(0).([]models.StudentCourse), args.Error(1)
}

// --- Mock: WhiteboardService ---

type mockWhiteboardService struct{ mock.Mock }

func (m *mockWhiteboardService) GetOrCreateBoard(ctx context.Context, courseID, tutorID string) (models.BoardWithPages, error) {
	args := m.Called(ctx, courseID, tutorID)
	return args.Get(0).(models.BoardWithPages), args.Error(1)
}
func (m *mockWhiteboardService) GetOrCreateTrialBoard(ctx context.Context, tutorID string) (models.BoardWithPages, error) {
	args := m.Called(ctx, tutorID)
	return args.Get(0).(models.BoardWithPages), args.Error(1)
}
func (m *mockWhiteboardService) ValidateInvite(ctx context.Context, inviteID string) (models.BoardWithPages, error) {
	args := m.Called(ctx, inviteID)
	return args.Get(0).(models.BoardWithPages), args.Error(1)
}
func (m *mockWhiteboardService) CreatePage(ctx context.Context, boardID, tutorID, title string) (models.BoardPage, error) {
	args := m.Called(ctx, boardID, tutorID, title)
	return args.Get(0).(models.BoardPage), args.Error(1)
}
func (m *mockWhiteboardService) UpdatePage(ctx context.Context, pageID, tutorID string, req models.UpdateBoardPageRequest) (models.BoardPage, error) {
	args := m.Called(ctx, pageID, tutorID, req)
	return args.Get(0).(models.BoardPage), args.Error(1)
}
func (m *mockWhiteboardService) DeletePage(ctx context.Context, pageID, tutorID string) error {
	return m.Called(ctx, pageID, tutorID).Error(0)
}
func (m *mockWhiteboardService) SaveSnapshot(ctx context.Context, pageID string, snapshot json.RawMessage) error {
	return m.Called(ctx, pageID, snapshot).Error(0)
}
func (m *mockWhiteboardService) CreateInvite(ctx context.Context, boardID, tutorID string) (models.BoardInvite, error) {
	args := m.Called(ctx, boardID, tutorID)
	return args.Get(0).(models.BoardInvite), args.Error(1)
}
func (m *mockWhiteboardService) DeleteInvite(ctx context.Context, boardID, tutorID string) error {
	return m.Called(ctx, boardID, tutorID).Error(0)
}
func (m *mockWhiteboardService) SaveAsset(ctx context.Context, boardID, tutorID, filePath, mimeType string, sizeBytes int) (models.BoardAsset, error) {
	args := m.Called(ctx, boardID, tutorID, filePath, mimeType, sizeBytes)
	return args.Get(0).(models.BoardAsset), args.Error(1)
}
func (m *mockWhiteboardService) SaveAssetByInvite(ctx context.Context, inviteID, filePath, mimeType string, sizeBytes int) (models.BoardAsset, error) {
	args := m.Called(ctx, inviteID, filePath, mimeType, sizeBytes)
	return args.Get(0).(models.BoardAsset), args.Error(1)
}
func (m *mockWhiteboardService) GetAsset(ctx context.Context, assetID string) (models.BoardAsset, error) {
	args := m.Called(ctx, assetID)
	return args.Get(0).(models.BoardAsset), args.Error(1)
}
func (m *mockWhiteboardService) PageBelongsToTutor(ctx context.Context, pageID, tutorID string) (bool, error) {
	args := m.Called(ctx, pageID, tutorID)
	return args.Bool(0), args.Error(1)
}
func (m *mockWhiteboardService) MergeElements(ctx context.Context, pageID string, els []models.BoardElement) error {
	return m.Called(ctx, pageID, els).Error(0)
}
func (m *mockWhiteboardService) MergeSnapshot(ctx context.Context, pageID string, elements []json.RawMessage, files json.RawMessage) error {
	return m.Called(ctx, pageID, elements, files).Error(0)
}
func (m *mockWhiteboardService) GetPageState(ctx context.Context, pageID string) (json.RawMessage, error) {
	args := m.Called(ctx, pageID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(json.RawMessage), args.Error(1)
}
func (m *mockWhiteboardService) DeleteOldTombstones(ctx context.Context) (int64, error) {
	args := m.Called(ctx)
	return int64(args.Int(0)), args.Error(1)
}

// --- Mock: RegistrationService ---

type mockRegistrationService struct{ mock.Mock }

func (m *mockRegistrationService) Start(ctx context.Context, req models.RegisterRequest) error {
	return m.Called(ctx, req).Error(0)
}
func (m *mockRegistrationService) Verify(ctx context.Context, email, code string) (models.Tutor, error) {
	args := m.Called(ctx, email, code)
	return args.Get(0).(models.Tutor), args.Error(1)
}
func (m *mockRegistrationService) Resend(ctx context.Context, email string) error {
	return m.Called(ctx, email).Error(0)
}

// --- Mock: TutorService ---

type mockTutorService struct{ mock.Mock }

func (m *mockTutorService) Create(ctx context.Context, req models.CreateTutorRequest, passwordHash string) (models.Tutor, error) {
	args := m.Called(ctx, req, passwordHash)
	return args.Get(0).(models.Tutor), args.Error(1)
}
func (m *mockTutorService) Register(ctx context.Context, req models.CreateTutorRequest, passwordHash string) (models.Tutor, error) {
	args := m.Called(ctx, req, passwordHash)
	return args.Get(0).(models.Tutor), args.Error(1)
}
func (m *mockTutorService) GetAll(ctx context.Context) ([]models.Tutor, error) {
	args := m.Called(ctx)
	return args.Get(0).([]models.Tutor), args.Error(1)
}
func (m *mockTutorService) GetByID(ctx context.Context, id string) (models.Tutor, error) {
	args := m.Called(ctx, id)
	return args.Get(0).(models.Tutor), args.Error(1)
}
func (m *mockTutorService) GetByEmail(ctx context.Context, email string) (string, string, error) {
	args := m.Called(ctx, email)
	return args.String(0), args.String(1), args.Error(2)
}
func (m *mockTutorService) GetByPhone(ctx context.Context, phone string) (string, string, error) {
	args := m.Called(ctx, phone)
	return args.String(0), args.String(1), args.Error(2)
}

func (m *mockTutorService) Update(ctx context.Context, id string, req models.UpdateTutorRequest) (models.Tutor, error) {
	args := m.Called(ctx, id, req)
	return args.Get(0).(models.Tutor), args.Error(1)
}
func (m *mockTutorService) Delete(ctx context.Context, id string) error {
	return m.Called(ctx, id).Error(0)
}

func (m *mockTutorService) GetPasswordHash(ctx context.Context, id string) (string, error) {
	args := m.Called(ctx, id)
	return args.String(0), args.Error(1)
}

func (m *mockTutorService) UpdatePassword(ctx context.Context, id string, hash string) error {
	return m.Called(ctx, id, hash).Error(0)
}

// --- Mock: CourseService ---

type mockCourseService struct{ mock.Mock }

func (m *mockCourseService) Create(ctx context.Context, req models.CreateCourseRequest, tutorID string) (models.Course, error) {
	args := m.Called(ctx, req, tutorID)
	return args.Get(0).(models.Course), args.Error(1)
}
func (m *mockCourseService) GetSubjects(ctx context.Context, tutorID string) ([]string, error) {
	args := m.Called(ctx, tutorID)
	return args.Get(0).([]string), args.Error(1)
}
func (m *mockCourseService) GetAll(ctx context.Context, tutorID string, p models.Pagination) ([]models.Course, int, error) {
	args := m.Called(ctx, tutorID, p)
	return args.Get(0).([]models.Course), args.Int(1), args.Error(2)
}
func (m *mockCourseService) GetByID(ctx context.Context, id string, tutorID string) (models.Course, error) {
	args := m.Called(ctx, id, tutorID)
	return args.Get(0).(models.Course), args.Error(1)
}
func (m *mockCourseService) Update(ctx context.Context, id string, tutorID string, req models.UpdateCourseRequest) (models.Course, error) {
	args := m.Called(ctx, id, tutorID, req)
	return args.Get(0).(models.Course), args.Error(1)
}
func (m *mockCourseService) GetByStudent(ctx context.Context, studentID string, tutorID string) ([]models.Course, error) {
	args := m.Called(ctx, studentID, tutorID)
	return args.Get(0).([]models.Course), args.Error(1)
}
func (m *mockCourseService) Delete(ctx context.Context, id string, tutorID string) error {
	return m.Called(ctx, id, tutorID).Error(0)
}
func (m *mockCourseService) GetArchived(ctx context.Context, tutorID string, p models.Pagination) ([]models.Course, int, error) {
	args := m.Called(ctx, tutorID, p)
	return args.Get(0).([]models.Course), args.Int(1), args.Error(2)
}
func (m *mockCourseService) Restore(ctx context.Context, id string, tutorID string) error {
	return m.Called(ctx, id, tutorID).Error(0)
}
func (m *mockCourseService) GetHomework(ctx context.Context, id string, tutorID string) (string, error) {
	args := m.Called(ctx, id, tutorID)
	return args.String(0), args.Error(1)
}
func (m *mockCourseService) SetHomework(ctx context.Context, id string, tutorID string, homework string) error {
	return m.Called(ctx, id, tutorID, homework).Error(0)
}

// --- Mock: PaymentService ---

type mockPaymentService struct{ mock.Mock }

func (m *mockPaymentService) Create(ctx context.Context, req models.CreatePaymentRequest, tutorID string) (models.Payment, error) {
	args := m.Called(ctx, req, tutorID)
	return args.Get(0).(models.Payment), args.Error(1)
}
func (m *mockPaymentService) GetByCourse(ctx context.Context, courseID string, tutorID string, p models.Pagination) ([]models.Payment, int, error) {
	args := m.Called(ctx, courseID, tutorID, p)
	return args.Get(0).([]models.Payment), args.Int(1), args.Error(2)
}
func (m *mockPaymentService) GetAllByTutor(ctx context.Context, tutorID string, limit int) ([]models.Payment, error) {
	args := m.Called(ctx, tutorID, limit)
	return args.Get(0).([]models.Payment), args.Error(1)
}
func (m *mockPaymentService) GetAllByTutorPaged(ctx context.Context, tutorID string, p models.Pagination) ([]models.Payment, int, error) {
	args := m.Called(ctx, tutorID, p)
	return args.Get(0).([]models.Payment), args.Int(1), args.Error(2)
}
func (m *mockPaymentService) GetBalance(ctx context.Context, courseID string, tutorID string) (models.CourseBalance, error) {
	args := m.Called(ctx, courseID, tutorID)
	return args.Get(0).(models.CourseBalance), args.Error(1)
}

func (m *mockPaymentService) GetMonthlyIncome(ctx context.Context, tutorID string) (float64, error) {
	args := m.Called(ctx, tutorID)
	return args.Get(0).(float64), args.Error(1)
}

func (m *mockPaymentService) GetMonthlyExpected(ctx context.Context, tutorID string) (float64, error) {
	args := m.Called(ctx, tutorID)
	return args.Get(0).(float64), args.Error(1)
}

func (m *mockPaymentService) Delete(ctx context.Context, id string, tutorID string) error {
	args := m.Called(ctx, id, tutorID)
	return args.Error(0)
}

func (m *mockPaymentService) Update(ctx context.Context, id string, tutorID string, req models.UpdatePaymentRequest) (models.Payment, error) {
	args := m.Called(ctx, id, tutorID, req)
	return args.Get(0).(models.Payment), args.Error(1)
}

// --- Mock: RefreshTokenService ---

type mockRefreshTokenService struct{ mock.Mock }

func (m *mockRefreshTokenService) Create(ctx context.Context, tutorID string) (string, error) {
	args := m.Called(ctx, tutorID)
	return args.String(0), args.Error(1)
}
func (m *mockRefreshTokenService) Validate(ctx context.Context, token string) (string, error) {
	args := m.Called(ctx, token)
	return args.String(0), args.Error(1)
}
func (m *mockRefreshTokenService) Revoke(ctx context.Context, token string) error {
	return m.Called(ctx, token).Error(0)
}
func (m *mockRefreshTokenService) RevokeAll(ctx context.Context, tutorID string) error {
	return m.Called(ctx, tutorID).Error(0)
}

// --- Mock: StudentRefreshTokenService ---

type mockStudentRefreshTokenService struct{ mock.Mock }

func (m *mockStudentRefreshTokenService) Create(ctx context.Context, studentID string) (string, error) {
	args := m.Called(ctx, studentID)
	return args.String(0), args.Error(1)
}
func (m *mockStudentRefreshTokenService) Validate(ctx context.Context, token string) (string, error) {
	args := m.Called(ctx, token)
	return args.String(0), args.Error(1)
}
func (m *mockStudentRefreshTokenService) Revoke(ctx context.Context, token string) error {
	return m.Called(ctx, token).Error(0)
}
func (m *mockStudentRefreshTokenService) RevokeAll(ctx context.Context, studentID string) error {
	return m.Called(ctx, studentID).Error(0)
}

// --- Mock: LessonService ---

type mockLessonService struct{ mock.Mock }

func (m *mockLessonService) Create(ctx context.Context, req models.CreateLessonRequest, tutorID string) (models.Lesson, error) {
	args := m.Called(ctx, req, tutorID)
	return args.Get(0).(models.Lesson), args.Error(1)
}
func (m *mockLessonService) GetByCourse(ctx context.Context, courseID string, tutorID string) ([]models.Lesson, error) {
	args := m.Called(ctx, courseID, tutorID)
	return args.Get(0).([]models.Lesson), args.Error(1)
}
func (m *mockLessonService) GetByID(ctx context.Context, id string, tutorID string) (models.Lesson, error) {
	args := m.Called(ctx, id, tutorID)
	return args.Get(0).(models.Lesson), args.Error(1)
}
func (m *mockLessonService) Update(ctx context.Context, id string, req models.UpdateLessonRequest, tutorID, scope string) (models.Lesson, error) {
	args := m.Called(ctx, id, req, tutorID, scope)
	return args.Get(0).(models.Lesson), args.Error(1)
}
func (m *mockLessonService) Delete(ctx context.Context, id string, tutorID, scope string) error {
	return m.Called(ctx, id, tutorID, scope).Error(0)
}

func (m *mockLessonService) GetCalendar(ctx context.Context, tutorID string, from string, to string) ([]models.CalendarLesson, error) {
	args := m.Called(ctx, tutorID, from, to)
	return args.Get(0).([]models.CalendarLesson), args.Error(1)
}

func (m *mockLessonService) DeleteByCourse(ctx context.Context, courseID string, tutorID string) error {
	return m.Called(ctx, courseID, tutorID).Error(0)
}

func (m *mockLessonService) ArchiveCourseSchedule(ctx context.Context, courseID, tutorID string) error {
	return m.Called(ctx, courseID, tutorID).Error(0)
}

func (m *mockLessonService) StartRoom(ctx context.Context, lessonID string, tutorID string) error {
	return m.Called(ctx, lessonID, tutorID).Error(0)
}

func (m *mockLessonService) EndRoom(ctx context.Context, lessonID string, tutorID string) error {
	return m.Called(ctx, lessonID, tutorID).Error(0)
}

func (m *mockLessonService) EndRoomByID(ctx context.Context, lessonID string) error {
	return m.Called(ctx, lessonID).Error(0)
}

func (m *mockLessonService) GetRoomStatus(ctx context.Context, id string) (string, error) {
	args := m.Called(ctx, id)
	return args.String(0), args.Error(1)
}

func (m *mockLessonService) GetByCoursePaged(ctx context.Context, courseID string, tutorID string, p models.Pagination) (models.PagedResponse[models.Lesson], error) {
	args := m.Called(ctx, courseID, tutorID, p)
	return args.Get(0).(models.PagedResponse[models.Lesson]), args.Error(1)
}

func (m *mockLessonService) GetByPeriod(ctx context.Context, courseID string, tutorID string, from string, to string) ([]models.Lesson, error) {
	args := m.Called(ctx, courseID, tutorID, from, to)
	return args.Get(0).([]models.Lesson), args.Error(1)
}

func (m *mockLessonService) GetCurrentCycles(ctx context.Context, tutorID string) ([]models.CurrentCycleInfo, error) {
	args := m.Called(ctx, tutorID)
	return args.Get(0).([]models.CurrentCycleInfo), args.Error(1)
}
