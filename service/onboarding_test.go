package service_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"tutorgo/models"
	"tutorgo/service"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type mockStudentCreator struct{ mock.Mock }

func (m *mockStudentCreator) Create(ctx context.Context, req models.CreateStudentRequest, tutorID string) (models.Student, error) {
	args := m.Called(ctx, req, tutorID)
	return args.Get(0).(models.Student), args.Error(1)
}
func (m *mockStudentCreator) Delete(ctx context.Context, id, tutorID string) error {
	return m.Called(ctx, id, tutorID).Error(0)
}

type mockCourseCreator struct{ mock.Mock }

func (m *mockCourseCreator) Create(ctx context.Context, req models.CreateCourseRequest, tutorID string) (models.Course, error) {
	args := m.Called(ctx, req, tutorID)
	return args.Get(0).(models.Course), args.Error(1)
}
func (m *mockCourseCreator) Delete(ctx context.Context, id, tutorID string) error {
	return m.Called(ctx, id, tutorID).Error(0)
}

type mockSeriesCreator struct{ mock.Mock }

func (m *mockSeriesCreator) Create(ctx context.Context, req models.CreateLessonRequest, tutorID string) (models.Lesson, error) {
	args := m.Called(ctx, req, tutorID)
	return args.Get(0).(models.Lesson), args.Error(1)
}
func (m *mockSeriesCreator) GetByCourse(ctx context.Context, courseID, tutorID string) ([]models.Lesson, error) {
	args := m.Called(ctx, courseID, tutorID)
	return args.Get(0).([]models.Lesson), args.Error(1)
}

func onboardingSvc(s *mockStudentCreator, c *mockCourseCreator, l *mockSeriesCreator) service.OnboardingService {
	return service.NewOnboardingService(s, c, l)
}

// Обязательно только имя: репетитор, у которого пока нет ни предмета, ни
// расписания, всё равно должен уметь завести ученика одним сабмитом.
func TestOnboarding_NameOnly(t *testing.T) {
	students, courses, lessons := new(mockStudentCreator), new(mockCourseCreator), new(mockSeriesCreator)

	students.On("Create", mock.Anything, models.CreateStudentRequest{FirstName: "Айгерим"}, tutorID).
		Return(models.Student{ID: "st-1", FirstName: "Айгерим"}, nil)

	res, err := onboardingSvc(students, courses, lessons).CreateStudent(context.Background(),
		models.OnboardingStudentRequest{FirstName: "Айгерим"}, tutorID)

	require.NoError(t, err)
	require.Equal(t, "st-1", res.Student.ID)
	require.Nil(t, res.Course)
	require.Zero(t, res.LessonsCreated)
	courses.AssertNotCalled(t, "Create")
	lessons.AssertNotCalled(t, "Create")
}

// Предмет без расписания — курс есть, уроков нет: договорились заниматься,
// но время ещё не выбрали.
func TestOnboarding_SubjectWithoutSchedule(t *testing.T) {
	students, courses, lessons := new(mockStudentCreator), new(mockCourseCreator), new(mockSeriesCreator)

	students.On("Create", mock.Anything, mock.Anything, tutorID).Return(models.Student{ID: "st-1"}, nil)
	courses.On("Create", mock.Anything, mock.MatchedBy(func(r models.CreateCourseRequest) bool {
		return r.Subject == "Математика" && r.PricePerCycle == 5000 && r.LessonsPerCycle == 1 &&
			r.StudentID != nil && *r.StudentID == "st-1"
	}), tutorID).Return(models.Course{ID: "c-1", Subject: "Математика"}, nil)

	res, err := onboardingSvc(students, courses, lessons).CreateStudent(context.Background(),
		models.OnboardingStudentRequest{
			FirstName: "Айгерим", Subject: "Математика",
			PricePerCycle: 5000, LessonsPerCycle: 1,
		}, tutorID)

	require.NoError(t, err)
	require.NotNil(t, res.Course)
	require.Equal(t, "c-1", res.Course.ID)
	require.Zero(t, res.LessonsCreated)
	lessons.AssertNotCalled(t, "Create")
}

// Полный сабмит: первое занятие встаёт на 17:00 по Алматы того дня, с которого
// начинается расписание, а дни недели уезжают в правило повторения.
func TestOnboarding_FullSchedule(t *testing.T) {
	students, courses, lessons := new(mockStudentCreator), new(mockCourseCreator), new(mockSeriesCreator)

	students.On("Create", mock.Anything, mock.Anything, tutorID).Return(models.Student{ID: "st-1"}, nil)
	courses.On("Create", mock.Anything, mock.Anything, tutorID).Return(models.Course{ID: "c-1"}, nil)
	lessons.On("Create", mock.Anything, mock.MatchedBy(func(r models.CreateLessonRequest) bool {
		wall := r.ScheduledAt.In(mustLoad(t, "Asia/Almaty"))
		return r.CourseID == "c-1" && r.DurationMinutes == 60 &&
			wall.Hour() == 17 && wall.Minute() == 0 &&
			wall.Year() == 2026 && wall.Month() == time.September && wall.Day() == 2 &&
			r.Recurrence != nil && r.Recurrence.Freq == "weekly" &&
			r.Recurrence.TZ == "Asia/Almaty" && len(r.Recurrence.ByWeekday) == 2
	}), tutorID).Return(models.Lesson{ID: "l-1"}, nil)
	lessons.On("GetByCourse", mock.Anything, "c-1", tutorID).
		Return([]models.Lesson{{ID: "l-1"}, {ID: "l-2"}, {ID: "l-3"}}, nil)

	res, err := onboardingSvc(students, courses, lessons).CreateStudent(context.Background(),
		models.OnboardingStudentRequest{
			FirstName: "Айгерим", Subject: "Математика",
			Schedule: &models.OnboardingSchedule{
				ByWeekday: []int{1, 3}, TimeLocal: "17:00", TZ: "Asia/Almaty",
				DurationMinutes: 60, StartsOn: date(2026, time.September, 2),
			},
		}, tutorID)

	require.NoError(t, err)
	require.Equal(t, 3, res.LessonsCreated)
}

// Упавший курс не оставляет ученика-сироту: пользователь увидит ошибку и
// нажмёт «Добавить» снова, а в списке не будет пустой карточки от прошлой
// попытки.
func TestOnboarding_CourseFailureRemovesStudent(t *testing.T) {
	students, courses, lessons := new(mockStudentCreator), new(mockCourseCreator), new(mockSeriesCreator)

	students.On("Create", mock.Anything, mock.Anything, tutorID).Return(models.Student{ID: "st-1"}, nil)
	courses.On("Create", mock.Anything, mock.Anything, tutorID).
		Return(models.Course{}, errors.New("db is down"))
	students.On("Delete", mock.Anything, "st-1", tutorID).Return(nil)

	_, err := onboardingSvc(students, courses, lessons).CreateStudent(context.Background(),
		models.OnboardingStudentRequest{FirstName: "Айгерим", Subject: "Математика"}, tutorID)

	require.Error(t, err)
	students.AssertExpectations(t)
}

// То же для расписания: не создалась серия — не остаётся ни курса, ни ученика.
func TestOnboarding_LessonFailureRemovesCourseAndStudent(t *testing.T) {
	students, courses, lessons := new(mockStudentCreator), new(mockCourseCreator), new(mockSeriesCreator)

	students.On("Create", mock.Anything, mock.Anything, tutorID).Return(models.Student{ID: "st-1"}, nil)
	courses.On("Create", mock.Anything, mock.Anything, tutorID).Return(models.Course{ID: "c-1"}, nil)
	lessons.On("Create", mock.Anything, mock.Anything, tutorID).
		Return(models.Lesson{}, errors.New("db is down"))
	courses.On("Delete", mock.Anything, "c-1", tutorID).Return(nil)
	students.On("Delete", mock.Anything, "st-1", tutorID).Return(nil)

	_, err := onboardingSvc(students, courses, lessons).CreateStudent(context.Background(),
		models.OnboardingStudentRequest{
			FirstName: "Айгерим", Subject: "Математика",
			Schedule: &models.OnboardingSchedule{
				TimeLocal: "17:00", TZ: "Asia/Almaty", DurationMinutes: 60,
				StartsOn: date(2026, time.September, 2),
			},
		}, tutorID)

	require.Error(t, err)
	courses.AssertExpectations(t)
	students.AssertExpectations(t)
}

// Дата начала читается в зоне репетитора, а не в той, в которой её прислали.
// Полночь 2 сентября по Алматы — это 1 сентября 19:00 UTC; сервер обязан
// увидеть здесь 2 сентября, иначе серия начинается днём раньше у всех, чей
// клиент отправил дату в UTC.
func TestOnboarding_StartsOnReadInTutorZone(t *testing.T) {
	students, courses, lessons := new(mockStudentCreator), new(mockCourseCreator), new(mockSeriesCreator)

	students.On("Create", mock.Anything, mock.Anything, tutorID).Return(models.Student{ID: "st-1"}, nil)
	courses.On("Create", mock.Anything, mock.Anything, tutorID).Return(models.Course{ID: "c-1"}, nil)
	lessons.On("Create", mock.Anything, mock.MatchedBy(func(r models.CreateLessonRequest) bool {
		wall := r.ScheduledAt.In(mustLoad(t, "Asia/Almaty"))
		return wall.Day() == 2 && wall.Month() == time.September && wall.Hour() == 17
	}), tutorID).Return(models.Lesson{ID: "l-1"}, nil)
	lessons.On("GetByCourse", mock.Anything, "c-1", tutorID).Return([]models.Lesson{{ID: "l-1"}}, nil)

	_, err := onboardingSvc(students, courses, lessons).CreateStudent(context.Background(),
		models.OnboardingStudentRequest{
			FirstName: "Айгерим", Subject: "Математика",
			Schedule: &models.OnboardingSchedule{
				TimeLocal: "17:00", TZ: "Asia/Almaty", DurationMinutes: 60,
				StartsOn: time.Date(2026, time.September, 1, 19, 0, 0, 0, time.UTC),
			},
		}, tutorID)

	require.NoError(t, err)
	lessons.AssertExpectations(t)
}

// Неизвестная зона — ошибка запроса, а не паника и не молчаливый UTC.
func TestOnboarding_BadTimezone(t *testing.T) {
	students, courses, lessons := new(mockStudentCreator), new(mockCourseCreator), new(mockSeriesCreator)

	students.On("Create", mock.Anything, mock.Anything, tutorID).Return(models.Student{ID: "st-1"}, nil)
	courses.On("Create", mock.Anything, mock.Anything, tutorID).Return(models.Course{ID: "c-1"}, nil)
	courses.On("Delete", mock.Anything, "c-1", tutorID).Return(nil)
	students.On("Delete", mock.Anything, "st-1", tutorID).Return(nil)

	_, err := onboardingSvc(students, courses, lessons).CreateStudent(context.Background(),
		models.OnboardingStudentRequest{
			FirstName: "Айгерим", Subject: "Математика",
			Schedule: &models.OnboardingSchedule{
				TimeLocal: "17:00", TZ: "Mars/Olympus", DurationMinutes: 60,
				StartsOn: date(2026, time.September, 2),
			},
		}, tutorID)

	require.ErrorIs(t, err, service.ErrBadRequest)
	lessons.AssertNotCalled(t, "Create")
}

func mustLoad(t *testing.T, tz string) *time.Location {
	t.Helper()
	loc, err := time.LoadLocation(tz)
	require.NoError(t, err)
	return loc
}
