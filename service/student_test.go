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
	"github.com/stretchr/testify/require"
)

type mockStudentRepo struct {
	mock.Mock
}

func (m *mockStudentRepo) Create(ctx context.Context, req models.CreateStudentRequest, tutorID string) (models.Student, error) {
	args := m.Called(ctx, req, tutorID)
	return args.Get(0).(models.Student), args.Error(1)
}

func (m *mockStudentRepo) GetAll(ctx context.Context, tutorID string, p models.Pagination, archived bool) ([]models.Student, int, error) {
	args := m.Called(ctx, tutorID, p, archived)
	return args.Get(0).([]models.Student), args.Int(1), args.Error(2)
}

func (m *mockStudentRepo) GetByID(ctx context.Context, id string, tutorID string) (models.Student, error) {
	args := m.Called(ctx, id, tutorID)
	return args.Get(0).(models.Student), args.Error(1)
}

func (m *mockStudentRepo) Update(ctx context.Context, id string, tutorID string, req models.UpdateStudentRequest) (models.Student, error) {
	args := m.Called(ctx, id, tutorID, req)
	return args.Get(0).(models.Student), args.Error(1)
}

func (m *mockStudentRepo) Delete(ctx context.Context, id string, tutorID string) (bool, error) {
	args := m.Called(ctx, id, tutorID)
	return args.Bool(0), args.Error(1)
}

func (m *mockStudentRepo) SetActive(ctx context.Context, id string, tutorID string, active bool) error {
	return m.Called(ctx, id, tutorID, active).Error(0)
}

// mockStudentSessions — отзыв refresh-токенов кабинета при архивации.
type mockStudentSessions struct{ mock.Mock }

func (m *mockStudentSessions) DeleteByStudentID(ctx context.Context, studentID string) error {
	return m.Called(ctx, studentID).Error(0)
}

func (m *mockStudentRepo) SetInvite(ctx context.Context, studentID, token string, expiresAt time.Time) error {
	args := m.Called(ctx, studentID, token, expiresAt)
	return args.Error(0)
}

func (m *mockStudentRepo) GetByInviteToken(ctx context.Context, token string) (string, time.Time, error) {
	args := m.Called(ctx, token)
	return args.String(0), args.Get(1).(time.Time), args.Error(2)
}

func (m *mockStudentRepo) ActivateAccount(ctx context.Context, studentID, username, passwordHash string) error {
	args := m.Called(ctx, studentID, username, passwordHash)
	return args.Error(0)
}

func (m *mockStudentRepo) GetCredentialsByLogin(ctx context.Context, identifier string) (string, string, error) {
	args := m.Called(ctx, identifier)
	return args.String(0), args.String(1), args.Error(2)
}

func (m *mockStudentRepo) EnrolledInLesson(ctx context.Context, studentID, lessonID string) (bool, error) {
	args := m.Called(ctx, studentID, lessonID)
	return args.Bool(0), args.Error(1)
}

func (m *mockStudentRepo) CourseAndTutorForLesson(ctx context.Context, lessonID string) (string, string, error) {
	args := m.Called(ctx, lessonID)
	return args.String(0), args.String(1), args.Error(2)
}

func (m *mockStudentRepo) GetProfile(ctx context.Context, studentID string) (models.StudentProfile, error) {
	args := m.Called(ctx, studentID)
	return args.Get(0).(models.StudentProfile), args.Error(1)
}

func (m *mockStudentRepo) ListLessons(ctx context.Context, studentID string, past bool) ([]models.CalendarLesson, error) {
	args := m.Called(ctx, studentID, past)
	return args.Get(0).([]models.CalendarLesson), args.Error(1)
}

func (m *mockStudentRepo) GetPasswordHash(ctx context.Context, studentID string) (string, error) {
	args := m.Called(ctx, studentID)
	return args.String(0), args.Error(1)
}

func (m *mockStudentRepo) UpdatePassword(ctx context.Context, studentID, hash string) error {
	args := m.Called(ctx, studentID, hash)
	return args.Error(0)
}

func (m *mockStudentRepo) ListHomework(ctx context.Context, studentID string) ([]models.StudentHomework, error) {
	args := m.Called(ctx, studentID)
	return args.Get(0).([]models.StudentHomework), args.Error(1)
}

func (m *mockStudentRepo) ListCourses(ctx context.Context, studentID string) ([]models.StudentCourse, error) {
	args := m.Called(ctx, studentID)
	return args.Get(0).([]models.StudentCourse), args.Error(1)
}

// Тесты
func TestGetAllStudents_Success(t *testing.T) {
	repo := new(mockStudentRepo)
	svc := service.NewStudentService(repo, new(mockPaymentRepo), nil, nil, nil, nil)

	p := models.Pagination{Page: 1, Limit: 20}
	expected := []models.Student{
		{ID: "1", FirstName: "Aiya", LastName: "Bekova", TutorID: "tutor-1"},
		{ID: "2", FirstName: "Zhanibek", LastName: "Gabitov", TutorID: "tutor-1"},
	}

	repo.On("GetAll", mock.Anything, "tutor-1", p, false).Return(expected, 2, nil)

	students, total, err := svc.GetAll(context.Background(), "tutor-1", p, false)

	assert.NoError(t, err)
	assert.Equal(t, expected, students)
	assert.Equal(t, 2, total)
	repo.AssertExpectations(t)
}

func TestGetAllStudents_Error(t *testing.T) {
	repo := new(mockStudentRepo)
	svc := service.NewStudentService(repo, new(mockPaymentRepo), nil, nil, nil, nil)

	p := models.Pagination{Page: 1, Limit: 20}
	repo.On("GetAll", mock.Anything, "tutor-1", p, false).Return([]models.Student{}, 0, errors.New("db error"))

	students, total, err := svc.GetAll(context.Background(), "tutor-1", p, false)

	assert.Error(t, err)
	assert.Empty(t, students)
	assert.Equal(t, 0, total)
	repo.AssertExpectations(t)
}

func TestCreateStudent_Success(t *testing.T) {
	repo := new(mockStudentRepo)
	svc := service.NewStudentService(repo, new(mockPaymentRepo), nil, nil, nil, nil)

	req := models.CreateStudentRequest{
		FirstName: "Aiya",
		LastName:  "Bekova",
	}
	expected := models.Student{ID: "1", FirstName: "Aiya", LastName: "Bekova", TutorID: "tutor-1"}

	repo.On("Create", mock.Anything, req, "tutor-1").Return(expected, nil)

	student, err := svc.Create(context.Background(), req, "tutor-1")

	assert.NoError(t, err)
	assert.Equal(t, expected, student)
	repo.AssertExpectations(t)
}

func TestCreateStudent_Error(t *testing.T) {
	repo := new(mockStudentRepo)
	svc := service.NewStudentService(repo, new(mockPaymentRepo), nil, nil, nil, nil)

	req := models.CreateStudentRequest{
		FirstName: "Aiya",
		LastName:  "Bekova",
	}
	repo.On("Create", mock.Anything, req, "tutor-1").Return(models.Student{}, errors.New("failed to create new student"))
	student, err := svc.Create(context.Background(), req, "tutor-1")

	assert.Error(t, err)
	assert.Empty(t, student)
	repo.AssertExpectations(t)
}

func TestDeleteStudent_Success(t *testing.T) {
	repo := new(mockStudentRepo)
	svc := service.NewStudentService(repo, new(mockPaymentRepo), nil, nil, nil, nil)

	repo.On("GetByID", mock.Anything, "student-1", "tutor-1").Return(models.Student{ID: "student-1"}, nil)
	repo.On("Delete", mock.Anything, "student-1", "tutor-1").Return(true, nil)

	err := svc.Delete(context.Background(), "student-1", "tutor-1")

	assert.NoError(t, err)
	repo.AssertExpectations(t)
}

// GetByID

func TestStudentGetByID_NotFound(t *testing.T) {
	repo := new(mockStudentRepo)
	svc := service.NewStudentService(repo, new(mockPaymentRepo), nil, nil, nil, nil)

	repo.On("GetByID", mock.Anything, "student-1", "tutor-1").Return(models.Student{}, errors.New("not found"))

	student, err := svc.GetByID(context.Background(), "student-1", "tutor-1")

	assert.ErrorIs(t, err, service.ErrNotFound)
	assert.Empty(t, student)
	repo.AssertExpectations(t)
}

// Update

var updateStudentReq = models.UpdateStudentRequest{FirstName: "Aiya", LastName: "Bekova"}

func TestStudentUpdate_Success(t *testing.T) {
	repo := new(mockStudentRepo)
	svc := service.NewStudentService(repo, new(mockPaymentRepo), nil, nil, nil, nil)

	updated := models.Student{ID: "student-1", FirstName: "Aiya", LastName: "Bekova", TutorID: "tutor-1"}
	repo.On("GetByID", mock.Anything, "student-1", "tutor-1").Return(models.Student{ID: "student-1"}, nil)
	repo.On("Update", mock.Anything, "student-1", "tutor-1", updateStudentReq).Return(updated, nil)

	student, err := svc.Update(context.Background(), "student-1", "tutor-1", updateStudentReq)

	assert.NoError(t, err)
	assert.Equal(t, updated, student)
	repo.AssertExpectations(t)
}

func TestStudentUpdate_NotFound(t *testing.T) {
	repo := new(mockStudentRepo)
	svc := service.NewStudentService(repo, new(mockPaymentRepo), nil, nil, nil, nil)

	repo.On("GetByID", mock.Anything, "student-1", "tutor-1").Return(models.Student{}, errors.New("not found"))

	student, err := svc.Update(context.Background(), "student-1", "tutor-1", updateStudentReq)

	assert.ErrorIs(t, err, service.ErrNotFound)
	assert.Empty(t, student)
	repo.AssertNotCalled(t, "Update")
	repo.AssertExpectations(t)
}

func TestStudentUpdate_RepoError(t *testing.T) {
	repo := new(mockStudentRepo)
	svc := service.NewStudentService(repo, new(mockPaymentRepo), nil, nil, nil, nil)

	repo.On("GetByID", mock.Anything, "student-1", "tutor-1").Return(models.Student{ID: "student-1"}, nil)
	repo.On("Update", mock.Anything, "student-1", "tutor-1", updateStudentReq).Return(models.Student{}, errors.New("db error"))

	student, err := svc.Update(context.Background(), "student-1", "tutor-1", updateStudentReq)

	assert.Error(t, err)
	assert.False(t, errors.Is(err, service.ErrNotFound))
	assert.Empty(t, student)
	repo.AssertExpectations(t)
}

// Delete

func TestStudentDelete_NotFound(t *testing.T) {
	repo := new(mockStudentRepo)
	svc := service.NewStudentService(repo, new(mockPaymentRepo), nil, nil, nil, nil)

	repo.On("GetByID", mock.Anything, "student-1", "tutor-1").Return(models.Student{}, errors.New("not found"))

	err := svc.Delete(context.Background(), "student-1", "tutor-1")

	assert.ErrorIs(t, err, service.ErrNotFound)
	repo.AssertNotCalled(t, "Delete")
	repo.AssertExpectations(t)
}

// Ученик существует, а репозиторий ничего не удалил — мешает история. Это 409,
// а не 404: фронт предложит архив (спека, п. 5a.2).
func TestStudentDelete_WithHistoryIsConflict(t *testing.T) {
	repo := new(mockStudentRepo)
	svc := service.NewStudentService(repo, new(mockPaymentRepo), nil, nil, nil, nil)

	repo.On("GetByID", mock.Anything, "student-1", "tutor-1").Return(models.Student{ID: "student-1"}, nil)
	repo.On("Delete", mock.Anything, "student-1", "tutor-1").Return(false, nil)

	err := svc.Delete(context.Background(), "student-1", "tutor-1")

	assert.ErrorIs(t, err, service.ErrConflict)
	repo.AssertExpectations(t)
}

func TestStudentDelete_RepoError(t *testing.T) {
	repo := new(mockStudentRepo)
	svc := service.NewStudentService(repo, new(mockPaymentRepo), nil, nil, nil, nil)

	repo.On("GetByID", mock.Anything, "student-1", "tutor-1").Return(models.Student{ID: "student-1"}, nil)
	repo.On("Delete", mock.Anything, "student-1", "tutor-1").Return(false, errors.New("db error"))

	err := svc.Delete(context.Background(), "student-1", "tutor-1")

	assert.Error(t, err)
	assert.False(t, errors.Is(err, service.ErrNotFound))
	repo.AssertExpectations(t)
}

// ListCourses

func TestListCourses_IndividualAndGroup(t *testing.T) {
	repo := new(mockStudentRepo)
	svc := service.NewStudentService(repo, new(mockPaymentRepo), nil, nil, nil, nil)

	expected := []models.StudentCourse{
		{ID: "c1", Subject: "Algebra", TutorID: "tutor-1"},   // индивидуальный курс
		{ID: "c2", Subject: "Chemistry", TutorID: "tutor-2"}, // групповой курс (через enrollment)
	}
	repo.On("ListCourses", mock.Anything, "student-1").Return(expected, nil)

	courses, err := svc.ListCourses(context.Background(), "student-1")

	assert.NoError(t, err)
	assert.Equal(t, expected, courses)
	repo.AssertExpectations(t)
}

func TestListCourses_Error(t *testing.T) {
	repo := new(mockStudentRepo)
	svc := service.NewStudentService(repo, new(mockPaymentRepo), nil, nil, nil, nil)

	repo.On("ListCourses", mock.Anything, "student-1").Return([]models.StudentCourse{}, errors.New("db error"))

	courses, err := svc.ListCourses(context.Background(), "student-1")

	assert.Error(t, err)
	assert.Empty(t, courses)
	repo.AssertExpectations(t)
}

func TestListLessons_PaidFlag(t *testing.T) {
	repo := new(mockStudentRepo)
	payRepo := new(mockPaymentRepo)
	svc := service.NewStudentService(repo, payRepo, nil, nil, nil, nil)

	rank1, rank2, rank3 := 1, 2, 3
	lessons := []models.CalendarLesson{
		{ID: "l1", CourseID: "c1", Rank: &rank1},
		{ID: "l2", CourseID: "c1", Rank: &rank2},
		{ID: "l3", CourseID: "c1", Rank: &rank3},
	}
	repo.On("ListLessons", mock.Anything, "stu-1", false).Return(lessons, nil)
	payRepo.On("GetByStudentBatch", mock.Anything, "stu-1").Return(map[string][]models.Payment{
		"c1": {{LessonsCount: 2}},
	}, nil)

	got, err := svc.ListLessons(context.Background(), "stu-1", false)

	assert.NoError(t, err)
	assert.NotNil(t, got[0].Paid)
	assert.True(t, *got[0].Paid) // rank 1 within the paid cycle of 2
	assert.NotNil(t, got[1].Paid)
	assert.True(t, *got[1].Paid) // rank 2 within the paid cycle of 2
	assert.NotNil(t, got[2].Paid)
	assert.False(t, *got[2].Paid) // rank 3 beyond the paid cycle
	repo.AssertExpectations(t)
	payRepo.AssertExpectations(t)
}

func TestStudentListLessons_CyclePositions(t *testing.T) {
	repo := new(mockStudentRepo)
	payRepo := new(mockPaymentRepo)
	svc := service.NewStudentService(repo, payRepo, nil, nil, nil, nil)

	rank3, rank9 := 3, 9
	lessons := []models.CalendarLesson{
		{ID: "l1", CourseID: "c1", Rank: &rank3},
		{ID: "l2", CourseID: "c1", Rank: &rank9},
		{ID: "l3", CourseID: "c2"}, // rank нет (все уроки отменены) — цикл не считаем
	}
	repo.On("ListLessons", mock.Anything, "stu-1", false).Return(lessons, nil)
	payRepo.On("GetByStudentBatch", mock.Anything, "stu-1").Return(map[string][]models.Payment{
		"c1": {{LessonsCount: 8}, {LessonsCount: 8}},
	}, nil)

	got, err := svc.ListLessons(context.Background(), "stu-1", false)

	assert.NoError(t, err)
	assert.Equal(t, 3, *got[0].CyclePosition) // 3-й урок первого пакета из 8
	assert.Equal(t, 8, *got[0].CycleSize)
	assert.Equal(t, 1, *got[1].CyclePosition) // 9-й урок = 1-й второго пакета
	assert.Nil(t, got[2].CyclePosition)
	repo.AssertExpectations(t)
	payRepo.AssertExpectations(t)
}

// Архивация складывает существующие действия в порядке, в котором сбой не
// оставляет полуархивного ученика вне списка: курсы, группы, сессии и только
// последним active = false (спека, п. 5a.3). Групповой курс не архивируется —
// он идёт для остальных, из него ученик уходит через left_at.
func TestStudentArchive_StepsInOrder(t *testing.T) {
	repo := new(mockStudentRepo)
	courses := new(mockCourseRepo) // тот же GetByStudent/Delete, что у courseService
	enrollments := new(mockEnrollmentRepo)
	sessions := new(mockStudentSessions)
	svc := service.NewStudentService(repo, new(mockPaymentRepo), courses, enrollments, sessions, nil)

	studentID := "student-1"
	individual := models.Course{ID: "course-ind", StudentID: &studentID}
	group := models.Course{ID: "course-group"}

	var order []string
	step := func(name string) func(mock.Arguments) {
		return func(mock.Arguments) { order = append(order, name) }
	}

	repo.On("GetByID", mock.Anything, studentID, "tutor-1").Return(models.Student{ID: studentID, Active: true}, nil)
	courses.On("GetByStudent", mock.Anything, studentID, "tutor-1").Return([]models.Course{individual, group}, nil)
	courses.On("Delete", mock.Anything, "course-ind", "tutor-1").Run(step("course")).Return(nil)
	enrollments.On("LeaveAllByStudent", mock.Anything, studentID).Run(step("groups")).Return(nil)
	sessions.On("DeleteByStudentID", mock.Anything, studentID).Run(step("sessions")).Return(nil)
	repo.On("SetActive", mock.Anything, studentID, "tutor-1", false).Run(step("inactive")).Return(nil)

	require.NoError(t, svc.Archive(context.Background(), studentID, "tutor-1"))

	assert.Equal(t, []string{"course", "groups", "sessions", "inactive"}, order)
	courses.AssertNotCalled(t, "Delete", mock.Anything, "course-group", "tutor-1")
}

// Сбой посередине не выставляет active = false: ученик остаётся в списке, и
// повторная архивация доделает начатое.
func TestStudentArchive_FailureKeepsStudentActive(t *testing.T) {
	repo := new(mockStudentRepo)
	courses := new(mockCourseRepo)
	enrollments := new(mockEnrollmentRepo)
	sessions := new(mockStudentSessions)
	svc := service.NewStudentService(repo, new(mockPaymentRepo), courses, enrollments, sessions, nil)

	studentID := "student-1"
	individual := models.Course{ID: "course-ind", StudentID: &studentID}

	repo.On("GetByID", mock.Anything, studentID, "tutor-1").Return(models.Student{ID: studentID, Active: true}, nil)
	courses.On("GetByStudent", mock.Anything, studentID, "tutor-1").Return([]models.Course{individual}, nil)
	courses.On("Delete", mock.Anything, "course-ind", "tutor-1").Return(errors.New("db down"))

	err := svc.Archive(context.Background(), studentID, "tutor-1")

	assert.Error(t, err)
	enrollments.AssertNotCalled(t, "LeaveAllByStudent", mock.Anything, mock.Anything)
	repo.AssertNotCalled(t, "SetActive", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func TestStudentArchive_NotFound(t *testing.T) {
	repo := new(mockStudentRepo)
	courses := new(mockCourseRepo)
	svc := service.NewStudentService(repo, new(mockPaymentRepo), courses, new(mockEnrollmentRepo), new(mockStudentSessions), nil)

	repo.On("GetByID", mock.Anything, "student-1", "tutor-1").Return(models.Student{}, errors.New("not found"))

	err := svc.Archive(context.Background(), "student-1", "tutor-1")

	assert.ErrorIs(t, err, service.ErrNotFound)
	courses.AssertNotCalled(t, "GetByStudent", mock.Anything, mock.Anything, mock.Anything)
}

func TestStudentRestore_SetsActive(t *testing.T) {
	repo := new(mockStudentRepo)
	svc := service.NewStudentService(repo, new(mockPaymentRepo), nil, nil, nil, nil)

	repo.On("GetByID", mock.Anything, "student-1", "tutor-1").Return(models.Student{ID: "student-1"}, nil)
	repo.On("SetActive", mock.Anything, "student-1", "tutor-1", true).Return(nil)

	require.NoError(t, svc.Restore(context.Background(), "student-1", "tutor-1"))
	repo.AssertExpectations(t)
}

type mockDebtsSource struct{ mock.Mock }

func (m *mockDebtsSource) GetDebts(ctx context.Context, tutorID string) ([]models.StudentDebt, error) {
	args := m.Called(ctx, tutorID)
	return args.Get(0).([]models.StudentDebt), args.Error(1)
}

// Обзор компонует существующие методы одним ответом: активные курсы с
// балансом, долг этого ученика из общего списка, ближайший и последние уроки,
// последние платежи (спека, п. 7.1).
func TestStudentOverview_ComposesExistingData(t *testing.T) {
	repo := new(mockStudentRepo)
	payRepo := new(mockPaymentRepo)
	courses := new(mockCourseRepo)
	debts := new(mockDebtsSource)
	svc := service.NewStudentService(repo, payRepo, courses, nil, nil, debts)

	studentID := "stu-1"
	course := models.Course{ID: "c1", StudentID: &studentID, Subject: "Математика", PricePerCycle: 40000, LessonsPerCycle: 8}
	repo.On("GetByID", mock.Anything, studentID, "tutor-1").Return(models.Student{ID: studentID, FirstName: "Айгерим"}, nil)
	courses.On("GetByStudent", mock.Anything, studentID, "tutor-1").Return([]models.Course{course}, nil)
	payRepo.On("GetBalancesByStudent", mock.Anything, studentID, "tutor-1").
		Return(map[string]models.CourseBalance{"c1": {LessonsPaid: 8, LessonsCompleted: 3, LessonsRemaining: 5}}, nil)
	debts.On("GetDebts", mock.Anything, "tutor-1").Return([]models.StudentDebt{
		{StudentID: studentID, AmountOwed: 12000, Courses: []models.DebtByCourse{{CourseID: "c1", AmountOwed: 12000}}},
	}, nil)
	repo.On("ListLessons", mock.Anything, studentID, false).Return([]models.CalendarLesson{}, nil)
	repo.On("ListLessons", mock.Anything, studentID, true).Return([]models.CalendarLesson{}, nil)
	payRepo.On("GetByStudentBatch", mock.Anything, studentID).Return(map[string][]models.Payment{}, nil)

	got, err := svc.Overview(context.Background(), studentID, "tutor-1")

	assert.NoError(t, err)
	assert.Equal(t, "Айгерим", got.Student.FirstName)
	assert.Len(t, got.Courses, 1)
	assert.Equal(t, 5, got.Courses[0].Balance.LessonsRemaining)
	assert.Equal(t, 12000.0, got.TotalOwed)
	assert.Len(t, got.PayableCourses, 1) // c1 уже активен, дублировать не должен
	repo.AssertExpectations(t)
	payRepo.AssertExpectations(t)
	courses.AssertExpectations(t)
	debts.AssertExpectations(t)
}

// Ушедший из группы с долгом: курс не в активных, но обязан попасть в
// PayableCourses — иначе долг невозможно погасить (спека, п. 7.0).
func TestStudentOverview_PayableCoursesIncludeLeftCourseWithDebt(t *testing.T) {
	repo := new(mockStudentRepo)
	payRepo := new(mockPaymentRepo)
	courses := new(mockCourseRepo)
	debts := new(mockDebtsSource)
	svc := service.NewStudentService(repo, payRepo, courses, nil, nil, debts)

	studentID := "stu-1"
	leftCourse := models.Course{ID: "c-left", Subject: "Группа"}
	repo.On("GetByID", mock.Anything, studentID, "tutor-1").Return(models.Student{ID: studentID}, nil)
	courses.On("GetByStudent", mock.Anything, studentID, "tutor-1").Return([]models.Course{}, nil) // активных нет
	payRepo.On("GetBalancesByStudent", mock.Anything, studentID, "tutor-1").Return(map[string]models.CourseBalance{}, nil)
	debts.On("GetDebts", mock.Anything, "tutor-1").Return([]models.StudentDebt{
		{StudentID: studentID, AmountOwed: 5000, Courses: []models.DebtByCourse{{CourseID: "c-left", AmountOwed: 5000}}},
	}, nil)
	courses.On("GetByID", mock.Anything, "c-left", "tutor-1").Return(leftCourse, nil)
	repo.On("ListLessons", mock.Anything, studentID, false).Return([]models.CalendarLesson{}, nil)
	repo.On("ListLessons", mock.Anything, studentID, true).Return([]models.CalendarLesson{}, nil)
	payRepo.On("GetByStudentBatch", mock.Anything, studentID).Return(map[string][]models.Payment{}, nil)

	got, err := svc.Overview(context.Background(), studentID, "tutor-1")

	assert.NoError(t, err)
	assert.Empty(t, got.Courses) // на «Обзоре» его нет
	require.Len(t, got.PayableCourses, 1)
	assert.Equal(t, "c-left", got.PayableCourses[0].ID) // но заплатить есть чем
}
