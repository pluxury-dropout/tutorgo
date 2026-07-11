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
		     OR EXISTS (SELECT 1 FROM course_enrollments ce WHERE ce.course_id=c.id AND ce.student_id=$1)
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
		`SELECT first_name, last_name, phone, COALESCE(username, '')
		 FROM students WHERE id = $1`, studentID,
	).Scan(&p.FirstName, &p.LastName, &p.Phone, &p.Username)
	return p, err
}
