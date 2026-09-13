package repository

import (
	"context"
	"time"
	"tutorgo/models"

	"github.com/jackc/pgx/v5/pgxpool"
)

type StudentRepository interface {
	Create(ctx context.Context, req models.CreateStudentRequest, tutorID string) (models.Student, error)
	GetAll(ctx context.Context, tutorID string, p models.Pagination) ([]models.Student, int, error)
	GetByID(ctx context.Context, id string, tutorID string) (models.Student, error)
	Update(ctx context.Context, id string, tutorID string, req models.UpdateStudentRequest) (models.Student, error)
	Delete(ctx context.Context, id string, tutorID string) error
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

type studentRepository struct {
	conn *pgxpool.Pool
}

func NewStudentRepository(conn *pgxpool.Pool) StudentRepository {
	return &studentRepository{conn: conn}
}

func (r *studentRepository) Create(ctx context.Context, req models.CreateStudentRequest, tutorID string) (models.Student, error) {
	var student models.Student
	err := r.conn.QueryRow(ctx,
		`INSERT INTO students (tutor_id, first_name, last_name, phone, email, notes)
		 VALUES ($1, $2, $3, $4, $5, $6)
		 RETURNING id, tutor_id, first_name, last_name, phone, email, notes, active`,
		tutorID, req.FirstName, req.LastName, req.Phone, req.Email, req.Notes,
	).Scan(&student.ID, &student.TutorID, &student.FirstName, &student.LastName, &student.Phone, &student.Email, &student.Notes, &student.Active)
	return student, err
}

func (r *studentRepository) GetAll(ctx context.Context, tutorID string, p models.Pagination) ([]models.Student, int, error) {
	var total int
	if err := r.conn.QueryRow(ctx,
		`SELECT COUNT(*) FROM students
		 WHERE tutor_id = $1
		   AND ($2 = '' OR first_name ILIKE '%' || $2 || '%'
		                 OR last_name  ILIKE '%' || $2 || '%'
		                 OR email      ILIKE '%' || $2 || '%')`,
		tutorID, p.Search,
	).Scan(&total); err != nil {
		return nil, 0, err
	}

	rows, err := r.conn.Query(ctx,
		`SELECT id, tutor_id, first_name, last_name, phone, email, notes, active
		 FROM students
		 WHERE tutor_id = $1
		   AND ($2 = '' OR first_name ILIKE '%' || $2 || '%'
		                 OR last_name  ILIKE '%' || $2 || '%'
		                 OR email      ILIKE '%' || $2 || '%')
		 ORDER BY first_name, last_name
		 LIMIT $3 OFFSET $4`,
		tutorID, p.Search, p.Limit, p.Offset())
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	students := []models.Student{}
	for rows.Next() {
		var student models.Student
		if err := rows.Scan(&student.ID, &student.TutorID, &student.FirstName, &student.LastName, &student.Phone, &student.Email, &student.Notes, &student.Active); err != nil {
			return nil, 0, err
		}
		students = append(students, student)
	}
	return students, total, rows.Err()
}

func (r *studentRepository) GetByID(ctx context.Context, id string, tutorID string) (models.Student, error) {
	var student models.Student
	err := r.conn.QueryRow(ctx,
		`SELECT id, tutor_id, first_name, last_name, phone, email, notes, active
		 FROM students WHERE id = $1 AND tutor_id = $2`, id, tutorID,
	).Scan(&student.ID, &student.TutorID, &student.FirstName, &student.LastName, &student.Phone, &student.Email, &student.Notes, &student.Active)
	return student, err
}

func (r *studentRepository) Update(ctx context.Context, id string, tutorID string, req models.UpdateStudentRequest) (models.Student, error) {
	var student models.Student
	err := r.conn.QueryRow(ctx,
		`UPDATE students SET first_name=$1, last_name=$2, phone=$3, email=$4, notes=$5
		 WHERE id=$6 AND tutor_id=$7
		 RETURNING id, tutor_id, first_name, last_name, phone, email, notes, active`,
		req.FirstName, req.LastName, req.Phone, req.Email, req.Notes, id, tutorID,
	).Scan(&student.ID, &student.TutorID, &student.FirstName, &student.LastName, &student.Phone, &student.Email, &student.Notes, &student.Active)
	return student, err
}

func (r *studentRepository) Delete(ctx context.Context, id string, tutorID string) error {
	_, err := r.conn.Exec(ctx,
		`DELETE FROM students WHERE id = $1 AND tutor_id = $2`, id, tutorID)
	return err
}

func (r *studentRepository) SetInvite(ctx context.Context, studentID, token string, expiresAt time.Time) error {
	_, err := r.conn.Exec(ctx,
		`UPDATE students SET invite_token=$2, invite_expires_at=$3 WHERE id=$1`,
		studentID, token, expiresAt)
	return err
}

func (r *studentRepository) GetByInviteToken(ctx context.Context, token string) (string, time.Time, error) {
	var id string
	var exp time.Time
	err := r.conn.QueryRow(ctx,
		`SELECT id, invite_expires_at FROM students WHERE invite_token=$1`, token,
	).Scan(&id, &exp)
	return id, exp, err
}

func (r *studentRepository) ActivateAccount(ctx context.Context, studentID, username, passwordHash string) error {
	_, err := r.conn.Exec(ctx,
		`UPDATE students SET username=$2, password_hash=$3, invite_token=NULL, invite_expires_at=NULL WHERE id=$1`,
		studentID, username, passwordHash)
	return err
}

func (r *studentRepository) GetCredentialsByLogin(ctx context.Context, identifier string) (string, string, error) {
	var id, hash string
	err := r.conn.QueryRow(ctx,
		`SELECT id, password_hash FROM students
		 WHERE password_hash IS NOT NULL AND (username=$1 OR phone=$1) LIMIT 1`, identifier,
	).Scan(&id, &hash)
	return id, hash, err
}

func (r *studentRepository) EnrolledInLesson(ctx context.Context, studentID, lessonID string) (bool, error) {
	var ok bool
	err := r.conn.QueryRow(ctx,
		`SELECT EXISTS (
		   SELECT 1 FROM lessons l JOIN courses c ON c.id=l.course_id
		   WHERE l.id=$2 AND (
		     c.student_id=$1
		     OR EXISTS (SELECT 1 FROM course_enrollments ce
		                WHERE ce.course_id=c.id AND ce.student_id=$1 AND ce.left_at IS NULL)
		   ))`, studentID, lessonID,
	).Scan(&ok)
	return ok, err
}

func (r *studentRepository) CourseAndTutorForLesson(ctx context.Context, lessonID string) (string, string, error) {
	var courseID, tutorID string
	err := r.conn.QueryRow(ctx,
		`SELECT c.id, c.tutor_id FROM lessons l JOIN courses c ON c.id=l.course_id WHERE l.id=$1`, lessonID,
	).Scan(&courseID, &tutorID)
	return courseID, tutorID, err
}

func (r *studentRepository) GetProfile(ctx context.Context, studentID string) (models.StudentProfile, error) {
	var p models.StudentProfile
	err := r.conn.QueryRow(ctx,
		`SELECT id, first_name, last_name, phone, COALESCE(username, '')
		 FROM students WHERE id = $1`, studentID,
	).Scan(&p.ID, &p.FirstName, &p.LastName, &p.Phone, &p.Username)
	return p, err
}

func (r *studentRepository) ListHomework(ctx context.Context, studentID string) ([]models.StudentHomework, error) {
	// enrollment-джойн идентичен ListLessons: индивидуальный курс ИЛИ групповой
	// через course_enrollments. Только курсы с непустым ДЗ.
	rows, err := r.conn.Query(ctx,
		`SELECT c.id, c.subject, c.homework FROM courses c
		 WHERE (c.student_id = $1
		        OR EXISTS (SELECT 1 FROM course_enrollments ce
		                   WHERE ce.course_id = c.id AND ce.student_id = $1))
		   AND c.homework <> ''
		 ORDER BY c.subject`, studentID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.StudentHomework
	for rows.Next() {
		var h models.StudentHomework
		if err := rows.Scan(&h.CourseID, &h.Subject, &h.Homework); err != nil {
			return nil, err
		}
		out = append(out, h)
	}
	return out, rows.Err()
}

func (r *studentRepository) ListCourses(ctx context.Context, studentID string) ([]models.StudentCourse, error) {
	// enrollment-джойн идентичен ListHomework: индивидуальный курс ИЛИ групповой
	// через course_enrollments. Без фильтра по homework.
	rows, err := r.conn.Query(ctx,
		`SELECT c.id, c.subject, c.tutor_id FROM courses c
		 WHERE (c.student_id = $1
		        OR EXISTS (SELECT 1 FROM course_enrollments ce
		                   WHERE ce.course_id = c.id AND ce.student_id = $1))
		 ORDER BY c.subject`, studentID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.StudentCourse
	for rows.Next() {
		var sc models.StudentCourse
		if err := rows.Scan(&sc.ID, &sc.Subject, &sc.TutorID); err != nil {
			return nil, err
		}
		out = append(out, sc)
	}
	return out, rows.Err()
}

func (r *studentRepository) ListLessons(ctx context.Context, studentID string, past bool) ([]models.CalendarLesson, error) {
	// enrollment-джойн идентичен EnrolledInLesson: индивидуальный курс (c.student_id)
	// ИЛИ групповой через course_enrollments. Фильтр активности курса намеренно
	// опущен — ученик видит все свои уроки, включая архивные (история).
	// rank считается по ВСЕМ неотменённым урокам курса (без date-фильтра),
	// иначе позиция в цикле зависела бы от выбранной вкладки.
	// Уроки группы после ухода (left_at) не показываются — та же граница, что
	// в burned фазы 2; прошлые остаются историей (спека, п. 5a.4).
	base := `WITH stu_courses AS MATERIALIZED (
	           SELECT c.id FROM courses c
	           WHERE c.student_id = $1
	              OR EXISTS (SELECT 1 FROM course_enrollments ce
	                         WHERE ce.course_id = c.id AND ce.student_id = $1)
	         ),
	         ranked AS (
	           SELECT l.id,
	                  ROW_NUMBER() OVER (PARTITION BY l.course_id ORDER BY l.scheduled_at)::int AS rank
	           FROM lessons l
	           WHERE l.course_id IN (SELECT id FROM stu_courses)
	             AND l.status != 'cancelled'
	         )
	         SELECT l.id, l.course_id, l.scheduled_at, l.duration_minutes, l.status,
	                l.notes, c.subject, s.first_name, (c.student_id IS NULL) AS is_group,
	                r.rank
	         FROM lessons l
	         JOIN courses c ON c.id = l.course_id
	         LEFT JOIN students s ON s.id = c.student_id
	         LEFT JOIN ranked r ON r.id = l.id
	         LEFT JOIN course_enrollments ce ON ce.course_id = l.course_id AND ce.student_id = $1
	         WHERE l.course_id IN (SELECT id FROM stu_courses)
	           AND l.scheduled_at < COALESCE(ce.left_at, 'infinity'::timestamptz)`
	var q string
	if past {
		q = base + ` AND l.scheduled_at + l.duration_minutes * interval '1 minute' < now() ORDER BY l.scheduled_at DESC`
	} else {
		q = base + ` AND l.scheduled_at + l.duration_minutes * interval '1 minute' >= now() ORDER BY l.scheduled_at ASC`
	}
	rows, err := r.conn.Query(ctx, q, studentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	lessons := []models.CalendarLesson{}
	for rows.Next() {
		var l models.CalendarLesson
		if err := rows.Scan(&l.ID, &l.CourseID, &l.ScheduledAt, &l.DurationMinutes,
			&l.Status, &l.Notes, &l.Subject, &l.StudentName, &l.IsGroup, &l.Rank); err != nil {
			return nil, err
		}
		lessons = append(lessons, l)
	}
	return lessons, rows.Err()
}

func (r *studentRepository) GetPasswordHash(ctx context.Context, studentID string) (string, error) {
	var hash string
	err := r.conn.QueryRow(ctx,
		`SELECT COALESCE(password_hash, '') FROM students WHERE id = $1`, studentID,
	).Scan(&hash)
	return hash, err
}

func (r *studentRepository) UpdatePassword(ctx context.Context, studentID, hash string) error {
	_, err := r.conn.Exec(ctx,
		`UPDATE students SET password_hash = $2 WHERE id = $1`, studentID, hash)
	return err
}
