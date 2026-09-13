package repository

import (
	"context"
	"errors"
	"time"
	"tutorgo/models"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type CourseRepository interface {
	Create(ctx context.Context, req models.CreateCourseRequest, tutorID string) (models.Course, error)
	GetOrCreateIndividual(ctx context.Context, tutorID string, studentID string, subject string, startedAt time.Time) (models.Course, error)
	GetSubjects(ctx context.Context, tutorID string) ([]string, error)
	GetAll(ctx context.Context, tutorID string, p models.Pagination) ([]models.Course, int, error)
	GetByID(ctx context.Context, id string, tutorID string) (models.Course, error)
	GetByStudent(ctx context.Context, studentID string, tutorID string) ([]models.Course, error)
	Update(ctx context.Context, id string, tutorID string, req models.UpdateCourseRequest) (models.Course, error)
	Delete(ctx context.Context, id string, tutorID string) error
	GetAllArchived(ctx context.Context, tutorID string, p models.Pagination) ([]models.Course, int, error)
	Restore(ctx context.Context, id string, tutorID string) error
	GetHomework(ctx context.Context, id string, tutorID string) (string, error)
	SetHomework(ctx context.Context, id string, tutorID string, homework string) (int64, error)
}

type courseRepository struct {
	conn *pgxpool.Pool
}

func NewCourseRepository(conn *pgxpool.Pool) CourseRepository {
	return &courseRepository{conn: conn}
}

func (r *courseRepository) Create(ctx context.Context, req models.CreateCourseRequest, tutorID string) (models.Course, error) {
	var course models.Course
	err := r.conn.QueryRow(ctx,
		`INSERT INTO courses (student_id, tutor_id, subject, price_per_cycle, lessons_per_cycle, started_at, ended_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)
		 RETURNING id, student_id, tutor_id, subject, price_per_cycle, lessons_per_cycle, started_at, ended_at, is_active`,
		req.StudentID, tutorID, req.Subject, req.PricePerCycle, req.LessonsPerCycle, req.StartedAt, req.EndedAt,
	).Scan(&course.ID, &course.StudentID, &course.TutorID, &course.Subject, &course.PricePerCycle, &course.LessonsPerCycle, &course.StartedAt, &course.EndedAt, &course.IsActive)
	return course, err
}

const courseCols = `id, student_id, tutor_id, subject, price_per_cycle, lessons_per_cycle, started_at, ended_at, is_active`

func scanCourse(row pgx.Row) (models.Course, error) {
	var c models.Course
	err := row.Scan(&c.ID, &c.StudentID, &c.TutorID, &c.Subject, &c.PricePerCycle, &c.LessonsPerCycle, &c.StartedAt, &c.EndedAt, &c.IsActive)
	return c, err
}

// GetOrCreateIndividual возвращает активный индивидуальный курс тьютора по паре
// «ученик + предмет», создавая его при отсутствии. Курс — производная сущность:
// пользователь ставит урок, а не заводит курс.
//
// Транзакции здесь нет намеренно: под READ COMMITTED она от гонки не спасает —
// два параллельных запроса оба увидят пустой SELECT. Спасает уникальный индекс
// из миграции 032 плюс ON CONFLICT DO NOTHING, поэтому проигравший гонку просто
// читает чужой курс.
//
// Вставка идёт SELECT'ом из students — это заодно и проверка владения: чужой
// ученик даёт ноль строк, а не чужой курс.
func (r *courseRepository) GetOrCreateIndividual(ctx context.Context, tutorID string, studentID string, subject string, startedAt time.Time) (models.Course, error) {
	const find = `SELECT ` + courseCols + `
	              FROM courses
	              WHERE tutor_id = $1::uuid AND student_id = $2::uuid AND subject = $3::text AND is_active`

	course, err := scanCourse(r.conn.QueryRow(ctx, find, tutorID, studentID, subject))
	if err == nil || !errors.Is(err, pgx.ErrNoRows) {
		return course, err
	}

	// Дефолт — самый частый пакет тьютора: спрашивать прайс посреди постановки
	// урока незачем. Сумма и число уроков берутся ПАРОЙ из одной группы: цена
	// бывает пакетом, который на уроки не делится (85 000 за 12), и независимые
	// «самая частая сумма» + «самое частое число» склеивали сумму одного пакета с
	// количеством другого. Ничья — самый поздний курс, то есть текущий прайс, а
	// не минимальная цена: у тьютора со всеми уникальными пакетами прежний
	// tie-break выбирал 1 ₸ за 12 уроков.
	// Правило зеркалит выбор дефолта в StudentOnboardingDialog — менять вместе.
	const create = `INSERT INTO courses (student_id, tutor_id, subject, price_per_cycle, lessons_per_cycle, started_at)
	                SELECT s.id, $1::uuid, $3::text,
	                       COALESCE(d.price_per_cycle, 0), COALESCE(d.lessons_per_cycle, 1),
	                       $4::timestamptz
	                FROM students s
	                LEFT JOIN LATERAL (
	                    SELECT price_per_cycle, lessons_per_cycle
	                      FROM courses
	                     WHERE tutor_id = $1::uuid AND is_active
	                     GROUP BY price_per_cycle, lessons_per_cycle
	                     ORDER BY count(*) DESC, max(started_at) DESC, price_per_cycle, lessons_per_cycle
	                     LIMIT 1
	                ) d ON TRUE
	                WHERE s.id = $2::uuid AND s.tutor_id = $1::uuid
	                ON CONFLICT DO NOTHING
	                RETURNING ` + courseCols

	course, err = scanCourse(r.conn.QueryRow(ctx, create, tutorID, studentID, subject, startedAt))
	if err == nil || !errors.Is(err, pgx.ErrNoRows) {
		return course, err
	}

	// Ноль строк — либо ученик чужой, либо гонку выиграл параллельный запрос.
	// Повторный SELECT различает: нашёлся курс — гонка, пусто — чужой ученик.
	return scanCourse(r.conn.QueryRow(ctx, find, tutorID, studentID, subject))
}

// GetSubjects — предметы тьютора для комбобокса: свободный ввод плодил
// «Математику» и «математику» как разные курсы.
func (r *courseRepository) GetSubjects(ctx context.Context, tutorID string) ([]string, error) {
	rows, err := r.conn.Query(ctx,
		`SELECT DISTINCT subject FROM courses WHERE tutor_id = $1 AND is_active ORDER BY subject`, tutorID)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowTo[string])
}

func (r *courseRepository) GetAll(ctx context.Context, tutorID string, p models.Pagination) ([]models.Course, int, error) {
	var total int
	if err := r.conn.QueryRow(ctx,
		`SELECT COUNT(*) FROM courses
		 WHERE tutor_id = $1 AND is_active = TRUE
		   AND ($2 = '' OR subject ILIKE '%' || $2 || '%')`,
		tutorID, p.Search,
	).Scan(&total); err != nil {
		return nil, 0, err
	}

	rows, err := r.conn.Query(ctx,
		`SELECT id, student_id, tutor_id, subject, price_per_cycle, lessons_per_cycle, started_at, ended_at, is_active
		 FROM courses
		 WHERE tutor_id = $1 AND is_active = TRUE
		   AND ($2 = '' OR subject ILIKE '%' || $2 || '%')
		 ORDER BY started_at DESC
		 LIMIT $3 OFFSET $4`,
		tutorID, p.Search, p.Limit, p.Offset())
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	courses := []models.Course{}
	for rows.Next() {
		var course models.Course
		if err := rows.Scan(&course.ID, &course.StudentID, &course.TutorID, &course.Subject, &course.PricePerCycle, &course.LessonsPerCycle, &course.StartedAt, &course.EndedAt, &course.IsActive); err != nil {
			return nil, 0, err
		}
		courses = append(courses, course)
	}
	return courses, total, rows.Err()
}

func (r *courseRepository) GetByID(ctx context.Context, id string, tutorID string) (models.Course, error) {
	var course models.Course
	err := r.conn.QueryRow(ctx,
		`SELECT id, student_id, tutor_id, subject, price_per_cycle, lessons_per_cycle, started_at, ended_at, is_active
		 FROM courses WHERE id = $1 AND tutor_id = $2`, id, tutorID,
	).Scan(&course.ID, &course.StudentID, &course.TutorID, &course.Subject, &course.PricePerCycle, &course.LessonsPerCycle, &course.StartedAt, &course.EndedAt, &course.IsActive)
	return course, err
}

func (r *courseRepository) GetByStudent(ctx context.Context, studentID string, tutorID string) ([]models.Course, error) {
	rows, err := r.conn.Query(ctx,
		`SELECT c.id, c.student_id, c.tutor_id, c.subject, c.price_per_cycle, c.lessons_per_cycle, c.started_at, c.ended_at, c.is_active
		 FROM courses c
		 WHERE c.tutor_id = $2 AND c.student_id = $1 AND c.is_active = TRUE
		 UNION
		 SELECT c.id, c.student_id, c.tutor_id, c.subject, c.price_per_cycle, c.lessons_per_cycle, c.started_at, c.ended_at, c.is_active
		 FROM courses c
		 JOIN course_enrollments ce ON ce.course_id = c.id
		 WHERE c.tutor_id = $2 AND ce.student_id = $1 AND c.is_active = TRUE
		 ORDER BY started_at DESC`,
		studentID, tutorID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var courses []models.Course
	for rows.Next() {
		var course models.Course
		if err := rows.Scan(&course.ID, &course.StudentID, &course.TutorID, &course.Subject, &course.PricePerCycle, &course.LessonsPerCycle, &course.StartedAt, &course.EndedAt, &course.IsActive); err != nil {
			return nil, err
		}
		courses = append(courses, course)
	}
	return courses, rows.Err()
}

func (r *courseRepository) Update(ctx context.Context, id string, tutorID string, req models.UpdateCourseRequest) (models.Course, error) {
	var course models.Course
	err := r.conn.QueryRow(ctx,
		`UPDATE courses SET subject=$1, price_per_cycle=$2, lessons_per_cycle=$3, started_at=$4, ended_at=$5
		 WHERE id=$6 AND tutor_id=$7
		 RETURNING id, student_id, tutor_id, subject, price_per_cycle, lessons_per_cycle, started_at, ended_at, is_active`,
		req.Subject, req.PricePerCycle, req.LessonsPerCycle, req.StartedAt, req.EndedAt, id, tutorID,
	).Scan(&course.ID, &course.StudentID, &course.TutorID, &course.Subject, &course.PricePerCycle, &course.LessonsPerCycle, &course.StartedAt, &course.EndedAt, &course.IsActive)
	return course, err
}

func (r *courseRepository) GetHomework(ctx context.Context, id string, tutorID string) (string, error) {
	var hw string
	err := r.conn.QueryRow(ctx,
		`SELECT homework FROM courses WHERE id=$1 AND tutor_id=$2`, id, tutorID,
	).Scan(&hw)
	return hw, err
}

func (r *courseRepository) SetHomework(ctx context.Context, id string, tutorID string, homework string) (int64, error) {
	tag, err := r.conn.Exec(ctx,
		`UPDATE courses SET homework=$1 WHERE id=$2 AND tutor_id=$3`, homework, id, tutorID,
	)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

func (r *courseRepository) Delete(ctx context.Context, id string, tutorID string) error {
	_, err := r.conn.Exec(ctx,
		`UPDATE courses SET is_active = FALSE WHERE id = $1 AND tutor_id = $2`, id, tutorID)
	return err
}

func (r *courseRepository) GetAllArchived(ctx context.Context, tutorID string, p models.Pagination) ([]models.Course, int, error) {
	var total int
	if err := r.conn.QueryRow(ctx,
		`SELECT COUNT(*) FROM courses
		 WHERE tutor_id = $1 AND is_active = FALSE
		   AND ($2 = '' OR subject ILIKE '%' || $2 || '%')`,
		tutorID, p.Search,
	).Scan(&total); err != nil {
		return nil, 0, err
	}

	rows, err := r.conn.Query(ctx,
		`SELECT id, student_id, tutor_id, subject, price_per_cycle, lessons_per_cycle, started_at, ended_at, is_active
		 FROM courses
		 WHERE tutor_id = $1 AND is_active = FALSE
		   AND ($2 = '' OR subject ILIKE '%' || $2 || '%')
		 ORDER BY started_at DESC
		 LIMIT $3 OFFSET $4`,
		tutorID, p.Search, p.Limit, p.Offset())
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	courses := []models.Course{}
	for rows.Next() {
		var course models.Course
		if err := rows.Scan(&course.ID, &course.StudentID, &course.TutorID, &course.Subject, &course.PricePerCycle, &course.LessonsPerCycle, &course.StartedAt, &course.EndedAt, &course.IsActive); err != nil {
			return nil, 0, err
		}
		courses = append(courses, course)
	}
	return courses, total, rows.Err()
}

func (r *courseRepository) Restore(ctx context.Context, id string, tutorID string) error {
	_, err := r.conn.Exec(ctx,
		`UPDATE courses SET is_active = TRUE WHERE id = $1 AND tutor_id = $2`, id, tutorID)
	return err
}
