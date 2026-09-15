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

type mockPaymentRepo struct {
	mock.Mock
}

func (m *mockPaymentRepo) Create(ctx context.Context, req models.CreatePaymentRequest) (models.Payment, error) {
	args := m.Called(ctx, req)
	return args.Get(0).(models.Payment), args.Error(1)
}

func (m *mockPaymentRepo) GetByID(ctx context.Context, id string, tutorID string) (models.Payment, error) {
	args := m.Called(ctx, id, tutorID)
	return args.Get(0).(models.Payment), args.Error(1)
}

func (m *mockPaymentRepo) GetByCourse(ctx context.Context, courseID string, p models.Pagination) ([]models.Payment, int, error) {
	args := m.Called(ctx, courseID, p)
	return args.Get(0).([]models.Payment), args.Int(1), args.Error(2)
}

func (m *mockPaymentRepo) GetAllByTutor(ctx context.Context, tutorID string, limit int) ([]models.Payment, error) {
	args := m.Called(ctx, tutorID, limit)
	return args.Get(0).([]models.Payment), args.Error(1)
}

func (m *mockPaymentRepo) GetAllByTutorPaged(ctx context.Context, tutorID string, p models.Pagination) ([]models.Payment, int, error) {
	args := m.Called(ctx, tutorID, p)
	return args.Get(0).([]models.Payment), args.Int(1), args.Error(2)
}

func (m *mockPaymentRepo) GetBalance(ctx context.Context, courseID, studentID string) (models.CourseBalance, error) {
	args := m.Called(ctx, courseID, studentID)
	return args.Get(0).(models.CourseBalance), args.Error(1)
}

func (m *mockPaymentRepo) GetMonthlyIncome(ctx context.Context, tutorID string) (float64, error) {
	args := m.Called(ctx, tutorID)
	return args.Get(0).(float64), args.Error(1)
}

func (m *mockPaymentRepo) GetMonthlyExpected(ctx context.Context, tutorID string) (float64, error) {
	args := m.Called(ctx, tutorID)
	return args.Get(0).(float64), args.Error(1)
}

func (m *mockPaymentRepo) Update(ctx context.Context, id string, tutorID string, req models.UpdatePaymentRequest) (models.Payment, error) {
	args := m.Called(ctx, id, tutorID, req)
	return args.Get(0).(models.Payment), args.Error(1)
}

func (m *mockPaymentRepo) Delete(ctx context.Context, id string, tutorID string) error {
	args := m.Called(ctx, id, tutorID)
	return args.Error(0)
}

func (m *mockPaymentRepo) GetByCoursesBatch(ctx context.Context, courseIDs []string) (map[string][]models.Payment, error) {
	args := m.Called(ctx, courseIDs)
	return args.Get(0).(map[string][]models.Payment), args.Error(1)
}

func (m *mockPaymentRepo) GetByStudentBatch(ctx context.Context, studentID string) (map[string][]models.Payment, error) {
	args := m.Called(ctx, studentID)
	return args.Get(0).(map[string][]models.Payment), args.Error(1)
}

func (m *mockPaymentRepo) GetPaymentsForCalendar(ctx context.Context, tutorID string, from string, to string) (map[string][]models.Payment, error) {
	args := m.Called(ctx, tutorID, from, to)
	return args.Get(0).(map[string][]models.Payment), args.Error(1)
}

func (m *mockPaymentRepo) GetDebts(ctx context.Context, tutorID string) ([]models.CourseDebt, error) {
	args := m.Called(ctx, tutorID)
	return args.Get(0).([]models.CourseDebt), args.Error(1)
}

func (m *mockPaymentRepo) CreateBulk(ctx context.Context, req models.CreateBulkPaymentRequest) ([]models.Payment, error) {
	args := m.Called(ctx, req)
	return args.Get(0).([]models.Payment), args.Error(1)
}

var (
	tutorID  = "tutor-uuid-1"
	courseID = "course-uuid-1"

	payStudentID = "student-uuid-1"

	paymentReq = models.CreatePaymentRequest{
		CourseID:     courseID,
		StudentID:    payStudentID,
		Amount:       5000,
		LessonsCount: 12,
		PaidAt:       time.Date(2001, time.September, 11, 0, 0, 0, 0, time.UTC),
	}

	expectedPayment = models.Payment{
		ID:           "payment-uuid-1",
		CourseID:     courseID,
		StudentID:    &payStudentID,
		Amount:       5000,
		LessonsCount: 12,
		PaidAt:       time.Date(2001, time.September, 11, 0, 0, 0, 0, time.UTC),
	}

	expectedCourse = models.Course{
		ID:       courseID,
		TutorID:  tutorID,
		IsActive: true,
	}

	// individualCourse — курс ученика payStudentID: платёж ему проходит проверку
	// связи без записи в группу.
	individualCourse = models.Course{
		ID:        courseID,
		TutorID:   tutorID,
		StudentID: &payStudentID,
		IsActive:  true,
	}
)

func newPaymentSvc(payRepo *mockPaymentRepo, courseRepo *mockCourseRepo, enrollRepo *mockEnrollmentRepo) service.PaymentService {
	return service.NewPaymentService(payRepo, courseRepo, enrollRepo)
}

// Create

func TestPaymentCreate_Success(t *testing.T) {
	payRepo := new(mockPaymentRepo)
	courseRepo := new(mockCourseRepo)
	svc := newPaymentSvc(payRepo, courseRepo, new(mockEnrollmentRepo))

	courseRepo.On("GetByID", mock.Anything, courseID, tutorID).Return(individualCourse, nil)
	payRepo.On("Create", mock.Anything, paymentReq).Return(expectedPayment, nil)

	payment, err := svc.Create(context.Background(), paymentReq, tutorID)

	assert.NoError(t, err)
	assert.Equal(t, expectedPayment, payment)
	courseRepo.AssertExpectations(t)
	payRepo.AssertExpectations(t)
}

func TestPaymentCreate_CourseNotFound(t *testing.T) {
	payRepo := new(mockPaymentRepo)
	courseRepo := new(mockCourseRepo)
	svc := newPaymentSvc(payRepo, courseRepo, new(mockEnrollmentRepo))

	courseRepo.On("GetByID", mock.Anything, courseID, tutorID).Return(models.Course{}, errors.New("not found"))

	payment, err := svc.Create(context.Background(), paymentReq, tutorID)

	assert.ErrorIs(t, err, service.ErrNotFound)
	assert.Empty(t, payment)
	payRepo.AssertNotCalled(t, "Create")
	courseRepo.AssertExpectations(t)
}

func TestPaymentCreate_RepoError(t *testing.T) {
	payRepo := new(mockPaymentRepo)
	courseRepo := new(mockCourseRepo)
	svc := newPaymentSvc(payRepo, courseRepo, new(mockEnrollmentRepo))

	courseRepo.On("GetByID", mock.Anything, courseID, tutorID).Return(individualCourse, nil)
	payRepo.On("Create", mock.Anything, paymentReq).Return(models.Payment{}, errors.New("db error"))

	payment, err := svc.Create(context.Background(), paymentReq, tutorID)

	assert.Error(t, err)
	assert.Empty(t, payment)
	courseRepo.AssertExpectations(t)
	payRepo.AssertExpectations(t)
}

// Индивидуальный курс чужого ученика — 400, в базу ничего не пишется (спека, п. 6.3).
func TestPaymentCreate_IndividualCourseOtherStudentRejected(t *testing.T) {
	payRepo := new(mockPaymentRepo)
	courseRepo := new(mockCourseRepo)
	svc := newPaymentSvc(payRepo, courseRepo, new(mockEnrollmentRepo))

	other := "student-uuid-2"
	course := individualCourse
	course.StudentID = &other
	courseRepo.On("GetByID", mock.Anything, courseID, tutorID).Return(course, nil)

	_, err := svc.Create(context.Background(), paymentReq, tutorID)

	assert.ErrorIs(t, err, service.ErrBadRequest)
	payRepo.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
}

func TestPaymentCreate_GroupRequiresEnrollment(t *testing.T) {
	for _, tc := range []struct {
		name     string
		enrolled bool
	}{{"enrolled", true}, {"stranger", false}} {
		t.Run(tc.name, func(t *testing.T) {
			payRepo := new(mockPaymentRepo)
			courseRepo := new(mockCourseRepo)
			enrollRepo := new(mockEnrollmentRepo)
			svc := newPaymentSvc(payRepo, courseRepo, enrollRepo)

			courseRepo.On("GetByID", mock.Anything, courseID, tutorID).Return(expectedCourse, nil)
			enrollRepo.On("IsEnrolled", mock.Anything, courseID, payStudentID).Return(tc.enrolled, nil)
			if tc.enrolled {
				payRepo.On("Create", mock.Anything, paymentReq).Return(expectedPayment, nil)
			}

			_, err := svc.Create(context.Background(), paymentReq, tutorID)

			if tc.enrolled {
				assert.NoError(t, err)
			} else {
				assert.ErrorIs(t, err, service.ErrBadRequest)
				payRepo.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
			}
			enrollRepo.AssertExpectations(t)
		})
	}
}

// Правка адресата проверяется так же, как создание: иначе легаси-платёж группы
// можно «починить» на ученика, которого в группе не было.
func TestPaymentUpdate_ChecksStudentOnCourse(t *testing.T) {
	payRepo := new(mockPaymentRepo)
	courseRepo := new(mockCourseRepo)
	enrollRepo := new(mockEnrollmentRepo)
	svc := newPaymentSvc(payRepo, courseRepo, enrollRepo)

	req := models.UpdatePaymentRequest{StudentID: payStudentID, Amount: 5000, LessonsCount: 4, PaidAt: paymentReq.PaidAt}
	payRepo.On("GetByID", mock.Anything, "payment-uuid-1", tutorID).Return(models.Payment{ID: "payment-uuid-1", CourseID: courseID}, nil)
	courseRepo.On("GetByID", mock.Anything, courseID, tutorID).Return(expectedCourse, nil)
	enrollRepo.On("IsEnrolled", mock.Anything, courseID, payStudentID).Return(false, nil)

	_, err := svc.Update(context.Background(), "payment-uuid-1", tutorID, req)

	assert.ErrorIs(t, err, service.ErrBadRequest)
	payRepo.AssertNotCalled(t, "Update", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func TestPaymentUpdate_PaymentNotFound(t *testing.T) {
	payRepo := new(mockPaymentRepo)
	svc := newPaymentSvc(payRepo, new(mockCourseRepo), new(mockEnrollmentRepo))

	payRepo.On("GetByID", mock.Anything, "missing", tutorID).Return(models.Payment{}, errors.New("no rows"))

	_, err := svc.Update(context.Background(), "missing", tutorID, models.UpdatePaymentRequest{StudentID: payStudentID})

	assert.ErrorIs(t, err, service.ErrNotFound)
}

// GetByCourse

func TestPaymentGetByCourse_Success(t *testing.T) {
	payRepo := new(mockPaymentRepo)
	courseRepo := new(mockCourseRepo)
	svc := newPaymentSvc(payRepo, courseRepo, new(mockEnrollmentRepo))

	p := models.Pagination{Page: 1, Limit: 20}
	expected := []models.Payment{expectedPayment}
	courseRepo.On("GetByID", mock.Anything, courseID, tutorID).Return(expectedCourse, nil)
	payRepo.On("GetByCourse", mock.Anything, courseID, p).Return(expected, 1, nil)

	payments, total, err := svc.GetByCourse(context.Background(), courseID, tutorID, p)

	assert.NoError(t, err)
	assert.Equal(t, expected, payments)
	assert.Equal(t, 1, total)
	courseRepo.AssertExpectations(t)
	payRepo.AssertExpectations(t)
}

func TestPaymentGetByCourse_CourseNotFound(t *testing.T) {
	payRepo := new(mockPaymentRepo)
	courseRepo := new(mockCourseRepo)
	svc := newPaymentSvc(payRepo, courseRepo, new(mockEnrollmentRepo))

	p := models.Pagination{Page: 1, Limit: 20}
	courseRepo.On("GetByID", mock.Anything, courseID, tutorID).Return(models.Course{}, errors.New("not found"))

	payments, total, err := svc.GetByCourse(context.Background(), courseID, tutorID, p)

	assert.ErrorIs(t, err, service.ErrNotFound)
	assert.Nil(t, payments)
	assert.Equal(t, 0, total)
	payRepo.AssertNotCalled(t, "GetByCourse")
	courseRepo.AssertExpectations(t)
}

// GetBalance

func TestPaymentGetBalance_Success(t *testing.T) {
	payRepo := new(mockPaymentRepo)
	courseRepo := new(mockCourseRepo)
	svc := newPaymentSvc(payRepo, courseRepo, new(mockEnrollmentRepo))

	expected := models.CourseBalance{
		LessonsPaid:      10,
		LessonsCompleted: 3,
		LessonsRemaining: 7,
	}
	courseRepo.On("GetByID", mock.Anything, courseID, tutorID).Return(individualCourse, nil)
	payRepo.On("GetBalance", mock.Anything, courseID, payStudentID).Return(expected, nil)

	balance, err := svc.GetBalance(context.Background(), courseID, payStudentID, tutorID)

	assert.NoError(t, err)
	assert.Equal(t, expected, balance)
	courseRepo.AssertExpectations(t)
	payRepo.AssertExpectations(t)
}

func TestPaymentGetBalance_CourseNotFound(t *testing.T) {
	payRepo := new(mockPaymentRepo)
	courseRepo := new(mockCourseRepo)
	svc := newPaymentSvc(payRepo, courseRepo, new(mockEnrollmentRepo))

	courseRepo.On("GetByID", mock.Anything, courseID, tutorID).Return(models.Course{}, errors.New("not found"))

	balance, err := svc.GetBalance(context.Background(), courseID, payStudentID, tutorID)

	assert.ErrorIs(t, err, service.ErrNotFound)
	assert.Empty(t, balance)
	payRepo.AssertNotCalled(t, "GetBalance")
	courseRepo.AssertExpectations(t)
}

// Баланс чужого для курса ученика — 400, а не нули: нули выглядели бы как
// «всё оплачено» (спека, п. 6.3).
func TestPaymentGetBalance_StudentNotOnCourse(t *testing.T) {
	payRepo := new(mockPaymentRepo)
	courseRepo := new(mockCourseRepo)
	svc := newPaymentSvc(payRepo, courseRepo, new(mockEnrollmentRepo))

	courseRepo.On("GetByID", mock.Anything, courseID, tutorID).Return(individualCourse, nil)

	_, err := svc.GetBalance(context.Background(), courseID, "student-uuid-2", tutorID)

	assert.ErrorIs(t, err, service.ErrBadRequest)
	payRepo.AssertNotCalled(t, "GetBalance", mock.Anything, mock.Anything, mock.Anything)
}

// GetMonthlyExpected

func TestPaymentGetMonthlyExpected_Success(t *testing.T) {
	payRepo := new(mockPaymentRepo)
	courseRepo := new(mockCourseRepo)
	svc := newPaymentSvc(payRepo, courseRepo, new(mockEnrollmentRepo))

	payRepo.On("GetMonthlyExpected", mock.Anything, tutorID).Return(75000.0, nil)

	result, err := svc.GetMonthlyExpected(context.Background(), tutorID)

	assert.NoError(t, err)
	assert.Equal(t, 75000.0, result)
	payRepo.AssertExpectations(t)
}

func TestPaymentGetMonthlyExpected_RepoError(t *testing.T) {
	payRepo := new(mockPaymentRepo)
	courseRepo := new(mockCourseRepo)
	svc := newPaymentSvc(payRepo, courseRepo, new(mockEnrollmentRepo))

	payRepo.On("GetMonthlyExpected", mock.Anything, tutorID).Return(0.0, errors.New("db error"))

	result, err := svc.GetMonthlyExpected(context.Background(), tutorID)

	assert.Error(t, err)
	assert.Equal(t, 0.0, result)
	payRepo.AssertExpectations(t)
}

// GetDebts

// Строки курсов складываются в строку на ученика: сумма — уроки × цена урока
// своего курса, напоминание — к ближайшему уроку, порядок — по сумме долга.
func TestPaymentGetDebts_GroupsByStudentAndSortsByAmount(t *testing.T) {
	payRepo := new(mockPaymentRepo)
	svc := newPaymentSvc(payRepo, new(mockCourseRepo), new(mockEnrollmentRepo))

	soon := time.Date(2026, time.September, 20, 10, 0, 0, 0, time.UTC)
	later := soon.Add(48 * time.Hour)
	payRepo.On("GetDebts", mock.Anything, tutorID).Return([]models.CourseDebt{
		{StudentID: "s1", StudentName: "Айгерим", CourseID: "math", Subject: "Математика", LessonsOwed: 2, LessonPrice: 5000, NextLessonAt: &later},
		{StudentID: "s1", StudentName: "Айгерим", CourseID: "phys", Subject: "Физика", LessonsOwed: 1, LessonPrice: 6000, NextLessonAt: &soon},
		{StudentID: "s2", StudentName: "Бекзат", CourseID: "eng", Subject: "Английский", LessonsOwed: 5, LessonPrice: 7083},
	}, nil)

	debts, err := svc.GetDebts(context.Background(), tutorID)

	assert.NoError(t, err)
	if assert.Len(t, debts, 2) {
		assert.Equal(t, "s2", debts[0].StudentID) // 35 415 ₸ больше 16 000 ₸
		assert.Equal(t, 35415.0, debts[0].AmountOwed)
		assert.Nil(t, debts[0].NextLessonAt)

		assert.Equal(t, "s1", debts[1].StudentID)
		assert.Equal(t, 3, debts[1].LessonsOwed)
		assert.Equal(t, 16000.0, debts[1].AmountOwed)
		assert.Equal(t, soon, *debts[1].NextLessonAt)
		assert.Equal(t, []models.DebtByCourse{
			{CourseID: "math", Subject: "Математика", LessonsOwed: 2, AmountOwed: 10000},
			{CourseID: "phys", Subject: "Физика", LessonsOwed: 1, AmountOwed: 6000},
		}, debts[1].Courses)
	}
}

// Никто не должен — пустой список, а не null: фронт рисует пустое состояние.
func TestPaymentGetDebts_EmptyIsNotNil(t *testing.T) {
	payRepo := new(mockPaymentRepo)
	svc := newPaymentSvc(payRepo, new(mockCourseRepo), new(mockEnrollmentRepo))
	payRepo.On("GetDebts", mock.Anything, tutorID).Return([]models.CourseDebt{}, nil)

	debts, err := svc.GetDebts(context.Background(), tutorID)

	assert.NoError(t, err)
	assert.NotNil(t, debts)
	assert.Empty(t, debts)
}

// CreateBulk

func bulkReq(items ...models.BulkPaymentItem) models.CreateBulkPaymentRequest {
	return models.CreateBulkPaymentRequest{PaidAt: paymentReq.PaidAt, Items: items}
}

// Ученик не на курсе во второй строке — 400 до записи: ничего не пишется
// (спека, п. 6.8).
func TestPaymentCreateBulk_SecondItemRejectedNothingWritten(t *testing.T) {
	payRepo := new(mockPaymentRepo)
	courseRepo := new(mockCourseRepo)
	svc := newPaymentSvc(payRepo, courseRepo, new(mockEnrollmentRepo))

	other := "student-uuid-2"
	foreign := models.Course{ID: "course-uuid-2", TutorID: tutorID, StudentID: &other, IsActive: true}
	courseRepo.On("GetByID", mock.Anything, courseID, tutorID).Return(individualCourse, nil)
	courseRepo.On("GetByID", mock.Anything, "course-uuid-2", tutorID).Return(foreign, nil)

	_, err := svc.CreateBulk(context.Background(), bulkReq(
		models.BulkPaymentItem{CourseID: courseID, StudentID: payStudentID, Amount: 40000, LessonsCount: 8},
		models.BulkPaymentItem{CourseID: "course-uuid-2", StudentID: payStudentID, Amount: 6000, LessonsCount: 1},
	), tutorID)

	assert.ErrorIs(t, err, service.ErrBadRequest)
	payRepo.AssertNotCalled(t, "CreateBulk", mock.Anything, mock.Anything)
}

func TestPaymentCreateBulk_ChecksEveryItemThenWritesOnce(t *testing.T) {
	payRepo := new(mockPaymentRepo)
	courseRepo := new(mockCourseRepo)
	enrollRepo := new(mockEnrollmentRepo)
	svc := newPaymentSvc(payRepo, courseRepo, enrollRepo)

	group := models.Course{ID: "group-uuid", TutorID: tutorID, IsActive: true}
	req := bulkReq(
		models.BulkPaymentItem{CourseID: courseID, StudentID: payStudentID, Amount: 40000, LessonsCount: 8},
		models.BulkPaymentItem{CourseID: "group-uuid", StudentID: payStudentID, Amount: 20000, LessonsCount: 4},
	)
	courseRepo.On("GetByID", mock.Anything, courseID, tutorID).Return(individualCourse, nil)
	courseRepo.On("GetByID", mock.Anything, "group-uuid", tutorID).Return(group, nil)
	enrollRepo.On("IsEnrolled", mock.Anything, "group-uuid", payStudentID).Return(true, nil)
	payRepo.On("CreateBulk", mock.Anything, req).Return([]models.Payment{{ID: "p1"}, {ID: "p2"}}, nil)

	payments, err := svc.CreateBulk(context.Background(), req, tutorID)

	assert.NoError(t, err)
	assert.Len(t, payments, 2)
	payRepo.AssertNumberOfCalls(t, "CreateBulk", 1)
	enrollRepo.AssertExpectations(t)
}
