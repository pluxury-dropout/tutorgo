package service

import (
	"context"
	"fmt"
	"time"
	"tutorgo/models"
	"tutorgo/repository"
)

// Архивация складывает уже существующие действия — зависимости узкими
// интерфейсами, как в onboarding.go.

// studentCourses — курсы ученика и их архивация. В проде это courseService, а
// не courseRepo: у репозитория тот же Delete, но без закрытия правил и удаления
// будущих уроков — архивный курс продолжал бы материализовать расписание.
type studentCourses interface {
	GetByStudent(ctx context.Context, studentID string, tutorID string) ([]models.Course, error)
	Delete(ctx context.Context, id string, tutorID string) error
}

type enrollmentLeaver interface {
	LeaveAllByStudent(ctx context.Context, studentID string) error
}

type studentSessions interface {
	DeleteByStudentID(ctx context.Context, studentID string) error
}

type StudentService interface {
	Create(ctx context.Context, req models.CreateStudentRequest, tutorID string) (models.Student, error)
	GetAll(ctx context.Context, tutorID string, p models.Pagination, archived bool) ([]models.Student, int, error)
	GetByID(ctx context.Context, id string, tutorID string) (models.Student, error)
	Update(ctx context.Context, id string, tutorID string, req models.UpdateStudentRequest) (models.Student, error)
	Delete(ctx context.Context, id string, tutorID string) error
	Archive(ctx context.Context, id string, tutorID string) error
	Restore(ctx context.Context, id string, tutorID string) error
	SetInvite(ctx context.Context, studentID, token string, expiresAt time.Time) error
	GetByInviteToken(ctx context.Context, token string) (string, time.Time, error)
	ActivateAccount(ctx context.Context, studentID, username, passwordHash string) error
	GetCredentialsByLogin(ctx context.Context, identifier string) (string, string, error)
	EnrolledInLesson(ctx context.Context, studentID, lessonID string) (bool, error)
	CourseAndTutorForLesson(ctx context.Context, lessonID string) (string, string, error)
	GetProfile(ctx context.Context, studentID string) (models.StudentProfile, error)
	ListLessons(ctx context.Context, studentID string, past bool) ([]models.CalendarLesson, error)
	GetPasswordHash(ctx context.Context, studentID string) (string, error)
	UpdatePassword(ctx context.Context, studentID, hash string) error
	ListHomework(ctx context.Context, studentID string) ([]models.StudentHomework, error)
	ListCourses(ctx context.Context, studentID string) ([]models.StudentCourse, error)
}

type studentService struct {
	repo        repository.StudentRepository
	paymentRepo repository.PaymentRepository
	courses     studentCourses
	enrollments enrollmentLeaver
	sessions    studentSessions
}

func NewStudentService(repo repository.StudentRepository, paymentRepo repository.PaymentRepository,
	courses studentCourses, enrollments enrollmentLeaver, sessions studentSessions) StudentService {
	return &studentService{repo: repo, paymentRepo: paymentRepo, courses: courses, enrollments: enrollments, sessions: sessions}
}

func (s *studentService) Create(ctx context.Context, req models.CreateStudentRequest, tutorID string) (models.Student, error) {
	return s.repo.Create(ctx, req, tutorID)
}

func (s *studentService) GetAll(ctx context.Context, tutorID string, p models.Pagination, archived bool) ([]models.Student, int, error) {
	return s.repo.GetAll(ctx, tutorID, p, archived)
}

func (s *studentService) GetByID(ctx context.Context, id string, tutorID string) (models.Student, error) {
	student, err := s.repo.GetByID(ctx, id, tutorID)
	if err != nil {
		return models.Student{}, fmt.Errorf("student: %w", ErrNotFound)
	}
	return student, nil
}

func (s *studentService) Update(ctx context.Context, id string, tutorID string, req models.UpdateStudentRequest) (models.Student, error) {
	if _, err := s.repo.GetByID(ctx, id, tutorID); err != nil {
		return models.Student{}, fmt.Errorf("student: %w", ErrNotFound)
	}
	return s.repo.Update(ctx, id, tutorID, req)
}

// Delete удаляет ученика без истории. С историей — ErrConflict: удаление стёрло
// бы платежи и уроки, фронт предлагает архив (спека, п. 5a.2).
func (s *studentService) Delete(ctx context.Context, id string, tutorID string) error {
	if _, err := s.repo.GetByID(ctx, id, tutorID); err != nil {
		return fmt.Errorf("student: %w", ErrNotFound)
	}
	deleted, err := s.repo.Delete(ctx, id, tutorID)
	if err != nil {
		return err
	}
	if !deleted {
		return fmt.Errorf("student has history: %w", ErrConflict)
	}
	return nil
}

// Archive убирает ученика с историей из работы, ничего не стирая (спека, п. 5a.3).
//
// Транзакции нет: каждый шаг идемпотентен, а active = false стоит последним и
// служит признаком «архивация завершена». Сбой посередине оставляет ученика в
// списке — повторный вызов доделает, уже архивные курсы в GetByStudent не попадут.
//
// ponytail: access-JWT ученика живёт 30 дней, а AuthStudent в базу не ходит —
// открытая сессия до истечения видит в кабинете свою историю; отзываются только
// refresh-токены. Проверка active в AuthStudent — если окно станет проблемой.
func (s *studentService) Archive(ctx context.Context, id string, tutorID string) error {
	if _, err := s.repo.GetByID(ctx, id, tutorID); err != nil {
		return fmt.Errorf("student: %w", ErrNotFound)
	}
	courses, err := s.courses.GetByStudent(ctx, id, tutorID)
	if err != nil {
		return err
	}
	for _, c := range courses {
		// Группа идёт для остальных — из неё ученик уходит через left_at ниже.
		if c.StudentID == nil {
			continue
		}
		if err := s.courses.Delete(ctx, c.ID, tutorID); err != nil {
			return err
		}
	}
	if err := s.enrollments.LeaveAllByStudent(ctx, id); err != nil {
		return err
	}
	if err := s.sessions.DeleteByStudentID(ctx, id); err != nil {
		return err
	}
	return s.repo.SetActive(ctx, id, tutorID, false)
}

// Restore возвращает ученика в список — и только: курсы восстанавливаются из
// архива курсов, в группу добавляют заново (спека, п. 5a.3).
func (s *studentService) Restore(ctx context.Context, id string, tutorID string) error {
	if _, err := s.repo.GetByID(ctx, id, tutorID); err != nil {
		return fmt.Errorf("student: %w", ErrNotFound)
	}
	return s.repo.SetActive(ctx, id, tutorID, true)
}

func (s *studentService) SetInvite(ctx context.Context, studentID, token string, expiresAt time.Time) error {
	return s.repo.SetInvite(ctx, studentID, token, expiresAt)
}

func (s *studentService) GetByInviteToken(ctx context.Context, token string) (string, time.Time, error) {
	return s.repo.GetByInviteToken(ctx, token)
}

func (s *studentService) ActivateAccount(ctx context.Context, studentID, username, passwordHash string) error {
	return s.repo.ActivateAccount(ctx, studentID, username, passwordHash)
}

func (s *studentService) GetCredentialsByLogin(ctx context.Context, identifier string) (string, string, error) {
	return s.repo.GetCredentialsByLogin(ctx, identifier)
}

func (s *studentService) EnrolledInLesson(ctx context.Context, studentID, lessonID string) (bool, error) {
	return s.repo.EnrolledInLesson(ctx, studentID, lessonID)
}

func (s *studentService) CourseAndTutorForLesson(ctx context.Context, lessonID string) (string, string, error) {
	return s.repo.CourseAndTutorForLesson(ctx, lessonID)
}

func (s *studentService) GetProfile(ctx context.Context, studentID string) (models.StudentProfile, error) {
	return s.repo.GetProfile(ctx, studentID)
}

func (s *studentService) ListLessons(ctx context.Context, studentID string, past bool) ([]models.CalendarLesson, error) {
	lessons, err := s.repo.ListLessons(ctx, studentID, past)
	if err != nil {
		return nil, err
	}
	// Циклы считаются от платежей — как в tutor-календаре (GetCalendar).
	seen := map[string]bool{}
	courseIDs := []string{}
	for _, l := range lessons {
		if l.Rank != nil && !seen[l.CourseID] {
			seen[l.CourseID] = true
			courseIDs = append(courseIDs, l.CourseID)
		}
	}
	if len(courseIDs) == 0 {
		return lessons, nil
	}
	paymentsMap, err := s.paymentRepo.GetByCoursesBatch(ctx, courseIDs)
	if err != nil {
		return nil, err
	}
	for i, l := range lessons {
		if l.Rank == nil {
			continue
		}
		coursePayments := paymentsMap[l.CourseID]
		if len(coursePayments) == 0 {
			continue
		}
		pos, size := cyclePositionFromRank(*l.Rank, coursePayments)
		paid := pos > 0
		lessons[i].Paid = &paid
		if pos > 0 {
			p, sz := pos, size
			lessons[i].CyclePosition = &p
			lessons[i].CycleSize = &sz
		}
	}
	return lessons, nil
}

func (s *studentService) GetPasswordHash(ctx context.Context, studentID string) (string, error) {
	return s.repo.GetPasswordHash(ctx, studentID)
}

func (s *studentService) UpdatePassword(ctx context.Context, studentID, hash string) error {
	return s.repo.UpdatePassword(ctx, studentID, hash)
}

func (s *studentService) ListHomework(ctx context.Context, studentID string) ([]models.StudentHomework, error) {
	return s.repo.ListHomework(ctx, studentID)
}

func (s *studentService) ListCourses(ctx context.Context, studentID string) ([]models.StudentCourse, error) {
	return s.repo.ListCourses(ctx, studentID)
}
