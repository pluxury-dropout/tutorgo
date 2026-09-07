package repository

import (
	"context"
	"errors"
	"time"
	"tutorgo/models"

	"github.com/jackc/pgx/v5/pgxpool"
)

type LessonRepository interface {
	Create(ctx context.Context, req models.CreateLessonRequest) (models.Lesson, error)
	GetByCourse(ctx context.Context, courseID string) ([]models.Lesson, error)
	GetByCoursePaged(ctx context.Context, courseID string, p models.Pagination) ([]models.Lesson, int, error)
	GetByID(ctx context.Context, id string) (models.Lesson, error)
	GetByIDForTutor(ctx context.Context, id string, tutorID string) (models.Lesson, error)
	Update(ctx context.Context, id string, req models.UpdateLessonRequest) (models.Lesson, error)
	Delete(ctx context.Context, id string) error
	Cancel(ctx context.Context, id string) error
	MarkOverride(ctx context.Context, id string) error
	DeleteByCourse(ctx context.Context, courseID string, tutorID string) error
	GetCalendar(ctx context.Context, tutorID string, from string, to string) ([]models.CalendarLesson, error)
	GetAllLessonsForCycles(ctx context.Context, tutorID string) ([]models.CalendarLesson, error)
	GetByPeriod(ctx context.Context, courseID string, tutorID string, from string, to string) ([]models.Lesson, error)
	AutoComplete(ctx context.Context) (int64, error)
	StartRoom(ctx context.Context, lessonID string, tutorID string) error
	EndRoom(ctx context.Context, lessonID string, tutorID string) error
	EndRoomByID(ctx context.Context, lessonID string) error
	GetRoomStatus(ctx context.Context, lessonID string) (string, error)
	GetRanksForCourses(ctx context.Context, courseIDs []string) (map[string]map[string]int, error)
	GetRuleIDsByCourse(ctx context.Context, courseID string) ([]string, error)
	DeleteFutureByCourse(ctx context.Context, courseID, tutorID string) error
}

type lessonRepository struct {
	pool *pgxpool.Pool
}

func NewLessonRepository(pool *pgxpool.Pool) LessonRepository {
	return &lessonRepository{pool: pool}
}

// Одна константа на все выборки урока: россыпь SQL — ровно та причина, по
// которой rule_id и occurrence_date забыли в четырёх местах, а у событий
// (см. eventColumns) не забыли ни в одном. Алиас `l` обязателен и там, где
// джойна нет, иначе константу не переиспользовать.
const lessonColumns = `l.id, l.course_id, l.scheduled_at, l.duration_minutes,
                       l.status, l.notes, l.rule_id, l.occurrence_date`

func scanLesson(row interface{ Scan(...any) error }) (models.Lesson, error) {
	var l models.Lesson
	err := row.Scan(&l.ID, &l.CourseID, &l.ScheduledAt, &l.DurationMinutes,
		&l.Status, &l.Notes, &l.RuleID, &l.OccurrenceDate)
	return l, err
}

func (r *lessonRepository) Create(ctx context.Context, req models.CreateLessonRequest) (models.Lesson, error) {
	var lesson models.Lesson
	err := r.pool.QueryRow(ctx,
		`INSERT INTO lessons (course_id, scheduled_at, duration_minutes, notes, status, rule_id, occurrence_date)
		 VALUES ($1, $2, $3, $4,
		         CASE WHEN $2::timestamptz + $3::integer * interval '1 minute' < NOW() THEN 'completed' ELSE 'scheduled' END,
		         NULLIF($5, '')::uuid, $6::date)
		 RETURNING id, course_id, scheduled_at, duration_minutes, status, notes`,
		req.CourseID, req.ScheduledAt, req.DurationMinutes, req.Notes, req.RuleID, req.OccurrenceDate,
	).Scan(&lesson.ID, &lesson.CourseID, &lesson.ScheduledAt, &lesson.DurationMinutes, &lesson.Status, &lesson.Notes)
	return lesson, err
}

func (r *lessonRepository) GetByCourse(ctx context.Context, courseID string) ([]models.Lesson, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT `+lessonColumns+` FROM lessons l
		 WHERE l.course_id = $1 ORDER BY l.scheduled_at`, courseID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	lessons := []models.Lesson{}
	for rows.Next() {
		lesson, err := scanLesson(rows)
		if err != nil {
			return nil, err
		}
		lessons = append(lessons, lesson)
	}
	return lessons, rows.Err()
}

func (r *lessonRepository) GetByCoursePaged(ctx context.Context, courseID string, p models.Pagination) ([]models.Lesson, int, error) {
	var total int
	if err := r.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM lessons WHERE course_id = $1`, courseID,
	).Scan(&total); err != nil {
		return nil, 0, err
	}

	rows, err := r.pool.Query(ctx,
		`SELECT `+lessonColumns+` FROM lessons l WHERE l.course_id = $1
		 ORDER BY l.scheduled_at DESC
		 LIMIT $2 OFFSET $3`,
		courseID, p.Limit, p.Offset())
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	lessons := []models.Lesson{}
	for rows.Next() {
		l, err := scanLesson(rows)
		if err != nil {
			return nil, 0, err
		}
		lessons = append(lessons, l)
	}
	return lessons, total, rows.Err()
}

func (r *lessonRepository) GetByID(ctx context.Context, id string) (models.Lesson, error) {
	return scanLesson(r.pool.QueryRow(ctx,
		`SELECT `+lessonColumns+` FROM lessons l WHERE l.id = $1`, id))
}

// Cancel вместо Delete для вхождения серии: удали строку целиком — и ночная
// материализация создаст урок заново, потому что дата снова свободна.
// is_override заодно защищает отмену от «изменить все следующие».
func (r *lessonRepository) Cancel(ctx context.Context, id string) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE lessons SET status = 'cancelled', is_override = TRUE WHERE id = $1`, id)
	return err
}

// MarkOverride помечает вхождение вручную правленным. Ставится при правке
// «только это»: без метки ближайшее «это и все следующие» снесёт строку
// (DeleteFutureByRule смотрит ровно на is_override), а материализация вернёт
// урок на место по расписанию правила — перенос молча пропадёт.
func (r *lessonRepository) MarkOverride(ctx context.Context, id string) error {
	_, err := r.pool.Exec(ctx, `UPDATE lessons SET is_override = TRUE WHERE id = $1`, id)
	return err
}

func (r *lessonRepository) GetByIDForTutor(ctx context.Context, id string, tutorID string) (models.Lesson, error) {
	return scanLesson(r.pool.QueryRow(ctx,
		`SELECT `+lessonColumns+` FROM lessons l
		 JOIN courses c ON c.id = l.course_id
		 WHERE l.id = $1 AND c.tutor_id = $2`, id, tutorID))
}

func (r *lessonRepository) Update(ctx context.Context, id string, req models.UpdateLessonRequest) (models.Lesson, error) {
	var lesson models.Lesson
	err := r.pool.QueryRow(ctx,
		`UPDATE lessons SET scheduled_at=$1, duration_minutes=$2, status=$3, notes=$4
		 WHERE id=$5
		 RETURNING id, course_id, scheduled_at, duration_minutes, status, notes`,
		req.ScheduledAt, req.DurationMinutes, req.Status, req.Notes, id,
	).Scan(&lesson.ID, &lesson.CourseID, &lesson.ScheduledAt, &lesson.DurationMinutes, &lesson.Status, &lesson.Notes)
	return lesson, err
}

func (r *lessonRepository) Delete(ctx context.Context, id string) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM lessons WHERE id = $1`, id)
	return err
}

func (r *lessonRepository) DeleteByCourse(ctx context.Context, courseID string, tutorID string) error {
	_, err := r.pool.Exec(ctx,
		`DELETE FROM lessons
		 USING courses
		 WHERE lessons.course_id = $1
		   AND lessons.course_id = courses.id
		   AND courses.tutor_id = $2`,
		courseID, tutorID)
	return err
}

func (r *lessonRepository) GetCalendar(ctx context.Context, tutorID string, from string, to string) ([]models.CalendarLesson, error) {
	rows, err := r.pool.Query(ctx,
		// ranked CTE computes global lesson rank inside the DB — eliminates a separate GetRanksForCourses round-trip.
		// cal_courses is materialized so the subquery runs once and is reused by ranked.
		`WITH cal_courses AS MATERIALIZED (
		   SELECT DISTINCT l.course_id
		   FROM lessons l
		   JOIN courses c ON c.id = l.course_id
		   WHERE c.tutor_id = $1
		     AND l.scheduled_at >= $2::timestamptz
		     AND l.scheduled_at < $3::timestamptz
		 ),
		 ranked AS (
		   SELECT l.id,
		          ROW_NUMBER() OVER (PARTITION BY l.course_id ORDER BY l.scheduled_at)::int AS rank
		   FROM lessons l
		   WHERE l.course_id IN (SELECT course_id FROM cal_courses)
		     AND l.status != 'cancelled'
		 )
		 SELECT l.id, l.course_id, l.scheduled_at, l.duration_minutes, l.status, l.notes,
		        c.subject,
		        CASE WHEN c.student_id IS NOT NULL
		             THEN CASE WHEN s.last_name = '' THEN s.first_name ELSE s.first_name || ' ' || s.last_name END
		             ELSE NULL
		        END AS student_name,
		        (c.student_id IS NULL) AS is_group,
		        l.rule_id,
		        r.rank
		 FROM lessons l
		 JOIN courses c ON c.id = l.course_id
		 LEFT JOIN students s ON s.id = c.student_id
		 LEFT JOIN ranked r ON r.id = l.id
		 WHERE c.tutor_id = $1
		   AND l.scheduled_at >= $2::timestamptz
		   AND l.scheduled_at < $3::timestamptz
		 ORDER BY l.scheduled_at`,
		tutorID, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var lessons []models.CalendarLesson
	for rows.Next() {
		var cl models.CalendarLesson
		if err := rows.Scan(&cl.ID, &cl.CourseID, &cl.ScheduledAt, &cl.DurationMinutes,
			&cl.Status, &cl.Notes, &cl.Subject, &cl.StudentName, &cl.IsGroup, &cl.RuleID, &cl.Rank); err != nil {
			return nil, err
		}
		lessons = append(lessons, cl)
	}
	return lessons, rows.Err()
}

func (r *lessonRepository) GetByPeriod(ctx context.Context, courseID string, tutorID string, from string, to string) ([]models.Lesson, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT `+lessonColumns+`
		 FROM lessons l
		 JOIN courses c ON c.id = l.course_id
		 WHERE l.course_id = $1
		   AND c.tutor_id = $2
		   AND l.scheduled_at >= $3::timestamptz
		   AND l.scheduled_at < $4::timestamptz
		 ORDER BY l.scheduled_at ASC`,
		courseID, tutorID, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	lessons := []models.Lesson{}
	for rows.Next() {
		l, err := scanLesson(rows)
		if err != nil {
			return nil, err
		}
		lessons = append(lessons, l)
	}
	return lessons, rows.Err()
}

func (r *lessonRepository) AutoComplete(ctx context.Context) (int64, error) {
	result, err := r.pool.Exec(ctx,
		`UPDATE lessons SET status = 'completed'
		 WHERE status = 'scheduled'
		   AND scheduled_at + duration_minutes * interval '1 minute' < NOW()`)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected(), nil
}

func (r *lessonRepository) StartRoom(ctx context.Context, lessonID string, tutorID string) error {
	result, err := r.pool.Exec(ctx,
		`UPDATE lessons
		SET room_started_at = NOW(), room_ended_at = NULL
		FROM courses
		WHERE lessons.id = $1
		  AND lessons.course_id = courses.id
		  AND courses.tutor_id = $2`,
		lessonID, tutorID)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return errors.New("lesson not found")
	}
	return nil
}

func (r *lessonRepository) EndRoom(ctx context.Context, lessonID string, tutorID string) error {
	result, err := r.pool.Exec(ctx,
		`UPDATE lessons
		SET room_ended_at = NOW()
		FROM courses
		WHERE lessons.id = $1
		  AND lessons.course_id = courses.id
		  AND courses.tutor_id = $2`,
		lessonID, tutorID)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return errors.New("lesson not found")
	}
	return nil
}

// EndRoomByID marks a room ended without a tutor scope — used by the LiveKit
// webhook, which is authenticated by signature, not by a logged-in tutor.
// Idempotent: only the first call (room started, not yet ended) writes; a
// re-delivered webhook or an already-ended room is a no-op, not an error.
func (r *lessonRepository) EndRoomByID(ctx context.Context, lessonID string) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE lessons
		SET room_ended_at = NOW()
		WHERE id = $1
		  AND room_started_at IS NOT NULL
		  AND room_ended_at IS NULL`,
		lessonID)
	return err
}

func (r *lessonRepository) GetRoomStatus(ctx context.Context, lessonID string) (string, error) {
	var startedAt, endedAt *time.Time
	err := r.pool.QueryRow(ctx,
		`SELECT room_started_at, room_ended_at FROM lessons WHERE id = $1`, lessonID).Scan(&startedAt, &endedAt)
	if err != nil {
		return "", err
	}
	if endedAt != nil {
		return "ended", nil
	}
	if startedAt != nil {
		return "active", nil
	}
	return "waiting", nil
}

func (r *lessonRepository) GetAllLessonsForCycles(ctx context.Context, tutorID string) ([]models.CalendarLesson, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT l.id, l.course_id, l.scheduled_at, l.status,
		        c.subject,
		        CASE WHEN c.student_id IS NOT NULL
		             THEN CASE WHEN s.last_name = '' THEN s.first_name ELSE s.first_name || ' ' || s.last_name END
		             ELSE NULL
		        END AS student_name,
		        (c.student_id IS NULL) AS is_group,
		        ROW_NUMBER() OVER (PARTITION BY l.course_id ORDER BY l.scheduled_at)::int AS rank
		 FROM lessons l
		 JOIN courses c ON c.id = l.course_id
		 LEFT JOIN students s ON s.id = c.student_id
		 WHERE c.tutor_id = $1
		   AND l.status != 'cancelled'
		 ORDER BY l.course_id, l.scheduled_at`,
		tutorID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var lessons []models.CalendarLesson
	for rows.Next() {
		var cl models.CalendarLesson
		if err := rows.Scan(
			&cl.ID, &cl.CourseID, &cl.ScheduledAt, &cl.Status,
			&cl.Subject, &cl.StudentName, &cl.IsGroup, &cl.Rank,
		); err != nil {
			return nil, err
		}
		lessons = append(lessons, cl)
	}
	return lessons, rows.Err()
}

// GetRuleIDsByCourse — единственный путь от курса к его правилам: прямой связи
// в схеме нет, только через lessons.rule_id. Поэтому читать надо ДО удаления
// уроков, иначе связь потеряна безвозвратно.
func (r *lessonRepository) GetRuleIDsByCourse(ctx context.Context, courseID string) ([]string, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT DISTINCT rule_id::text FROM lessons
		 WHERE course_id = $1 AND rule_id IS NOT NULL`, courseID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	ids := []string{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// DeleteFutureByCourse убирает то, что ещё не состоялось. Завершённые,
// отменённые и пропущенные остаются: диалог архивации обещает именно это.
func (r *lessonRepository) DeleteFutureByCourse(ctx context.Context, courseID, tutorID string) error {
	_, err := r.pool.Exec(ctx,
		`DELETE FROM lessons
		 USING courses
		 WHERE lessons.course_id = $1
		   AND lessons.course_id = courses.id
		   AND courses.tutor_id = $2
		   AND lessons.status = 'scheduled'
		   AND lessons.scheduled_at > NOW()`,
		courseID, tutorID)
	return err
}

func (r *lessonRepository) GetRanksForCourses(ctx context.Context, courseIDs []string) (map[string]map[string]int, error) {
	if len(courseIDs) == 0 {
		return map[string]map[string]int{}, nil
	}
	rows, err := r.pool.Query(ctx,
		`SELECT id, course_id,
		        ROW_NUMBER() OVER (PARTITION BY course_id ORDER BY scheduled_at)::int AS rank
		 FROM lessons
		 WHERE course_id = ANY($1)
		   AND status != 'cancelled'`,
		courseIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := map[string]map[string]int{}
	for rows.Next() {
		var lessonID, courseID string
		var rank int
		if err := rows.Scan(&lessonID, &courseID, &rank); err != nil {
			return nil, err
		}
		if result[courseID] == nil {
			result[courseID] = map[string]int{}
		}
		result[courseID][lessonID] = rank
	}
	return result, rows.Err()
}
