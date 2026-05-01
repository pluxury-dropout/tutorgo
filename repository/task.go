package repository

import (
	"context"
	"tutorgo/models"

	"github.com/jackc/pgx/v5/pgxpool"
)

type TaskRepository interface {
	Create(ctx context.Context, tutorID string, req models.CreateTaskRequest) (models.Task, error)
	GetByRange(ctx context.Context, tutorID, from, to string) ([]models.Task, error)
	Update(ctx context.Context, id, tutorID string, req models.UpdateTaskRequest) (models.Task, error)
	Delete(ctx context.Context, id, tutorID string) error
	ToggleDone(ctx context.Context, id, tutorID string) (models.Task, error)
}

type taskRepository struct {
	conn *pgxpool.Pool
}

func NewTaskRepository(conn *pgxpool.Pool) TaskRepository {
	return &taskRepository{conn: conn}
}

func (r *taskRepository) Create(ctx context.Context, tutorID string, req models.CreateTaskRequest) (models.Task, error) {
	var t models.Task
	err := r.conn.QueryRow(ctx,
		`INSERT INTO tasks (tutor_id, title, scheduled_at, duration_minutes)
		 VALUES ($1, $2, $3, $4)
		 RETURNING id, tutor_id, title, scheduled_at, duration_minutes, done, created_at`,
		tutorID, req.Title, req.ScheduledAt, req.DurationMinutes,
	).Scan(&t.ID, &t.TutorID, &t.Title, &t.ScheduledAt, &t.DurationMinutes, &t.Done, &t.CreatedAt)
	return t, err
}

func (r *taskRepository) GetByRange(ctx context.Context, tutorID, from, to string) ([]models.Task, error) {
	rows, err := r.conn.Query(ctx,
		`SELECT id, tutor_id, title, scheduled_at, duration_minutes, done, created_at
		 FROM tasks
		 WHERE tutor_id = $1 AND scheduled_at >= $2 AND scheduled_at < $3
		 ORDER BY scheduled_at`,
		tutorID, from, to,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var tasks []models.Task
	for rows.Next() {
		var t models.Task
		if err := rows.Scan(&t.ID, &t.TutorID, &t.Title, &t.ScheduledAt, &t.DurationMinutes, &t.Done, &t.CreatedAt); err != nil {
			return nil, err
		}
		tasks = append(tasks, t)
	}
	return tasks, rows.Err()
}

func (r *taskRepository) Update(ctx context.Context, id, tutorID string, req models.UpdateTaskRequest) (models.Task, error) {
	var t models.Task
	err := r.conn.QueryRow(ctx,
		`UPDATE tasks SET title=$1, scheduled_at=$2, duration_minutes=$3, done=$4
		 WHERE id=$5 AND tutor_id=$6
		 RETURNING id, tutor_id, title, scheduled_at, duration_minutes, done, created_at`,
		req.Title, req.ScheduledAt, req.DurationMinutes, req.Done, id, tutorID,
	).Scan(&t.ID, &t.TutorID, &t.Title, &t.ScheduledAt, &t.DurationMinutes, &t.Done, &t.CreatedAt)
	return t, err
}

func (r *taskRepository) Delete(ctx context.Context, id, tutorID string) error {
	_, err := r.conn.Exec(ctx,
		`DELETE FROM tasks WHERE id=$1 AND tutor_id=$2`, id, tutorID,
	)
	return err
}

func (r *taskRepository) ToggleDone(ctx context.Context, id, tutorID string) (models.Task, error) {
	var t models.Task
	err := r.conn.QueryRow(ctx,
		`UPDATE tasks SET done = NOT done
		 WHERE id=$1 AND tutor_id=$2
		 RETURNING id, tutor_id, title, scheduled_at, duration_minutes, done, created_at`,
		id, tutorID,
	).Scan(&t.ID, &t.TutorID, &t.Title, &t.ScheduledAt, &t.DurationMinutes, &t.Done, &t.CreatedAt)
	return t, err
}
