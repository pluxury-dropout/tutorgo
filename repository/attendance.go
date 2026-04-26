package repository

import (
	"context"
	"tutorgo/models"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type AttendanceRepository interface {
	Upsert(ctx context.Context, lessonID string, entries []models.AttendanceEntry) error
	GetByLesson(ctx context.Context, lessonID string) ([]models.LessonAttendance, error)
}

type attendanceRepository struct {
	pool *pgxpool.Pool
}

func NewAttendanceRepository(pool *pgxpool.Pool) AttendanceRepository {
	return &attendanceRepository{pool: pool}
}

func (r *attendanceRepository) Upsert(ctx context.Context, lessonID string, entries []models.AttendanceEntry) error {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	batch := &pgx.Batch{}
	for _, e := range entries {
		batch.Queue(
			`INSERT INTO lesson_attendances (lesson_id, student_id, status)
			 VALUES ($1, $2, $3)
			 ON CONFLICT (lesson_id, student_id) DO UPDATE SET status = $3`,
			lessonID, e.StudentID, e.Status,
		)
	}
	br := tx.SendBatch(ctx, batch)
	for range entries {
		if _, err := br.Exec(); err != nil {
			br.Close()
			return err
		}
	}
	if err := br.Close(); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r *attendanceRepository) GetByLesson(ctx context.Context, lessonID string) ([]models.LessonAttendance, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, lesson_id, student_id, status
		 FROM lesson_attendances WHERE lesson_id = $1`, lessonID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var attendances []models.LessonAttendance
	for rows.Next() {
		var a models.LessonAttendance
		if err := rows.Scan(&a.ID, &a.LessonID, &a.StudentID, &a.Status); err != nil {
			return nil, err
		}
		attendances = append(attendances, a)
	}
	return attendances, rows.Err()
}
