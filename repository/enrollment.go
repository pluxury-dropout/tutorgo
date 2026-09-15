package repository

import (
	"context"
	"tutorgo/models"

	"github.com/jackc/pgx/v5/pgxpool"
)

type EnrollmentRepository interface {
	Add(ctx context.Context, courseID string, studentID string) (models.CourseEnrollment, error)
	AddBulk(ctx context.Context, courseID string, studentIDs []string, tutorID string) ([]models.CourseEnrollment, error)
	Remove(ctx context.Context, courseID string, studentID string) error
	GetByCourse(ctx context.Context, courseID string) ([]models.CourseEnrollment, error)
	LeaveAllByStudent(ctx context.Context, studentID string) error
	IsEnrolled(ctx context.Context, courseID, studentID string) (bool, error)
}

type enrollmentRepository struct {
	pool *pgxpool.Pool
}

func NewEnrollmentRepository(pool *pgxpool.Pool) EnrollmentRepository {
	return &enrollmentRepository{pool: pool}
}

// Add записывает ученика в группу. Возвращение ушедшего снимает left_at: строка
// одна на пару (UNIQUE), и запись продолжается. Запись уже состоящего ничего не
// обновляет — RETURNING пуст, и это ошибка, как раньше была ошибка уникальности.
//
// ponytail: возвращение склеивает периоды участия в один — после фазы 2 уроки
// между уходом и возвращением сгорят. Таблица периодов — когда «ушёл и вернулся
// в ту же группу» станет частым (спека, п. 5a.4).
func (r *enrollmentRepository) Add(ctx context.Context, courseID string, studentID string) (models.CourseEnrollment, error) {
	var e models.CourseEnrollment
	err := r.pool.QueryRow(ctx,
		`INSERT INTO course_enrollments (course_id, student_id)
		 VALUES ($1, $2)
		 ON CONFLICT (course_id, student_id) DO UPDATE SET left_at = NULL
		 WHERE course_enrollments.left_at IS NOT NULL
		 RETURNING id, course_id, student_id`,
		courseID, studentID,
	).Scan(&e.ID, &e.CourseID, &e.StudentID)
	return e, err
}

// AddBulk записывает в группу сразу нескольких учеников. Вставка идёт SELECT'ом
// из students — чужие ученики отсеиваются там же, отдельной проверкой владения
// по одному запросу на каждого. Повторная запись уже состоящего в группе
// молча пропускается: собрать группу дважды — не ошибка пользователя. Ушедший
// (left_at задан) возвращается в группу той же строкой.
func (r *enrollmentRepository) AddBulk(ctx context.Context, courseID string, studentIDs []string, tutorID string) ([]models.CourseEnrollment, error) {
	rows, err := r.pool.Query(ctx,
		`INSERT INTO course_enrollments (course_id, student_id)
		 SELECT $1::uuid, s.id
		 FROM students s
		 WHERE s.tutor_id = $2::uuid AND s.id = ANY($3::uuid[]) AND s.active
		 ON CONFLICT (course_id, student_id) DO UPDATE SET left_at = NULL
		 WHERE course_enrollments.left_at IS NOT NULL
		 RETURNING id, course_id, student_id`,
		courseID, tutorID, studentIDs,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	enrollments := []models.CourseEnrollment{}
	for rows.Next() {
		var e models.CourseEnrollment
		if err := rows.Scan(&e.ID, &e.CourseID, &e.StudentID); err != nil {
			return nil, err
		}
		enrollments = append(enrollments, e)
	}
	return enrollments, rows.Err()
}

// Remove — мягкий уход: строка остаётся с датой ухода. Она кусок истории
// участия, и фазе 2 нужна для разметки легаси-платежей группы (спека, п. 5a.4).
func (r *enrollmentRepository) Remove(ctx context.Context, courseID string, studentID string) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE course_enrollments SET left_at = NOW()
		 WHERE course_id = $1 AND student_id = $2 AND left_at IS NULL`,
		courseID, studentID)
	return err
}

// LeaveAllByStudent закрывает все текущие записи ученика — шаг архивации
// (спека, п. 5a.3). Уже закрытые не трогает: прежняя дата ухода — история.
// Владение учеником проверяет вызывающий сервис.
func (r *enrollmentRepository) LeaveAllByStudent(ctx context.Context, studentID string) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE course_enrollments SET left_at = NOW()
		 WHERE student_id = $1 AND left_at IS NULL`, studentID)
	return err
}

// IsEnrolled — была ли у ученика запись в группу, в том числе закрытая уходом.
// Без left_at IS NULL намеренно: ушедшему с долгом платёж обязан приниматься,
// иначе долг невзыскиваемый (спека, п. 6.3).
func (r *enrollmentRepository) IsEnrolled(ctx context.Context, courseID, studentID string) (bool, error) {
	var ok bool
	err := r.pool.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM course_enrollments WHERE course_id = $1 AND student_id = $2)`,
		courseID, studentID).Scan(&ok)
	return ok, err
}

func (r *enrollmentRepository) GetByCourse(ctx context.Context, courseID string) ([]models.CourseEnrollment, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT ce.id, ce.course_id, ce.student_id, s.first_name, s.last_name
		 FROM course_enrollments ce
		 JOIN students s ON s.id = ce.student_id
		 WHERE ce.course_id = $1 AND ce.left_at IS NULL`, courseID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var enrollments []models.CourseEnrollment
	for rows.Next() {
		var e models.CourseEnrollment
		if err := rows.Scan(&e.ID, &e.CourseID, &e.StudentID, &e.StudentFirstName, &e.StudentLastName); err != nil {
			return nil, err
		}
		enrollments = append(enrollments, e)
	}
	return enrollments, rows.Err()
}
