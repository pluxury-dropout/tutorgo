package repository

import (
	"context"
	"tutorgo/models"

	"github.com/jackc/pgx/v5/pgxpool"
)

// LessonTaskRepository — доступ к задачам урока (lesson_tasks). Все authz-условия
// вшиты в SQL: tutor через JOIN lessons→courses.tutor_id, student — по lesson_id
// (enrollment уже проверен в сервисе через StudentRepository.EnrolledInLesson).
type LessonTaskRepository interface {
	// tutor: список задач урока, если урок принадлежит tutorID
	ListByLessonForTutor(ctx context.Context, lessonID, tutorID string) ([]models.LessonTask, error)
	// tutor: создать задачу, если урок принадлежит tutorID; иначе pgx.ErrNoRows (0 строк)
	Create(ctx context.Context, lessonID, tutorID string, req models.CreateLessonTaskRequest) (models.LessonTask, error)
	// tutor: обновить текст, если задача на уроке tutorID
	Update(ctx context.Context, taskID, tutorID string, req models.UpdateLessonTaskRequest) (models.LessonTask, error)
	// tutor: удалить, если задача на уроке tutorID; вернуть кол-во удалённых
	Delete(ctx context.Context, taskID, tutorID string) (int64, error)
	// student: список задач урока (enrollment проверяется в сервисе)
	ListByLessonForStudent(ctx context.Context, lessonID string) ([]models.LessonTask, error)
	// student: сменить done; вернуть кол-во обновлённых (0 если задача не найдена)
	SetDone(ctx context.Context, taskID string, done bool) (int64, error)
	// student: lesson_id задачи (для проверки enrollment перед SetDone)
	LessonIDByTask(ctx context.Context, taskID string) (string, error)
}

type lessonTaskRepository struct {
	conn *pgxpool.Pool
}

func NewLessonTaskRepository(conn *pgxpool.Pool) LessonTaskRepository {
	return &lessonTaskRepository{conn: conn}
}

func (r *lessonTaskRepository) ListByLessonForTutor(ctx context.Context, lessonID, tutorID string) ([]models.LessonTask, error) {
	rows, err := r.conn.Query(ctx,
		`SELECT t.id, t.lesson_id, t.title, t.description, t.done, t.created_at
		   FROM lesson_tasks t JOIN lessons l ON l.id = t.lesson_id
		                       JOIN courses c ON c.id = l.course_id
		  WHERE t.lesson_id = $1 AND c.tutor_id = $2
		  ORDER BY t.created_at`, lessonID, tutorID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	tasks := []models.LessonTask{}
	for rows.Next() {
		var t models.LessonTask
		if err := rows.Scan(&t.ID, &t.LessonID, &t.Title, &t.Description, &t.Done, &t.CreatedAt); err != nil {
			return nil, err
		}
		tasks = append(tasks, t)
	}
	return tasks, rows.Err()
}

func (r *lessonTaskRepository) Create(ctx context.Context, lessonID, tutorID string, req models.CreateLessonTaskRequest) (models.LessonTask, error) {
	var t models.LessonTask
	err := r.conn.QueryRow(ctx,
		`INSERT INTO lesson_tasks (lesson_id, title, description)
		 SELECT l.id, $3, $4
		   FROM lessons l JOIN courses c ON c.id = l.course_id
		  WHERE l.id = $1 AND c.tutor_id = $2
		 RETURNING id, lesson_id, title, description, done, created_at`,
		lessonID, tutorID, req.Title, req.Description,
	).Scan(&t.ID, &t.LessonID, &t.Title, &t.Description, &t.Done, &t.CreatedAt)
	return t, err
}

func (r *lessonTaskRepository) Update(ctx context.Context, taskID, tutorID string, req models.UpdateLessonTaskRequest) (models.LessonTask, error) {
	var t models.LessonTask
	err := r.conn.QueryRow(ctx,
		`UPDATE lesson_tasks t SET title=$3, description=$4
		   FROM lessons l, courses c
		  WHERE t.id=$1 AND l.id=t.lesson_id AND c.id=l.course_id AND c.tutor_id=$2
		 RETURNING t.id, t.lesson_id, t.title, t.description, t.done, t.created_at`,
		taskID, tutorID, req.Title, req.Description,
	).Scan(&t.ID, &t.LessonID, &t.Title, &t.Description, &t.Done, &t.CreatedAt)
	return t, err
}

func (r *lessonTaskRepository) Delete(ctx context.Context, taskID, tutorID string) (int64, error) {
	tag, err := r.conn.Exec(ctx,
		`DELETE FROM lesson_tasks t USING lessons l, courses c
		  WHERE t.id=$1 AND l.id=t.lesson_id AND c.id=l.course_id AND c.tutor_id=$2`,
		taskID, tutorID)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

func (r *lessonTaskRepository) ListByLessonForStudent(ctx context.Context, lessonID string) ([]models.LessonTask, error) {
	rows, err := r.conn.Query(ctx,
		`SELECT id, lesson_id, title, description, done, created_at
		   FROM lesson_tasks WHERE lesson_id=$1 ORDER BY created_at`, lessonID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	tasks := []models.LessonTask{}
	for rows.Next() {
		var t models.LessonTask
		if err := rows.Scan(&t.ID, &t.LessonID, &t.Title, &t.Description, &t.Done, &t.CreatedAt); err != nil {
			return nil, err
		}
		tasks = append(tasks, t)
	}
	return tasks, rows.Err()
}

func (r *lessonTaskRepository) SetDone(ctx context.Context, taskID string, done bool) (int64, error) {
	tag, err := r.conn.Exec(ctx,
		`UPDATE lesson_tasks SET done=$2 WHERE id=$1`, taskID, done)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

func (r *lessonTaskRepository) LessonIDByTask(ctx context.Context, taskID string) (string, error) {
	var lessonID string
	err := r.conn.QueryRow(ctx,
		`SELECT lesson_id FROM lesson_tasks WHERE id=$1`, taskID,
	).Scan(&lessonID)
	return lessonID, err
}
