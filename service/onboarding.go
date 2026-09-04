package service

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"tutorgo/models"
)

// Онбординг ничего не умеет сам — он складывает три уже существующих сценария
// в один сабмит. Зависимости объявлены узкими интерфейсами, а не целыми
// сервисами: связность меньше, мок в тесте короче.

type studentCreator interface {
	Create(ctx context.Context, req models.CreateStudentRequest, tutorID string) (models.Student, error)
	Delete(ctx context.Context, id string, tutorID string) error
}

type courseCreator interface {
	Create(ctx context.Context, req models.CreateCourseRequest, tutorID string) (models.Course, error)
	Delete(ctx context.Context, id string, tutorID string) error
}

type seriesCreator interface {
	Create(ctx context.Context, req models.CreateLessonRequest, tutorID string) (models.Lesson, error)
	GetByCourse(ctx context.Context, courseID string, tutorID string) ([]models.Lesson, error)
}

type OnboardingService interface {
	CreateStudent(ctx context.Context, req models.OnboardingStudentRequest, tutorID string) (models.OnboardingResult, error)
}

type onboardingService struct {
	students studentCreator
	courses  courseCreator
	lessons  seriesCreator
}

func NewOnboardingService(students studentCreator, courses courseCreator, lessons seriesCreator) OnboardingService {
	return &onboardingService{students: students, courses: courses, lessons: lessons}
}

// CreateStudent заводит ученика, при указанном предмете — курс, при указанном
// расписании — серию уроков.
//
// ponytail: это сага с компенсацией, а не транзакция. Настоящая транзакция
// потребовала бы протащить pgx.Tx через все три сервиса и их репозитории —
// цена несоразмерна одной ручке. Упавший шаг откатывает уже созданное сам;
// если откат тоже упадёт, останется запись, которую видно в списке учеников и
// можно удалить руками. Перейти на общую транзакцию — когда таких составных
// сценариев станет больше одного.
func (s *onboardingService) CreateStudent(ctx context.Context, req models.OnboardingStudentRequest, tutorID string) (models.OnboardingResult, error) {
	student, err := s.students.Create(ctx, models.CreateStudentRequest{
		FirstName: req.FirstName,
		Phone:     req.Phone,
	}, tutorID)
	if err != nil {
		return models.OnboardingResult{}, err
	}
	if req.Subject == "" {
		return models.OnboardingResult{Student: student}, nil
	}

	lessonsPerCycle := req.LessonsPerCycle
	if lessonsPerCycle == 0 {
		lessonsPerCycle = 1
	}
	startedAt := time.Now()
	if req.Schedule != nil {
		startedAt = req.Schedule.StartsOn
	}

	course, err := s.courses.Create(ctx, models.CreateCourseRequest{
		StudentID:       &student.ID,
		Subject:         req.Subject,
		PricePerCycle:   req.PricePerCycle,
		LessonsPerCycle: lessonsPerCycle,
		StartedAt:       startedAt,
	}, tutorID)
	if err != nil {
		s.rollback(ctx, tutorID, student.ID, "")
		return models.OnboardingResult{}, err
	}
	if req.Schedule == nil {
		return models.OnboardingResult{Student: student, Course: &course}, nil
	}

	firstAt, err := firstOccurrenceAt(*req.Schedule)
	if err != nil {
		s.rollback(ctx, tutorID, student.ID, course.ID)
		return models.OnboardingResult{}, err
	}

	if _, err := s.lessons.Create(ctx, models.CreateLessonRequest{
		CourseID:        course.ID,
		ScheduledAt:     firstAt,
		DurationMinutes: req.Schedule.DurationMinutes,
		Recurrence: &models.RecurrenceInput{
			Freq:      "weekly",
			ByWeekday: req.Schedule.ByWeekday,
			TZ:        req.Schedule.TZ,
			EndsOn:    req.Schedule.EndsOn,
		},
	}, tutorID); err != nil {
		s.rollback(ctx, tutorID, student.ID, course.ID)
		return models.OnboardingResult{}, err
	}

	// Сколько вхождений реально легло в горизонт, знает только база: остальные
	// добьёт ночная джоба.
	created, err := s.lessons.GetByCourse(ctx, course.ID, tutorID)
	if err != nil {
		return models.OnboardingResult{Student: student, Course: &course}, nil
	}
	return models.OnboardingResult{Student: student, Course: &course, LessonsCreated: len(created)}, nil
}

// rollback убирает уже созданное. Ошибки отката только логируются: исходную
// ошибку они не отменяют, а пользователю нужна именно она.
func (s *onboardingService) rollback(ctx context.Context, tutorID, studentID, courseID string) {
	if courseID != "" {
		if err := s.courses.Delete(ctx, courseID, tutorID); err != nil {
			slog.Error("onboarding rollback: course", slog.String("course_id", courseID), slog.String("error", err.Error()))
		}
	}
	if err := s.students.Delete(ctx, studentID, tutorID); err != nil {
		slog.Error("onboarding rollback: student", slog.String("student_id", studentID), slog.String("error", err.Error()))
	}
}

// firstOccurrenceAt собирает первое занятие как стенные часы в зоне репетитора:
// та же сборка по календарной дате, что и в генераторе повторений, — иначе
// перевод часов уводил бы время на час.
func firstOccurrenceAt(sch models.OnboardingSchedule) (time.Time, error) {
	loc, err := time.LoadLocation(sch.TZ)
	if err != nil {
		return time.Time{}, fmt.Errorf("timezone %q: %w", sch.TZ, ErrBadRequest)
	}
	hm, err := time.Parse("15:04", sch.TimeLocal)
	if err != nil {
		return time.Time{}, fmt.Errorf("time_local %q: %w", sch.TimeLocal, ErrBadRequest)
	}
	// Дата читается в зоне репетитора: клиент вправе прислать её в UTC, и тогда
	// полночь 2 сентября по Алматы приезжает как 1 сентября 19:00 — серия
	// началась бы днём раньше.
	d := sch.StartsOn.In(loc)
	return time.Date(d.Year(), d.Month(), d.Day(), hm.Hour(), hm.Minute(), 0, 0, loc), nil
}
