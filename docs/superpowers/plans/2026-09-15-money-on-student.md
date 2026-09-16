# Фаза 2 — деньги на ученике: план реализации

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** платёж знает, кто заплатил; баланс, долг, прогноз и позиция в цикле считаются по человеку с учётом периодов участия (запись, уход, заморозка); появляются экран «Долги», оплата на несколько предметов и заморозка ученика.

**Architecture:** миграция 039 — `payments.student_id`, `course_enrollments.enrolled_at`, `student_pauses`. Все денежные запросы строятся от одной выборки пар «курс + ученик» (`studentCoursePairs`) и одного предиката «урок принадлежит ученику» (`lessonInParticipation`) в `repository/payment.go`. Заморозка — один SQL-оператор с data-modifying CTE (пауза + отмена уроков + сдвиг хвоста правил), затем материализация хвоста.

**Tech Stack:** Go + Gin + pgx/v5, goose-миграции, testify/mock, интеграционные тесты с build-тегом `integration`; фронт — Next.js App Router + React Query + react-hook-form/zod + base-ui/shadcn.

**Spec:** `docs/specs/2026-09-06-price-units-and-student-centric-money.md`, раздел **6** (фаза 2), а также п. 3.4–3.9, 5a.1, 5a.5.

**Прод-данные, сверенные 2026-09-15 (read-only):** групповых платежей 5 из 61 (разметка руками, эвристика не нужна); записей в группы 8 — дата по посещаемости найдётся у 6, по `started_at` у 2, ушедших 0; правил повторения с `ends_on` 32, с `max_count` 3, бессрочных 5; goose-версия прода 38.

## Global Constraints

- Правки файлов — **только через Edit/Write**, не через `sed`/`python`/heredoc в Bash (CLAUDE.md).
- **Никогда не запускать `make migrate-up` / `make migrate-down` / `make migrate-status`**: они бьют в прод. Тестовая БД — `TEST_DB_URL="postgres://dev:dev@localhost:5432/tutorgo_dev?sslmode=disable"`, миграции на неё — `make migrate-test` или `goose -dir migrations postgres "$TEST_DB_URL" <cmd>`.
- Интеграционный прогон: `TEST_DB_URL="postgres://dev:dev@localhost:5432/tutorgo_dev?sslmode=disable" go test -tags=integration -count=1 ./repository/ -run '<regex>' -v`.
- Известное красное, не связанное с фазой: `TestGetAllByTutor_CarriesSubjectAndStudentName`. Любое другое падение — стоп.
- Ветка `feat/money-on-student`. Не пушить, не мержить.
- **Параллельная работа в одном дереве:** фронт и бэкенд идут одновременно. Коммитить только свои файлы явными путями: `git add path1 path2`. Запрещено: `git add -A`, `git add .`, `git commit -a`, `git stash`, `git checkout`, `git reset`, `git restore`. Чужие незакоммиченные файлы в `git status` — не трогать. Если `git commit` упал на `index.lock` — подождать 5 секунд и повторить.
- Каждый коммит заканчивается строками:
  ```
  Co-Authored-By: Claude Opus 5 <noreply@anthropic.com>
  Claude-Session: https://claude.ai/code/session_01KLiSrLKjmzZMQgDzgsUdbn
  ```
- Комментарии в коде — по-русски, в стиле соседнего кода: объясняют «почему», ссылаются на спеку как `(спека, п. 6.N)`.
- Главный footgun проекта: расширил интерфейс репозитория или сервиса — обнови мок в `service/*_test.go` / `handlers/mocks_test.go` в том же коммите. Сверка: `go vet ./...` и `go vet -tags=integration ./repository/`.
- Интеграционные тесты `repository/*_integration_test.go` — один пакет `repository_test`. Уже существующие хелперы переиспользовать, не дублировать: `testPool`, `seedCourse`, `addLessons`, `addPayment`, `seedTutorStudent` (ученик «Иван Петров»), `addPriced`, `createFromCalendar`, `seedSeries`, `seedEventSeries`, `nextWeekday`, `addGroupCourse` (цена 5000 за 1 урок), `addLessonAt`, `leftAt`, `addIndividualCourse` («Математика», 40000 за 8), `studentExists`. Перед добавлением нового — `grep -n '^func ' repository/*_integration_test.go`.
- **Денежные правила (спека, п. 3.6):** сгорает урок со `status IN ('completed', 'missed')`; `cancelled` не сгорает никогда; `lesson_attendances` в денежных запросах не участвует вообще. Цена урока = `ROUND(price_per_cycle / lessons_per_cycle)` — до тенге.
- **Периоды участия (спека, п. 3.9):** урок принадлежит ученику, если `scheduled_at >= enrolled_at`, `scheduled_at < left_at` и дата урока не попала в его паузу. У индивидуального курса границ записи нет, паузы действуют.
- Фронт: тест-раннера нет; проверка — `cd frontend && npm run lint`. Ошибки axios — `ApiError { message, status }`; сбой мутации ловить на месте и показывать `toast.error`. Версия Next.js новее тренировочных данных — сверяться с `node_modules/next/dist/docs/`. Весь текст интерфейса — по-русски.

## Контракт API фазы (фронт опирается только на него)

| Метод и путь | Тело / query | Ответ |
|---|---|---|
| `POST /payments` | `{course_id, student_id, amount, lessons_count, paid_at}` — `student_id` обязателен | `201 Payment`; ученик не на курсе → `400` |
| `PUT /payments/:id` | `{student_id, amount, lessons_count, paid_at}` | `200 Payment`; ученик не на курсе → `400` |
| `GET /payments?course_id=` | — | `Payment` получает `student_id` и `student_name` (null у платежа без адресата) |
| `GET /payments/balance` | `?course_id=&student_id=` — **оба обязательны** | `CourseBalance {lessons_paid, lessons_completed, lessons_remaining}` |
| `GET /payments/debts` | — | `StudentDebt[]` по убыванию `amount_owed` |
| `POST /payments/bulk` | `{paid_at, items: [{course_id, student_id, amount, lessons_count}]}`, 1–20 строк | `201 Payment[]`; ошибка в любой строке → ничего не записано |
| `GET /students/:id/pauses` | — | `StudentPause[]` по убыванию `starts_on` |
| `POST /students/:id/pauses` | `{starts_on, ends_on, reason?}` (даты `YYYY-MM-DDT00:00:00Z`) | `201 StudentPause`; `ends_on < starts_on` → `400` |
| `DELETE /students/:id/pauses/:pauseId` | — | `204` |

```ts
Payment      { id, course_id, student_id: string | null, amount, lessons_count, paid_at, subject?, student_name?: string | null }
StudentDebt  { student_id, student_name, lessons_owed, amount_owed, next_lesson_at: string | null, courses: DebtByCourse[] }
DebtByCourse { course_id, subject, lessons_owed, amount_owed }
StudentPause { id, student_id, starts_on, ends_on, reason: string | null, created_at }
```

## Карта файлов

| Файл | Задачи |
|---|---|
| `migrations/039_payments_student.sql` | 1 |
| `models/payment.go` | 1, 3, 5 |
| `models/pause.go` (новый) | 6 |
| `repository/payment.go` | 1, 2, 3, 4, 5 |
| `repository/enrollment.go`, `repository/student.go` (`Delete`) | 1 |
| `repository/lesson.go`, `repository/student.go` (`ListLessons`) | 4 |
| `repository/pause.go` (новый) | 6 |
| `service/payment.go` | 1, 2, 3, 5 |
| `service/lesson.go`, `service/student.go` | 4 |
| `service/pause.go` (новый) | 6 |
| `handlers/payment.go` | 2, 3, 5 |
| `handlers/pause.go` (новый) | 6 |
| `router/router.go` | 1, 3, 5, 6 |
| `frontend/src/{types/api.ts, lib/api/{payments,courses}.ts, lib/hooks/{usePayments,useCourses}.ts, schemas/payment.ts}` | 7 |
| `frontend/src/components/payments/{PaymentForm,DebtsList}.tsx`, `app/(dashboard)/{payments,courses/[id]}/page.tsx` | 7 |
| `frontend/src/lib/{api/students.ts, hooks/useStudents.ts}`, `components/payments/BulkPaymentDialog.tsx`, `components/students/PauseDialog.tsx`, `app/(dashboard)/students/[id]/page.tsx` | 8 |

Порядок: **Task 1** первым (фундамент). Затем параллельно две полосы: бэкенд **2 → 3 → 4 → 5 → 6** и фронт **7 → 8**. **Task 9** — последним.

---

### Task 1: Миграция 039, адресный платёж, проверка связи ученика с курсом

**Files:**
- Create: `migrations/039_payments_student.sql`
- Modify: `models/payment.go`
- Modify: `repository/payment.go` (колонки платежа, `Create`, `GetByCourse`, `GetByCoursesBatch`, `GetPaymentsForCalendar`, `GetAllByTutor`, `GetAllByTutorPaged`, `Update`, новый `GetByID`, `studentNameExpr`)
- Modify: `repository/enrollment.go` (новый `IsEnrolled`)
- Modify: `repository/student.go` (`Delete` — критерий истории)
- Modify: `service/payment.go` (конструктор, `studentOnCourse`, `Create`, `Update`)
- Modify: `router/router.go:54`
- Modify: `service/payment_test.go`, `service/enrollment_test.go`, `handlers/payment_test.go`
- Modify: `repository/payment_integration_test.go` (хелпер `addPayment`)
- Create: `repository/payment_student_integration_test.go`

**Interfaces:**
- Produces:
  - `models.Payment.StudentID *string`; `CreatePaymentRequest.StudentID string`; `UpdatePaymentRequest.StudentID string`
  - `repository.paymentColumns` (const) и `paymentDest(p *models.Payment) []any` — для всех выборок платежа
  - `PaymentRepository.GetByID(ctx context.Context, id string, tutorID string) (models.Payment, error)`
  - `EnrollmentRepository.IsEnrolled(ctx context.Context, courseID, studentID string) (bool, error)`
  - `service.NewPaymentService(repo repository.PaymentRepository, courseRepo repository.CourseRepository, enrollmentRepo repository.EnrollmentRepository) PaymentService`
  - `(*paymentService).studentOnCourse(ctx context.Context, course models.Course, studentID string) error` — `ErrBadRequest`, если ученик не на курсе
  - integration-хелперы: `addStudent(t *testing.T, pool *pgxpool.Pool, tutorID, firstName string) string`, `enroll(t *testing.T, pool *pgxpool.Pool, courseID, studentID, atExpr string)`, `payFor(t *testing.T, pool *pgxpool.Pool, courseID, studentID string, lessons int)`
  - service-тест: `newPaymentSvc(payRepo *mockPaymentRepo, courseRepo *mockCourseRepo, enrollRepo *mockEnrollmentRepo) service.PaymentService`, переменная `payStudentID`

- [ ] **Step 1: Засеять на тестовой БД сценарий бэкфилла (схема ещё 038)**

Проверить: `goose -dir migrations postgres "$TEST_DB_URL" status | tail -1` показывает `038_enrollments_left_at.sql` последней применённой. Создать файл `/tmp/claude-1000/seed-039.sql` (вне репозитория, не коммитить):

```sql
BEGIN;
INSERT INTO tutors (id, email, password_hash, first_name, last_name)
VALUES ('00000000-0000-0000-0000-00000000a039', 'mig039@example.com', 'x', 'M', 'T');
INSERT INTO students (id, tutor_id, first_name) VALUES
  ('00000000-0000-0000-0000-00000000b001', '00000000-0000-0000-0000-00000000a039', 'Посещал'),
  ('00000000-0000-0000-0000-00000000b002', '00000000-0000-0000-0000-00000000a039', 'Без отметок');
INSERT INTO courses (id, tutor_id, subject, price_per_cycle, lessons_per_cycle, started_at) VALUES
  ('00000000-0000-0000-0000-00000000c001', '00000000-0000-0000-0000-00000000a039', 'Группа', 5000, 4, '2026-09-01 00:00+00');
INSERT INTO courses (id, student_id, tutor_id, subject, price_per_cycle, lessons_per_cycle, started_at) VALUES
  ('00000000-0000-0000-0000-00000000c002', '00000000-0000-0000-0000-00000000b001', '00000000-0000-0000-0000-00000000a039', 'Индивидуально', 40000, 8, '2026-09-01 00:00+00');
INSERT INTO course_enrollments (course_id, student_id) VALUES
  ('00000000-0000-0000-0000-00000000c001', '00000000-0000-0000-0000-00000000b001'),
  ('00000000-0000-0000-0000-00000000c001', '00000000-0000-0000-0000-00000000b002');
INSERT INTO lessons (id, course_id, scheduled_at, duration_minutes, status) VALUES
  ('00000000-0000-0000-0000-00000000d001', '00000000-0000-0000-0000-00000000c001', '2026-09-10 12:00+00', 60, 'completed'),
  ('00000000-0000-0000-0000-00000000d002', '00000000-0000-0000-0000-00000000c001', '2026-11-05 12:00+00', 60, 'completed');
-- «Посещал» впервые отмечен в ноябре — дата записи обязана стать ноябрьской.
INSERT INTO lesson_attendances (lesson_id, student_id, status) VALUES
  ('00000000-0000-0000-0000-00000000d002', '00000000-0000-0000-0000-00000000b001', 'present');
INSERT INTO payments (course_id, amount, lessons_count, paid_at) VALUES
  ('00000000-0000-0000-0000-00000000c001', 20000, 4, '2026-09-10'),
  ('00000000-0000-0000-0000-00000000c002', 40000, 8, '2026-09-02');
COMMIT;
```

Run: `psql "postgres://dev:dev@localhost:5432/tutorgo_dev?sslmode=disable" -v ON_ERROR_STOP=1 -f /tmp/claude-1000/seed-039.sql`
Expected: `COMMIT`.

- [ ] **Step 2: Создать миграцию**

Create `migrations/039_payments_student.sql`:

```sql
-- +goose Up
-- Платёж адресный: долг и баланс — свойства человека, а не курса. Групповой
-- курс (student_id IS NULL) вообще не даёт понять, кто из пяти заплатил.
ALTER TABLE payments ADD COLUMN student_id UUID REFERENCES students(id) ON DELETE CASCADE;

-- Индивидуальные курсы бэкфиллятся однозначно: у курса ровно один ученик.
UPDATE payments p
   SET student_id = c.student_id
  FROM courses c
 WHERE c.id = p.course_id AND c.student_id IS NOT NULL;

-- Групповые платежи адресата не имеют и восстановить его неоткуда: остаются
-- NULL и в баланс конкретного ученика не входят (спека, п. 3.4).

CREATE INDEX idx_payments_student ON payments(student_id) WHERE student_id IS NOT NULL;

-- Периоды участия (спека, п. 3.9): с какого числа уроки ученика сгорают; по
-- какое — left_at, заведённый миграцией 038 (фаза 1.5). Без enrolled_at ученик,
-- добавленный в ноябре, задолжает за все занятия с сентября: сгорает каждый
-- состоявшийся урок курса, а строка не помнит, когда участник появился.
ALTER TABLE course_enrollments ADD COLUMN enrolled_at TIMESTAMPTZ;

-- Строки course_enrollments бывают только у групповых курсов, а групповым
-- платежам student_id здесь не выдаётся (см. выше) — значит искать «первый
-- платёж этого ученика» бессмысленно, подзапрос вернул бы NULL для всех строк.
-- Единственный след участия, который есть у групп сейчас, — посещаемость;
-- started_at остаётся для тех, кого ни разу не отмечали (спека, п. 3.7).
UPDATE course_enrollments ce
   SET enrolled_at = COALESCE(
         (SELECT min(l.scheduled_at)
            FROM lesson_attendances la
            JOIN lessons l ON l.id = la.lesson_id
           WHERE l.course_id = ce.course_id AND la.student_id = ce.student_id),
         c.started_at)
  FROM courses c
 WHERE c.id = ce.course_id;

ALTER TABLE course_enrollments
  ALTER COLUMN enrolled_at SET NOT NULL,
  ALTER COLUMN enrolled_at SET DEFAULT NOW();

-- Заморозка (спека, п. 3.9 и 6.9): интервал висит на ученике, без course_id —
-- «уехал на месяц» закрывает все его предметы и группы одним действием.
CREATE TABLE student_pauses (
    id         UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    student_id UUID        NOT NULL REFERENCES students(id) ON DELETE CASCADE,
    starts_on  DATE        NOT NULL,
    ends_on    DATE        NOT NULL,
    reason     TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK (ends_on >= starts_on)
);

CREATE INDEX idx_pauses_student ON student_pauses(student_id, starts_on);

-- +goose Down
DROP TABLE IF EXISTS student_pauses;
ALTER TABLE course_enrollments DROP COLUMN enrolled_at;
DROP INDEX IF EXISTS idx_payments_student;
ALTER TABLE payments DROP COLUMN student_id;
```

- [ ] **Step 3: Накатить, проверить бэкфилл, откатить, накатить снова**

Run:
```bash
export TEST_DB_URL="postgres://dev:dev@localhost:5432/tutorgo_dev?sslmode=disable"
goose -dir migrations postgres "$TEST_DB_URL" up
psql "$TEST_DB_URL" -At -c "SELECT c.subject, p.student_id FROM payments p JOIN courses c ON c.id=p.course_id WHERE c.tutor_id='00000000-0000-0000-0000-00000000a039' ORDER BY c.subject"
psql "$TEST_DB_URL" -At -c "SELECT s.first_name, ce.enrolled_at FROM course_enrollments ce JOIN students s ON s.id=ce.student_id WHERE ce.course_id='00000000-0000-0000-0000-00000000c001' ORDER BY s.first_name"
goose -dir migrations postgres "$TEST_DB_URL" down
psql "$TEST_DB_URL" -At -c "SELECT count(*) FROM information_schema.columns WHERE table_name='payments' AND column_name='student_id'"
goose -dir migrations postgres "$TEST_DB_URL" up
psql "$TEST_DB_URL" -c "DELETE FROM tutors WHERE id='00000000-0000-0000-0000-00000000a039'"
```
Expected, по порядку: `Группа|` (пусто — легаси) и `Индивидуально|00000000-0000-0000-0000-00000000b001`; `Без отметок|2026-09-01 …` (started_at) и `Посещал|2026-11-05 …` (первая отметка); после `down` — `0`; второй `up` проходит; `DELETE 1`. Вывод всех команд — в отчёт.

- [ ] **Step 4: Модель**

Replace the whole `models/payment.go` with:

```go
package models

import "time"

type Payment struct {
	ID       string `json:"id"`
	CourseID string `json:"course_id"`
	// NULL — легаси-платёж группы, заведённый до миграции 039: восстановить
	// адресата неоткуда, в баланс конкретного ученика он не входит (спека, п. 3.4).
	StudentID    *string   `json:"student_id"`
	Amount       float64   `json:"amount"`
	LessonsCount int       `json:"lessons_count"`
	PaidAt       time.Time `json:"paid_at"`

	// Subject заполняют списки по репетитору (GetAllByTutor, GetAllByTutorPaged),
	// где платёж показывают вне контекста курса. StudentName — имя адресата в
	// списках и в истории курса; пуст у платежа без адресата.
	Subject     string  `json:"subject,omitempty"`
	StudentName *string `json:"student_name,omitempty"`
}

type CreatePaymentRequest struct {
	CourseID string `json:"course_id"     validate:"required,uuid"`
	// Обязателен в API, хотя колонка nullable: NULL — только легаси (спека, п. 3.4).
	StudentID    string    `json:"student_id"    validate:"required,uuid"`
	Amount       float64   `json:"amount"        validate:"required,gt=0"`
	LessonsCount int       `json:"lessons_count" validate:"required,gt=0"`
	PaidAt       time.Time `json:"paid_at"       validate:"required"`
}

// UpdatePaymentRequest несёт адресата: иначе легаси-платёж группы нечем
// починить из интерфейса (спека, п. 6.2).
type UpdatePaymentRequest struct {
	StudentID    string    `json:"student_id"    validate:"required,uuid"`
	Amount       float64   `json:"amount"        validate:"required,gt=0"`
	LessonsCount int       `json:"lessons_count" validate:"required,gt=0"`
	PaidAt       time.Time `json:"paid_at"       validate:"required"`
}
```

- [ ] **Step 5: Написать падающие интеграционные тесты**

In `repository/payment_integration_test.go` replace `addPayment` (keep the signature) with:

```go
// addPayment — платёж по курсу; адресат берётся из курса, как в бэкфилле
// миграции 039: у индивидуального — его ученик, у группы — NULL (легаси).
func addPayment(t *testing.T, pool *pgxpool.Pool, courseID string, amount float64, lessonsCount int, paidAtExpr string) {
	_, err := pool.Exec(context.Background(),
		fmt.Sprintf(`INSERT INTO payments (course_id, student_id, amount, lessons_count, paid_at)
		             SELECT c.id, c.student_id, $2, $3, %s FROM courses c WHERE c.id = $1`, paidAtExpr),
		courseID, amount, lessonsCount)
	require.NoError(t, err)
}
```

Create `repository/payment_student_integration_test.go`:

```go
//go:build integration

package repository_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"tutorgo/models"
	"tutorgo/repository"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Платёж адресный (спека 2026-09-06-price-units…, п. 6.1–6.3). Запуск:
// make test-integration.

// addStudent — ещё один ученик репетитора.
func addStudent(t *testing.T, pool *pgxpool.Pool, tutorID, firstName string) string {
	var id string
	require.NoError(t, pool.QueryRow(context.Background(),
		`INSERT INTO students (tutor_id, first_name) VALUES ($1, $2) RETURNING id`,
		tutorID, firstName).Scan(&id))
	return id
}

// enroll — запись в группу с явной датой записи; atExpr — SQL-выражение.
func enroll(t *testing.T, pool *pgxpool.Pool, courseID, studentID, atExpr string) {
	_, err := pool.Exec(context.Background(),
		fmt.Sprintf(`INSERT INTO course_enrollments (course_id, student_id, enrolled_at)
		             VALUES ($1, $2, %s)`, atExpr),
		courseID, studentID)
	require.NoError(t, err)
}

// payFor — адресный платёж ученика по курсу за lessons уроков.
func payFor(t *testing.T, pool *pgxpool.Pool, courseID, studentID string, lessons int) {
	_, err := pool.Exec(context.Background(),
		`INSERT INTO payments (course_id, student_id, amount, lessons_count, paid_at)
		 VALUES ($1, $2, $3, $4, NOW())`,
		courseID, studentID, float64(lessons*1000), lessons)
	require.NoError(t, err)
}

// Ушедшему из группы платёж обязан приниматься — иначе долг невзыскиваемый;
// ученику, которого в группе не было, — нет (спека, п. 6.3).
func TestIsEnrolled_TrueForLeftStudentFalseForStranger(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	tutorID, studentID := seedTutorStudent(t, pool)
	stranger := addStudent(t, pool, tutorID, "Чужой")
	groupID := addGroupCourse(t, pool, tutorID, "Группа")
	enroll(t, pool, groupID, studentID, "NOW() - interval '1 month'")

	repo := repository.NewEnrollmentRepository(pool)
	require.NoError(t, repo.Remove(ctx, groupID, studentID))

	ok, err := repo.IsEnrolled(ctx, groupID, studentID)
	require.NoError(t, err)
	assert.True(t, ok)

	ok, err = repo.IsEnrolled(ctx, groupID, stranger)
	require.NoError(t, err)
	assert.False(t, ok)
}

// Участник группы, заплативший, но ни разу не отмеченный, — ученик с историей:
// иначе каскад по payments.student_id унёс бы его платёж (спека, п. 6.3).
func TestStudentDelete_AddressedGroupPaymentBlocksDeletion(t *testing.T) {
	pool := testPool(t)
	tutorID, studentID := seedTutorStudent(t, pool)
	groupID := addGroupCourse(t, pool, tutorID, "Группа")
	enroll(t, pool, groupID, studentID, "NOW()")
	payFor(t, pool, groupID, studentID, 4)

	deleted, err := repository.NewStudentRepository(pool).Delete(context.Background(), studentID, tutorID)
	require.NoError(t, err)
	assert.False(t, deleted)
	assert.True(t, studentExists(t, pool, studentID))
}

func TestPaymentGetByID_ScopedByTutor(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	tutorID, studentID := seedTutorStudent(t, pool)
	otherTutor, _ := seedTutorStudent(t, pool)
	courseID := addIndividualCourse(t, pool, tutorID, studentID)
	payFor(t, pool, courseID, studentID, 8)

	var paymentID string
	require.NoError(t, pool.QueryRow(ctx, `SELECT id FROM payments WHERE course_id = $1`, courseID).Scan(&paymentID))

	repo := repository.NewPaymentRepository(pool)
	p, err := repo.GetByID(ctx, paymentID, tutorID)
	require.NoError(t, err)
	require.NotNil(t, p.StudentID)
	assert.Equal(t, studentID, *p.StudentID)

	_, err = repo.GetByID(ctx, paymentID, otherTutor)
	assert.True(t, errors.Is(err, pgx.ErrNoRows))
}

// История курса показывает адресата платежа группы, а легаси-платёж — без него.
func TestPaymentGetByCourse_NameComesFromAddressee(t *testing.T) {
	pool := testPool(t)
	tutorID, studentID := seedTutorStudent(t, pool)
	groupID := addGroupCourse(t, pool, tutorID, "Группа")
	enroll(t, pool, groupID, studentID, "NOW() - interval '1 month'")
	payFor(t, pool, groupID, studentID, 4)
	addPayment(t, pool, groupID, 5000, 4, "NOW() - interval '1 day'")

	list, total, err := repository.NewPaymentRepository(pool).
		GetByCourse(context.Background(), groupID, models.Pagination{Page: 1, Limit: 20})
	require.NoError(t, err)
	require.Equal(t, 2, total)
	require.Len(t, list, 2)

	require.NotNil(t, list[0].StudentName)
	assert.Equal(t, "Иван Петров", *list[0].StudentName)
	assert.Nil(t, list[1].StudentID)
	assert.Nil(t, list[1].StudentName)
}
```

- [ ] **Step 6: Убедиться, что тесты падают**

Run: `go vet -tags=integration ./repository/`
Expected: FAIL — `repo.IsEnrolled undefined`, `repo.GetByID undefined`.

- [ ] **Step 7: Репозитории**

In `repository/payment.go`:

1. Add `GetByID(ctx context.Context, id string, tutorID string) (models.Payment, error)` to `PaymentRepository` (after `Create`).
2. Add right after `NewPaymentRepository`:

```go
// Одна константа на все выборки платежа — тот же приём, что lessonColumns в
// lesson.go: колонку, добавленную руками в семь запросов, где-нибудь забудут.
// Алиас `p` обязателен и там, где джойна нет.
const paymentColumns = `p.id, p.course_id, p.student_id, p.amount, p.lessons_count, p.paid_at`

// paymentDest — приёмники Scan в порядке paymentColumns; списки дописывают свои
// поля через append.
func paymentDest(p *models.Payment) []any {
	return []any{&p.ID, &p.CourseID, &p.StudentID, &p.Amount, &p.LessonsCount, &p.PaidAt}
}
```

3. Replace `Create`:

```go
func (r *paymentRepository) Create(ctx context.Context, req models.CreatePaymentRequest) (models.Payment, error) {
	var payment models.Payment
	err := r.conn.QueryRow(ctx,
		`INSERT INTO payments AS p (course_id, student_id, amount, lessons_count, paid_at)
		 VALUES ($1, $2, $3, $4, $5)
		 RETURNING `+paymentColumns,
		req.CourseID, req.StudentID, req.Amount, req.LessonsCount, req.PaidAt,
	).Scan(paymentDest(&payment)...)
	return payment, err
}

// GetByID — платёж репетитора; чужой неотличим от несуществующего.
func (r *paymentRepository) GetByID(ctx context.Context, id string, tutorID string) (models.Payment, error) {
	var payment models.Payment
	err := r.conn.QueryRow(ctx,
		`SELECT `+paymentColumns+`
		 FROM payments p
		 JOIN courses c ON c.id = p.course_id
		 WHERE p.id = $1 AND c.tutor_id = $2`, id, tutorID,
	).Scan(paymentDest(&payment)...)
	return payment, err
}
```

4. In `GetByCourse` keep the COUNT query; replace the list query and its scan:

```go
	rows, err := r.conn.Query(ctx,
		`SELECT `+paymentColumns+`, `+studentNameExpr+`
		 FROM payments p
		 LEFT JOIN students s ON s.id = p.student_id
		 WHERE p.course_id = $1
		 ORDER BY p.paid_at DESC
		 LIMIT $2 OFFSET $3`,
		courseID, p.Limit, p.Offset())
```
with scan `rows.Scan(append(paymentDest(&payment), &payment.StudentName)...)`.

5. In `GetByCoursesBatch`: query becomes `SELECT `+paymentColumns+` FROM payments p WHERE p.course_id = ANY($1) ORDER BY p.course_id, p.paid_at ASC`, scan `rows.Scan(paymentDest(&p)...)`. In `GetPaymentsForCalendar`: `SELECT `+paymentColumns+` FROM payments p WHERE p.course_id IN (…unchanged…) ORDER BY p.course_id, p.paid_at ASC`, same scan. Note: the local variable in these loops is named `p` — keep `paymentDest(&p)`.

6. Replace `studentNameExpr` and its comment:

```go
// studentNameExpr — имя адресата платежа; NULL у легаси-платежа группы без
// адресата (спека, п. 3.4). Ожидает LEFT JOIN students s ON s.id = p.student_id.
// TRIM с COALESCE, а не конкатенация напрямую: last_name в схеме nullable, и
// `first_name || ' ' || NULL` даёт NULL — имя пропало бы целиком.
const studentNameExpr = `CASE WHEN p.student_id IS NULL THEN NULL
                              ELSE TRIM(s.first_name || ' ' || COALESCE(s.last_name, ''))
                         END`
```

7. In `GetAllByTutor` and `GetAllByTutorPaged` (list query only): `SELECT `+paymentColumns+`, c.subject, `+studentNameExpr+``, join `LEFT JOIN students s ON s.id = p.student_id` (was `c.student_id`), scan `append(paymentDest(&p), &p.Subject, &p.StudentName)...` (variable name as in each function).

8. Replace `Update`:

```go
func (r *paymentRepository) Update(ctx context.Context, id string, tutorID string, req models.UpdatePaymentRequest) (models.Payment, error) {
	var payment models.Payment
	err := r.conn.QueryRow(ctx,
		`UPDATE payments AS p SET student_id=$1, amount=$2, lessons_count=$3, paid_at=$4
		 WHERE p.id=$5 AND p.course_id IN (SELECT id FROM courses WHERE tutor_id=$6)
		 RETURNING `+paymentColumns,
		req.StudentID, req.Amount, req.LessonsCount, req.PaidAt, id, tutorID,
	).Scan(paymentDest(&payment)...)
	if errors.Is(err, pgx.ErrNoRows) {
		return models.Payment{}, errors.New("payment not found")
	}
	return payment, err
}
```

In `repository/enrollment.go` add `IsEnrolled(ctx context.Context, courseID, studentID string) (bool, error)` to the interface and:

```go
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
```

In `repository/student.go` `Delete`: add the condition as the last line of the `WHERE`:

```sql
		   AND NOT EXISTS (SELECT 1 FROM payments p WHERE p.student_id = s.id)
```
and extend the doc comment's list of history traces: «платежей (по его курсам и адресованных ему — фаза 2, спека п. 6.3)».

- [ ] **Step 8: Интеграционные тесты зелёные**

Run: `TEST_DB_URL="postgres://dev:dev@localhost:5432/tutorgo_dev?sslmode=disable" go test -tags=integration -count=1 ./repository/ -run 'TestIsEnrolled|TestStudentDelete|TestPaymentGetByID|TestPaymentGetByCourse|TestMonthlyExpected' -v`
Expected: PASS.

- [ ] **Step 9: Падающие unit-тесты сервиса**

In `service/enrollment_test.go` add to `mockEnrollmentRepo`:

```go
func (m *mockEnrollmentRepo) IsEnrolled(ctx context.Context, courseID, studentID string) (bool, error) {
	args := m.Called(ctx, courseID, studentID)
	return args.Bool(0), args.Error(1)
}
```

In `service/payment_test.go`:
- add to `mockPaymentRepo`:
```go
func (m *mockPaymentRepo) GetByID(ctx context.Context, id string, tutorID string) (models.Payment, error) {
	args := m.Called(ctx, id, tutorID)
	return args.Get(0).(models.Payment), args.Error(1)
}
```
- in the `var (...)` block add `payStudentID = "student-uuid-1"`, `StudentID: payStudentID` to `paymentReq`, `StudentID: &payStudentID` to `expectedPayment`, and a new var:
```go
	// individualCourse — курс ученика payStudentID: платёж ему проходит проверку
	// связи без записи в группу.
	individualCourse = models.Course{
		ID:        courseID,
		TutorID:   tutorID,
		StudentID: &payStudentID,
		IsActive:  true,
	}
```
- change the helper and **every** call site (pass `new(mockEnrollmentRepo)` where a test does not care):
```go
func newPaymentSvc(payRepo *mockPaymentRepo, courseRepo *mockCourseRepo, enrollRepo *mockEnrollmentRepo) service.PaymentService {
	return service.NewPaymentService(payRepo, courseRepo, enrollRepo)
}
```
- in `TestPaymentCreate_Success` and `TestPaymentCreate_RepoError` the course mock returns `individualCourse` instead of `expectedCourse`.
- add tests:

```go
// Индивидуальный курс чужого ученика — 400, в базу ничего не пишется (спека, п. 6.3).
func TestPaymentCreate_IndividualCourseOtherStudentRejected(t *testing.T) {
	payRepo := new(mockPaymentRepo)
	courseRepo := new(mockCourseRepo)
	svc := newPaymentSvc(payRepo, courseRepo, new(mockEnrollmentRepo))

	other := "student-uuid-2"
	course := individualCourse
	course.StudentID = &other
	courseRepo.On("GetByID", mock.Anything, courseID, tutorID).Return(course, nil)

	_, err := svc.Create(context.Background(), paymentReq, tutorID)

	assert.ErrorIs(t, err, service.ErrBadRequest)
	payRepo.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
}

func TestPaymentCreate_GroupRequiresEnrollment(t *testing.T) {
	for _, tc := range []struct {
		name     string
		enrolled bool
	}{{"enrolled", true}, {"stranger", false}} {
		t.Run(tc.name, func(t *testing.T) {
			payRepo := new(mockPaymentRepo)
			courseRepo := new(mockCourseRepo)
			enrollRepo := new(mockEnrollmentRepo)
			svc := newPaymentSvc(payRepo, courseRepo, enrollRepo)

			courseRepo.On("GetByID", mock.Anything, courseID, tutorID).Return(expectedCourse, nil)
			enrollRepo.On("IsEnrolled", mock.Anything, courseID, payStudentID).Return(tc.enrolled, nil)
			if tc.enrolled {
				payRepo.On("Create", mock.Anything, paymentReq).Return(expectedPayment, nil)
			}

			_, err := svc.Create(context.Background(), paymentReq, tutorID)

			if tc.enrolled {
				assert.NoError(t, err)
			} else {
				assert.ErrorIs(t, err, service.ErrBadRequest)
				payRepo.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
			}
			enrollRepo.AssertExpectations(t)
		})
	}
}

// Правка адресата проверяется так же, как создание: иначе легаси-платёж группы
// можно «починить» на ученика, которого в группе не было.
func TestPaymentUpdate_ChecksStudentOnCourse(t *testing.T) {
	payRepo := new(mockPaymentRepo)
	courseRepo := new(mockCourseRepo)
	enrollRepo := new(mockEnrollmentRepo)
	svc := newPaymentSvc(payRepo, courseRepo, enrollRepo)

	req := models.UpdatePaymentRequest{StudentID: payStudentID, Amount: 5000, LessonsCount: 4, PaidAt: paymentReq.PaidAt}
	payRepo.On("GetByID", mock.Anything, "payment-uuid-1", tutorID).Return(models.Payment{ID: "payment-uuid-1", CourseID: courseID}, nil)
	courseRepo.On("GetByID", mock.Anything, courseID, tutorID).Return(expectedCourse, nil)
	enrollRepo.On("IsEnrolled", mock.Anything, courseID, payStudentID).Return(false, nil)

	_, err := svc.Update(context.Background(), "payment-uuid-1", tutorID, req)

	assert.ErrorIs(t, err, service.ErrBadRequest)
	payRepo.AssertNotCalled(t, "Update", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func TestPaymentUpdate_PaymentNotFound(t *testing.T) {
	payRepo := new(mockPaymentRepo)
	svc := newPaymentSvc(payRepo, new(mockCourseRepo), new(mockEnrollmentRepo))

	payRepo.On("GetByID", mock.Anything, "missing", tutorID).Return(models.Payment{}, errors.New("no rows"))

	_, err := svc.Update(context.Background(), "missing", tutorID, models.UpdatePaymentRequest{StudentID: payStudentID})

	assert.ErrorIs(t, err, service.ErrNotFound)
}
```
If an existing `TestPaymentUpdate_*` test exists, adapt it to the new flow (`GetByID` → course → check → `Update`). Add `errors` to imports if missing.

In `handlers/payment_test.go` add `StudentID: testStudentID` to `testCreatePaymentReq`; if a test posts an update body, add `student_id` there too.

Run: `go vet ./...`
Expected: FAIL — `too many arguments in call to service.NewPaymentService`.

- [ ] **Step 10: Сервис и проводка**

In `service/payment.go`:

```go
type paymentService struct {
	repo           repository.PaymentRepository
	courseRepo     repository.CourseRepository
	enrollmentRepo repository.EnrollmentRepository
}

func NewPaymentService(repo repository.PaymentRepository, courseRepo repository.CourseRepository, enrollmentRepo repository.EnrollmentRepository) PaymentService {
	return &paymentService{repo: repo, courseRepo: courseRepo, enrollmentRepo: enrollmentRepo}
}

// studentOnCourse проверяет, что платёж адресован ученику этого курса (спека,
// п. 6.3). Индивидуальный курс — ученик обязан совпасть с course.student_id,
// групповой — иметь запись, пусть и закрытую уходом. Иначе платёж уедет в
// баланс, который нигде не показывается.
func (s *paymentService) studentOnCourse(ctx context.Context, course models.Course, studentID string) error {
	if course.StudentID != nil {
		if *course.StudentID != studentID {
			return fmt.Errorf("student is not on course: %w", ErrBadRequest)
		}
		return nil
	}
	enrolled, err := s.enrollmentRepo.IsEnrolled(ctx, course.ID, studentID)
	if err != nil {
		return err
	}
	if !enrolled {
		return fmt.Errorf("student is not on course: %w", ErrBadRequest)
	}
	return nil
}

func (s *paymentService) Create(ctx context.Context, req models.CreatePaymentRequest, tutorID string) (models.Payment, error) {
	course, err := s.courseRepo.GetByID(ctx, req.CourseID, tutorID)
	if err != nil {
		return models.Payment{}, fmt.Errorf("course: %w", ErrNotFound)
	}
	if err := s.studentOnCourse(ctx, course, req.StudentID); err != nil {
		return models.Payment{}, err
	}
	payment, err := s.repo.Create(ctx, req)
	if err == nil {
		globalCalendarCache.Invalidate(tutorID)
	}
	return payment, err
}

func (s *paymentService) Update(ctx context.Context, id string, tutorID string, req models.UpdatePaymentRequest) (models.Payment, error) {
	existing, err := s.repo.GetByID(ctx, id, tutorID)
	if err != nil {
		return models.Payment{}, fmt.Errorf("payment: %w", ErrNotFound)
	}
	course, err := s.courseRepo.GetByID(ctx, existing.CourseID, tutorID)
	if err != nil {
		return models.Payment{}, fmt.Errorf("course: %w", ErrNotFound)
	}
	if err := s.studentOnCourse(ctx, course, req.StudentID); err != nil {
		return models.Payment{}, err
	}
	payment, err := s.repo.Update(ctx, id, tutorID, req)
	if err != nil {
		return models.Payment{}, fmt.Errorf("payment: %w", ErrNotFound)
	}
	globalCalendarCache.Invalidate(tutorID)
	return payment, nil
}
```

In `router/router.go:54`: `paymentService := service.NewPaymentService(paymentRepo, courseRepo, enrollmentRepo)` (`enrollmentRepo` уже создан строкой 37).

- [ ] **Step 11: Всё зелёное**

Run:
```bash
go build ./... && go vet ./... && go vet -tags=integration ./repository/
make test
TEST_DB_URL="postgres://dev:dev@localhost:5432/tutorgo_dev?sslmode=disable" go test -tags=integration -count=1 ./repository/
```
Expected: сборка и vet чистые; `make test` PASS; интеграция PASS, кроме известного `TestGetAllByTutor_CarriesSubjectAndStudentName`.

- [ ] **Step 12: Commit**

```bash
git add migrations/039_payments_student.sql models/payment.go repository/payment.go repository/enrollment.go repository/student.go service/payment.go router/router.go service/payment_test.go service/enrollment_test.go handlers/payment_test.go repository/payment_integration_test.go repository/payment_student_integration_test.go
git commit -m "feat(payments): миграция 039 — платёж адресный, проверка ученика на курсе

Co-Authored-By: Claude Opus 5 <noreply@anthropic.com>
Claude-Session: https://claude.ai/code/session_01KLiSrLKjmzZMQgDzgsUdbn"
```

---

### Task 2: Периоды участия, баланс и прогноз по ученику

**Files:**
- Modify: `repository/payment.go` (новые `studentCoursePairs`, `lessonInParticipation`; `GetBalance`, `GetMonthlyExpected`)
- Modify: `service/payment.go` (`GetBalance`)
- Modify: `handlers/payment.go` (`GetBalance`)
- Modify: `service/payment_test.go`, `handlers/mocks_test.go`, `handlers/payment_test.go`
- Create: `repository/balance_integration_test.go`

**Interfaces:**
- Consumes (Task 1): `addStudent`, `enroll`, `payFor`; `(*paymentService).studentOnCourse`; `newPaymentSvc(payRepo, courseRepo, enrollRepo)`; `individualCourse`, `payStudentID`.
- Produces:
  - `repository.studentCoursePairs` (const) — выборка `course_id, student_id, enrolled_at, left_at`
  - `repository.lessonInParticipation` (const) — предикат, ожидает алиасы `l` (lessons) и `sc` (`student_id`, `enrolled_at`, `left_at`)
  - `PaymentRepository.GetBalance(ctx context.Context, courseID, studentID string) (models.CourseBalance, error)`
  - `PaymentService.GetBalance(ctx context.Context, courseID, studentID, tutorID string) (models.CourseBalance, error)`
  - integration-хелпер `addPause(t *testing.T, pool *pgxpool.Pool, studentID, startsExpr, endsExpr string)`

- [ ] **Step 1: Написать падающие интеграционные тесты**

Create `repository/balance_integration_test.go`:

```go
//go:build integration

package repository_test

import (
	"context"
	"fmt"
	"testing"

	"tutorgo/models"
	"tutorgo/repository"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Баланс и прогноз считаются по паре «курс + ученик» в периодах участия
// (спека 2026-09-06-price-units…, п. 3.6, 3.9, 6.4, 6.6). Запуск:
// make test-integration.

// addPause — заморозка ученика; границы — SQL-выражения дат.
func addPause(t *testing.T, pool *pgxpool.Pool, studentID, startsExpr, endsExpr string) {
	_, err := pool.Exec(context.Background(),
		fmt.Sprintf(`INSERT INTO student_pauses (student_id, starts_on, ends_on)
		             VALUES ($1, %s, %s)`, startsExpr, endsExpr),
		studentID)
	require.NoError(t, err)
}

func balanceOf(t *testing.T, pool *pgxpool.Pool, courseID, studentID string) models.CourseBalance {
	b, err := repository.NewPaymentRepository(pool).GetBalance(context.Background(), courseID, studentID)
	require.NoError(t, err)
	return b
}

// Группа из трёх: каждый оплатил 4, проведено 2, один отмечен absent — у всех
// осталось по 2. Посещаемость денег не касается (спека, п. 3.6).
func TestBalance_GroupAttendanceDoesNotTouchMoney(t *testing.T) {
	pool := testPool(t)
	tutorID, a := seedTutorStudent(t, pool)
	b := addStudent(t, pool, tutorID, "Бекзат")
	c := addStudent(t, pool, tutorID, "Вика")
	groupID := addGroupCourse(t, pool, tutorID, "Группа")
	for _, s := range []string{a, b, c} {
		enroll(t, pool, groupID, s, "NOW() - interval '1 month'")
		payFor(t, pool, groupID, s, 4)
	}
	first := addLessonAt(t, pool, groupID, "NOW() - interval '2 weeks'", "completed")
	addLessonAt(t, pool, groupID, "NOW() - interval '1 week'", "completed")
	_, err := pool.Exec(context.Background(),
		`INSERT INTO lesson_attendances (lesson_id, student_id, status) VALUES ($1, $2, 'absent')`, first, c)
	require.NoError(t, err)

	for _, s := range []string{a, b, c} {
		assert.Equal(t, models.CourseBalance{LessonsPaid: 4, LessonsCompleted: 2, LessonsRemaining: 2},
			balanceOf(t, pool, groupID, s))
	}
}

// Записан после третьего занятия — за них не должен (спека, п. 3.7).
func TestBalance_LateEnrolleeOwesNothingForEarlierLessons(t *testing.T) {
	pool := testPool(t)
	tutorID, s := seedTutorStudent(t, pool)
	groupID := addGroupCourse(t, pool, tutorID, "Группа")
	for i := 3; i >= 1; i-- {
		addLessonAt(t, pool, groupID, fmt.Sprintf("NOW() - interval '%d weeks'", i), "completed")
	}
	enroll(t, pool, groupID, s, "NOW() - interval '1 day'")

	assert.Equal(t, models.CourseBalance{}, balanceOf(t, pool, groupID, s))
}

// Отменённый не сгорает, пропущенный — сгорает (спека, п. 3.6).
func TestBalance_CancelledNotBurnedMissedBurned(t *testing.T) {
	pool := testPool(t)
	tutorID, s := seedTutorStudent(t, pool)
	courseID := addIndividualCourse(t, pool, tutorID, s)
	addLessonAt(t, pool, courseID, "NOW() - interval '3 days'", "cancelled")
	addLessonAt(t, pool, courseID, "NOW() - interval '2 days'", "missed")
	addLessonAt(t, pool, courseID, "NOW() - interval '1 day'", "completed")
	payFor(t, pool, courseID, s, 8)

	assert.Equal(t, models.CourseBalance{LessonsPaid: 8, LessonsCompleted: 2, LessonsRemaining: 6},
		balanceOf(t, pool, courseID, s))
}

// Платёж за математику не меняет баланс физики.
func TestBalance_TwoSubjectsAreSeparate(t *testing.T) {
	pool := testPool(t)
	tutorID, s := seedTutorStudent(t, pool)
	math := addIndividualCourse(t, pool, tutorID, s)
	var physics string
	require.NoError(t, pool.QueryRow(context.Background(),
		`INSERT INTO courses (student_id, tutor_id, subject, price_per_cycle, lessons_per_cycle, started_at)
		 VALUES ($1, $2, 'Физика', 6000, 1, NOW() - interval '1 month') RETURNING id`,
		s, tutorID).Scan(&physics))
	addLessonAt(t, pool, physics, "NOW() - interval '1 day'", "completed")
	payFor(t, pool, math, s, 8)

	assert.Equal(t, models.CourseBalance{LessonsPaid: 0, LessonsCompleted: 1, LessonsRemaining: -1},
		balanceOf(t, pool, physics, s))
	assert.Equal(t, 8, balanceOf(t, pool, math, s).LessonsPaid)
}

// Ушедший: занятия после ухода не сгорают; легаси-платёж группы без адресата
// в его баланс не входит (спека, п. 3.4, 5a.4).
func TestBalance_LeftStudentStopsBurningAndLegacyPaymentIgnored(t *testing.T) {
	pool := testPool(t)
	tutorID, s := seedTutorStudent(t, pool)
	groupID := addGroupCourse(t, pool, tutorID, "Группа")
	enroll(t, pool, groupID, s, "NOW() - interval '1 month'")
	addLessonAt(t, pool, groupID, "NOW() - interval '2 weeks'", "completed")
	_, err := pool.Exec(context.Background(),
		`UPDATE course_enrollments SET left_at = NOW() - interval '10 days'
		 WHERE course_id = $1 AND student_id = $2`, groupID, s)
	require.NoError(t, err)
	addLessonAt(t, pool, groupID, "NOW() - interval '1 week'", "completed")
	addPayment(t, pool, groupID, 5000, 4, "NOW() - interval '3 weeks'")

	assert.Equal(t, models.CourseBalance{LessonsPaid: 0, LessonsCompleted: 1, LessonsRemaining: -1},
		balanceOf(t, pool, groupID, s))
}

// Заморозка задним числом: урок в паузе не сгорает ни на индивидуальном курсе,
// ни в группе, где занятие шло для остальных (спека, п. 6.9).
func TestBalance_PauseExcludesLessonsOnIndividualAndGroup(t *testing.T) {
	pool := testPool(t)
	tutorID, s := seedTutorStudent(t, pool)
	courseID := addIndividualCourse(t, pool, tutorID, s)
	groupID := addGroupCourse(t, pool, tutorID, "Группа")
	enroll(t, pool, groupID, s, "NOW() - interval '1 month'")
	addLessonAt(t, pool, courseID, "NOW() - interval '20 days'", "completed")
	addLessonAt(t, pool, courseID, "NOW() - interval '3 days'", "completed")
	addLessonAt(t, pool, groupID, "NOW() - interval '3 days'", "completed")
	addPause(t, pool, s, "CURRENT_DATE - 7", "CURRENT_DATE - 1")

	assert.Equal(t, 1, balanceOf(t, pool, courseID, s).LessonsCompleted)
	assert.Equal(t, 0, balanceOf(t, pool, groupID, s).LessonsCompleted)
}

// Группа из пяти по 5000 за урок: прогноз — пять пакетов, а не один (спека, п. 6.6).
func TestMonthlyExpected_GroupCountsEveryMember(t *testing.T) {
	pool := testPool(t)
	tutorID, first := seedTutorStudent(t, pool)
	groupID := addGroupCourse(t, pool, tutorID, "Группа")
	members := []string{first}
	for i := 0; i < 4; i++ {
		members = append(members, addStudent(t, pool, tutorID, fmt.Sprintf("Ученик %d", i)))
	}
	for _, m := range members {
		enroll(t, pool, groupID, m, "date_trunc('month', NOW()) - interval '1 month'")
	}
	addLessonAt(t, pool, groupID, "date_trunc('month', NOW()) + interval '10 days'", "scheduled")

	total, err := repository.NewPaymentRepository(pool).GetMonthlyExpected(context.Background(), tutorID)
	require.NoError(t, err)
	assert.Equal(t, 25000.0, total)
}
```

Run: `go vet -tags=integration ./repository/`
Expected: FAIL — `too many arguments in call to …GetBalance`.

- [ ] **Step 2: Предикат, баланс, прогноз**

In `repository/payment.go` change the interface line to `GetBalance(ctx context.Context, courseID, studentID string) (models.CourseBalance, error)` and add next to `studentNameExpr`:

```go
// studentCoursePairs — пары «курс + ученик» с периодом участия (спека, п. 3.9).
// Индивидуальный курс — одна пара без границ: курс заводится вместе с учеником,
// все его уроки — его. Группа — строка course_enrollments на участника, включая
// ушедших: долг не исчезает оттого, что человек перестал ходить. Фильтров по
// активности курса и ученика нет намеренно — их добавляет запрос, которому они
// нужны (прогноз); долгам они вредны (спека, п. 6.5).
const studentCoursePairs = `
	SELECT c.id AS course_id, c.student_id,
	       NULL::timestamptz AS enrolled_at, NULL::timestamptz AS left_at
	  FROM courses c
	 WHERE c.student_id IS NOT NULL
	UNION ALL
	SELECT ce.course_id, ce.student_id, ce.enrolled_at, ce.left_at
	  FROM course_enrollments ce`

// lessonInParticipation — урок l считается уроком ученика пары sc: не раньше
// записи, раньше ухода и не в его заморозке. Одно выражение на баланс, долги,
// прогноз и позицию в цикле (спека, п. 6.4): бейдж «3 из 8» и строка «оплачено
// 3 из 8» обязаны считать одинаково. Статус урока сюда не входит: сгорание
// (completed/missed) и ранги (всё, кроме cancelled) добавляют его сами.
// Посещаемости нет намеренно — деньги её не касаются (спека, п. 3.6).
//
// Ожидает алиасы l (lessons) и sc (student_id, enrolled_at, left_at).
//
// ponytail: границы паузы — DATE, а scheduled_at приводится к дате в зоне
// сессии, то есть в UTC: урок между полуночью и 05:00 по Алматы попадёт в
// соседний день. Часового пояса у тьютора в схеме нет; апгрейд — приводить
// через его tz, когда он появится (спека, п. 6.1).
const lessonInParticipation = `l.scheduled_at >= COALESCE(sc.enrolled_at, '-infinity'::timestamptz)
	   AND l.scheduled_at <  COALESCE(sc.left_at, 'infinity'::timestamptz)
	   AND NOT EXISTS (SELECT 1 FROM student_pauses sp
	                    WHERE sp.student_id = sc.student_id
	                      AND l.scheduled_at::date BETWEEN sp.starts_on AND sp.ends_on)`
```

Replace `GetBalance`:

```go
// GetBalance — баланс пары «курс + ученик» (спека, п. 6.4): оплачено этим
// учеником против сгоревшего в его периодах участия. Легаси-платежи группы без
// адресата сюда не входят (п. 3.4). Пары нет — сгоревших нет.
func (r *paymentRepository) GetBalance(ctx context.Context, courseID, studentID string) (models.CourseBalance, error) {
	var paid, burned int
	err := r.conn.QueryRow(ctx,
		`WITH sc AS (
		     SELECT * FROM (`+studentCoursePairs+`) pr
		      WHERE pr.course_id = $1 AND pr.student_id = $2
		 )
		 SELECT
		     COALESCE((SELECT SUM(lessons_count) FROM payments
		                WHERE course_id = $1 AND student_id = $2), 0),
		     (SELECT count(*) FROM lessons l, sc
		       WHERE l.course_id = sc.course_id
		         AND l.status IN ('completed', 'missed')
		         AND `+lessonInParticipation+`)`,
		courseID, studentID,
	).Scan(&paid, &burned)
	if err != nil {
		return models.CourseBalance{}, err
	}
	return models.CourseBalance{
		LessonsPaid:      paid,
		LessonsCompleted: burned,
		LessonsRemaining: paid - burned,
	}, nil
}
```

Replace `GetMonthlyExpected` and its doc comment:

```go
// GetMonthlyExpected — сумма платежей, ожидаемых к поступлению в текущем месяце.
//
// Ученик платит за цикл на его первом уроке, поэтому ожидаемое поступление
// привязано к дате первого урока каждого ещё не оплаченного цикла. Считается
// по паре «курс + ученик» (спека, п. 6.6): пакет группы — цена за одного
// участника, и ждут его от каждого. Уроки пары ранжируются по scheduled_at в
// периодах участия ученика — тем же lessonInParticipation, что и сгорание в
// балансе, — а его платежи покрывают ранги нарастающим итогом по lessons_count.
// Урок с rank = paid_through+1 открывает первый неоплаченный цикл, дальше
// старты идут с шагом lessons_per_cycle. Ушедшие и замороженные в прогноз не
// попадают сами: их уроков в периодах участия нет.
//
// Просроченные ожидания (урок цикла уже прошёл, а платежа нет) остаются в сумме:
// деньги ждали в этом месяце и не пришли — долг не должен исчезать из метрики.
//
// ponytail: будущие циклы нарезаются по плановому lessons_per_cycle, тогда как
// прошлые — по фактическим lessons_count платежей. Если ученик регулярно платит
// нестандартными пачками, даты прогноза поплывут. Апгрейд — медиана lessons_count
// последних платежей пары; делать, только если реально разъедется.
func (r *paymentRepository) GetMonthlyExpected(ctx context.Context, tutorID string) (float64, error) {
	var total float64
	err := r.conn.QueryRow(ctx,
		`WITH sc AS (
		     SELECT pr.course_id, pr.student_id, pr.enrolled_at, pr.left_at,
		            c.price_per_cycle, c.lessons_per_cycle,
		            COALESCE((SELECT SUM(p.lessons_count) FROM payments p
		                       WHERE p.course_id = pr.course_id
		                         AND p.student_id = pr.student_id), 0) AS paid_through
		       FROM (`+studentCoursePairs+`) pr
		       JOIN courses c ON c.id = pr.course_id
		      WHERE c.tutor_id = $1 AND c.is_active
		 ),
		 ranked AS (
		     SELECT sc.course_id, sc.student_id, l.scheduled_at,
		            ROW_NUMBER() OVER (PARTITION BY sc.course_id, sc.student_id
		                               ORDER BY l.scheduled_at) AS rank
		       FROM sc
		       JOIN lessons l ON l.course_id = sc.course_id
		      WHERE l.status != 'cancelled'
		        AND `+lessonInParticipation+`
		 )
		 SELECT COALESCE(SUM(sc.price_per_cycle), 0)
		   FROM ranked r
		   JOIN sc ON sc.course_id = r.course_id AND sc.student_id = r.student_id
		  WHERE r.rank > sc.paid_through
		    AND (r.rank - sc.paid_through - 1) % sc.lessons_per_cycle = 0
		    AND r.scheduled_at >= date_trunc('month', NOW())
		    AND r.scheduled_at <  date_trunc('month', NOW()) + interval '1 month'`,
		tutorID,
	).Scan(&total)
	return total, err
}
```

Run: `TEST_DB_URL="postgres://dev:dev@localhost:5432/tutorgo_dev?sslmode=disable" go test -tags=integration -count=1 ./repository/ -run 'TestBalance|TestMonthlyExpected' -v`
Expected: PASS (включая прежние `TestMonthlyExpected_*`).

- [ ] **Step 3: Падающие unit-тесты сервиса и хендлера**

In `service/payment_test.go`: mock signature
```go
func (m *mockPaymentRepo) GetBalance(ctx context.Context, courseID, studentID string) (models.CourseBalance, error) {
	args := m.Called(ctx, courseID, studentID)
	return args.Get(0).(models.CourseBalance), args.Error(1)
}
```
`TestPaymentGetBalance_Success`: course mock returns `individualCourse`, repo expects `GetBalance(mock.Anything, courseID, payStudentID)`, call `svc.GetBalance(ctx, courseID, payStudentID, tutorID)`. `TestPaymentGetBalance_CourseNotFound`: call with `payStudentID`. Add:

```go
// Баланс чужого для курса ученика — 400, а не нули: нули выглядели бы как
// «всё оплачено» (спека, п. 6.3).
func TestPaymentGetBalance_StudentNotOnCourse(t *testing.T) {
	payRepo := new(mockPaymentRepo)
	courseRepo := new(mockCourseRepo)
	svc := newPaymentSvc(payRepo, courseRepo, new(mockEnrollmentRepo))

	courseRepo.On("GetByID", mock.Anything, courseID, tutorID).Return(individualCourse, nil)

	_, err := svc.GetBalance(context.Background(), courseID, "student-uuid-2", tutorID)

	assert.ErrorIs(t, err, service.ErrBadRequest)
	payRepo.AssertNotCalled(t, "GetBalance", mock.Anything, mock.Anything, mock.Anything)
}
```

In `handlers/mocks_test.go`:
```go
func (m *mockPaymentService) GetBalance(ctx context.Context, courseID, studentID, tutorID string) (models.CourseBalance, error) {
	args := m.Called(ctx, courseID, studentID, tutorID)
	return args.Get(0).(models.CourseBalance), args.Error(1)
}
```
In `handlers/payment_test.go`: in `TestPaymentGetBalance_Success` and `_ServiceError` the URL becomes `"/payments/balance?course_id="+testCourseID+"&student_id="+testStudentID` and the mock expects `(mock.Anything, testCourseID, testStudentID, testTutorID)`. Add:

```go
func TestPaymentGetBalance_MissingStudentID(t *testing.T) {
	svc := new(mockPaymentService)
	r := newPaymentRouter(svc, testTutorID)

	w := makeRequest(t, r, http.MethodGet, "/payments/balance?course_id="+testCourseID, nil)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	svc.AssertNotCalled(t, "GetBalance")
}
```

Run: `go vet ./...`
Expected: FAIL — `*paymentService does not implement PaymentService` / wrong argument count.

- [ ] **Step 4: Сервис и хендлер**

`service/payment.go` — interface line `GetBalance(ctx context.Context, courseID, studentID, tutorID string) (models.CourseBalance, error)` and:

```go
// GetBalance — баланс ученика по курсу (спека, п. 6.4). Ученик обязан быть на
// курсе: у чужого баланс вышел бы нулевым и читался бы как «всё оплачено».
func (s *paymentService) GetBalance(ctx context.Context, courseID, studentID, tutorID string) (models.CourseBalance, error) {
	course, err := s.courseRepo.GetByID(ctx, courseID, tutorID)
	if err != nil {
		return models.CourseBalance{}, fmt.Errorf("course: %w", ErrNotFound)
	}
	if err := s.studentOnCourse(ctx, course, studentID); err != nil {
		return models.CourseBalance{}, err
	}
	return s.repo.GetBalance(ctx, courseID, studentID)
}
```

`handlers/payment.go` `GetBalance` — replace the query parsing:

```go
	courseID := c.Query("course_id")
	studentID := c.Query("student_id")
	// Баланс — свойство пары «курс + ученик»: у группы без ученика он ничего не
	// значит (спека, п. 6.4).
	if courseID == "" || studentID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "course_id and student_id are required"})
		return
	}
	balance, err := h.service.GetBalance(c.Request.Context(), courseID, studentID, tutorID)
```

- [ ] **Step 5: Всё зелёное**

Run:
```bash
go build ./... && go vet ./... && go vet -tags=integration ./repository/
make test
TEST_DB_URL="postgres://dev:dev@localhost:5432/tutorgo_dev?sslmode=disable" go test -tags=integration -count=1 ./repository/
```
Expected: PASS, кроме известного `TestGetAllByTutor_CarriesSubjectAndStudentName`.

- [ ] **Step 6: Commit**

```bash
git add repository/payment.go service/payment.go handlers/payment.go service/payment_test.go handlers/mocks_test.go handlers/payment_test.go repository/balance_integration_test.go
git commit -m "feat(payments): баланс и прогноз по ученику в периодах участия

Co-Authored-By: Claude Opus 5 <noreply@anthropic.com>
Claude-Session: https://claude.ai/code/session_01KLiSrLKjmzZMQgDzgsUdbn"
```

---

### Task 3: Долги — `GET /payments/debts`

**Files:**
- Modify: `models/payment.go` (`CourseDebt`, `DebtByCourse`, `StudentDebt`)
- Modify: `repository/payment.go` (`GetDebts`)
- Modify: `service/payment.go` (`GetDebts`)
- Modify: `handlers/payment.go` (`GetDebts`), `router/router.go`
- Modify: `service/payment_test.go`, `handlers/mocks_test.go`, `handlers/payment_test.go`
- Create: `repository/debts_integration_test.go`

**Interfaces:**
- Consumes (Tasks 1–2): `studentCoursePairs`, `lessonInParticipation`; хелперы `addStudent`, `enroll`, `payFor`; `newPaymentSvc`.
- Produces:
  - `PaymentRepository.GetDebts(ctx context.Context, tutorID string) ([]models.CourseDebt, error)`
  - `PaymentService.GetDebts(ctx context.Context, tutorID string) ([]models.StudentDebt, error)`
  - маршрут `GET /payments/debts`

- [ ] **Step 1: Модели**

Append to `models/payment.go`:

```go
// CourseDebt — долг ученика по одному курсу, строка выборки долгов. Наружу не
// отдаётся: сервис складывает строки в StudentDebt.
type CourseDebt struct {
	StudentID    string
	StudentName  string
	CourseID     string
	Subject      string
	LessonsOwed  int
	LessonPrice  float64 // пакет / N, округлено до тенге
	NextLessonAt *time.Time
}

// DebtByCourse — разбивка долга по предмету.
type DebtByCourse struct {
	CourseID    string  `json:"course_id"`
	Subject     string  `json:"subject"`
	LessonsOwed int     `json:"lessons_owed"`
	AmountOwed  float64 `json:"amount_owed"`
}

// StudentDebt — должник: долг по всем его курсам одной строкой (спека, п. 6.5).
type StudentDebt struct {
	StudentID    string         `json:"student_id"`
	StudentName  string         `json:"student_name"`
	LessonsOwed  int            `json:"lessons_owed"`
	AmountOwed   float64        `json:"amount_owed"`
	NextLessonAt *time.Time     `json:"next_lesson_at"` // когда напомнить
	Courses      []DebtByCourse `json:"courses"`
}
```

- [ ] **Step 2: Падающий интеграционный тест**

Create `repository/debts_integration_test.go`:

```go
//go:build integration

package repository_test

import (
	"context"
	"testing"

	"tutorgo/repository"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Долги по всем курсам репетитора, включая архивные, и по всем ученикам,
// включая ушедших (спека 2026-09-06-price-units…, п. 6.5). Запуск:
// make test-integration.

// Ученик с двумя долгами — две строки с ценой урока своего курса; архивный курс
// долг не прощает; урок группы после ухода не сгорает; погасивший в выборку не
// попадает.
func TestGetDebts_ArchivedCourseLeftGroupAndPaidOff(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	tutorID, s := seedTutorStudent(t, pool)

	math := addIndividualCourse(t, pool, tutorID, s) // 40000 за 8 → 5000 за урок
	addLessonAt(t, pool, math, "NOW() - interval '2 days'", "completed")
	addLessonAt(t, pool, math, "NOW() - interval '1 day'", "missed")
	_, err := pool.Exec(ctx, `UPDATE courses SET is_active = FALSE WHERE id = $1`, math)
	require.NoError(t, err)

	groupID := addGroupCourse(t, pool, tutorID, "Группа") // 5000 за урок
	enroll(t, pool, groupID, s, "NOW() - interval '1 month'")
	addLessonAt(t, pool, groupID, "NOW() - interval '3 days'", "completed")
	_, err = pool.Exec(ctx,
		`UPDATE course_enrollments SET left_at = NOW() - interval '2 days'
		 WHERE course_id = $1 AND student_id = $2`, groupID, s)
	require.NoError(t, err)
	addLessonAt(t, pool, groupID, "NOW() - interval '1 day'", "completed")

	paidOff := addStudent(t, pool, tutorID, "Погасил")
	enroll(t, pool, groupID, paidOff, "NOW() - interval '1 month'")
	payFor(t, pool, groupID, paidOff, 4)

	debts, err := repository.NewPaymentRepository(pool).GetDebts(ctx, tutorID)
	require.NoError(t, err)
	require.Len(t, debts, 2)

	assert.Equal(t, s, debts[0].StudentID)
	assert.Equal(t, "Иван Петров", debts[0].StudentName)
	assert.Equal(t, "Группа", debts[0].Subject)
	assert.Equal(t, 1, debts[0].LessonsOwed)
	assert.Equal(t, 5000.0, debts[0].LessonPrice)

	assert.Equal(t, "Математика", debts[1].Subject)
	assert.Equal(t, 2, debts[1].LessonsOwed)
	assert.Equal(t, 5000.0, debts[1].LessonPrice)
}
```

Run: `go vet -tags=integration ./repository/`
Expected: FAIL — `GetDebts undefined`.

- [ ] **Step 3: Репозиторий**

Add `GetDebts(ctx context.Context, tutorID string) ([]models.CourseDebt, error)` to `PaymentRepository` and:

```go
// GetDebts — пары «курс + ученик» репетитора, где сгорело больше, чем оплачено
// (спека, п. 6.5). Курсы — все, включая архивные; ученики — все, включая
// ушедших из группы и архивных: архивация закрывает расписание, но не прощает
// долг. Расти долг ушедшего не может — сгорание обрезано по left_at.
// Погасивший уходит из выборки сам: остаток перестаёт быть положительным.
//
// Цена урока — пакет / N до тенге. У пакетных тьюторов сумма поэтому
// приблизительна (2 урока из 85 000/12 — 14 167 ₸, которых никто не назначал),
// долг в уроках точен всегда.
func (r *paymentRepository) GetDebts(ctx context.Context, tutorID string) ([]models.CourseDebt, error) {
	rows, err := r.conn.Query(ctx,
		`WITH sc AS (
		     SELECT pr.course_id, pr.student_id, pr.enrolled_at, pr.left_at,
		            c.subject, c.price_per_cycle, c.lessons_per_cycle
		       FROM (`+studentCoursePairs+`) pr
		       JOIN courses c ON c.id = pr.course_id
		      WHERE c.tutor_id = $1
		 ),
		 owed AS (
		     SELECT sc.student_id, sc.course_id, sc.subject,
		            sc.price_per_cycle, sc.lessons_per_cycle,
		            (SELECT count(*) FROM lessons l
		              WHERE l.course_id = sc.course_id
		                AND l.status IN ('completed', 'missed')
		                AND `+lessonInParticipation+`)
		          - COALESCE((SELECT SUM(p.lessons_count) FROM payments p
		                       WHERE p.course_id = sc.course_id
		                         AND p.student_id = sc.student_id), 0) AS lessons_owed,
		            (SELECT min(l.scheduled_at) FROM lessons l
		              WHERE l.course_id = sc.course_id
		                AND l.status = 'scheduled'
		                AND l.scheduled_at > NOW()
		                AND `+lessonInParticipation+`) AS next_lesson_at
		       FROM sc
		 )
		 SELECT o.student_id, TRIM(s.first_name || ' ' || COALESCE(s.last_name, '')),
		        o.course_id, o.subject, o.lessons_owed::int,
		        ROUND(o.price_per_cycle / o.lessons_per_cycle), o.next_lesson_at
		   FROM owed o
		   JOIN students s ON s.id = o.student_id
		  WHERE o.lessons_owed > 0
		  ORDER BY o.student_id, o.subject`,
		tutorID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	debts := []models.CourseDebt{}
	for rows.Next() {
		var d models.CourseDebt
		if err := rows.Scan(&d.StudentID, &d.StudentName, &d.CourseID, &d.Subject,
			&d.LessonsOwed, &d.LessonPrice, &d.NextLessonAt); err != nil {
			return nil, err
		}
		debts = append(debts, d)
	}
	return debts, rows.Err()
}
```

Run: `TEST_DB_URL="postgres://dev:dev@localhost:5432/tutorgo_dev?sslmode=disable" go test -tags=integration -count=1 ./repository/ -run TestGetDebts -v`
Expected: PASS.

- [ ] **Step 4: Падающие unit-тесты сервиса и хендлера**

`service/payment_test.go` — mock and test:

```go
func (m *mockPaymentRepo) GetDebts(ctx context.Context, tutorID string) ([]models.CourseDebt, error) {
	args := m.Called(ctx, tutorID)
	return args.Get(0).([]models.CourseDebt), args.Error(1)
}

// Строки курсов складываются в строку на ученика: сумма — уроки × цена урока
// своего курса, напоминание — к ближайшему уроку, порядок — по сумме долга.
func TestPaymentGetDebts_GroupsByStudentAndSortsByAmount(t *testing.T) {
	payRepo := new(mockPaymentRepo)
	svc := newPaymentSvc(payRepo, new(mockCourseRepo), new(mockEnrollmentRepo))

	soon := time.Date(2026, time.September, 20, 10, 0, 0, 0, time.UTC)
	later := soon.Add(48 * time.Hour)
	payRepo.On("GetDebts", mock.Anything, tutorID).Return([]models.CourseDebt{
		{StudentID: "s1", StudentName: "Айгерим", CourseID: "math", Subject: "Математика", LessonsOwed: 2, LessonPrice: 5000, NextLessonAt: &later},
		{StudentID: "s1", StudentName: "Айгерим", CourseID: "phys", Subject: "Физика", LessonsOwed: 1, LessonPrice: 6000, NextLessonAt: &soon},
		{StudentID: "s2", StudentName: "Бекзат", CourseID: "eng", Subject: "Английский", LessonsOwed: 5, LessonPrice: 7083},
	}, nil)

	debts, err := svc.GetDebts(context.Background(), tutorID)

	assert.NoError(t, err)
	if assert.Len(t, debts, 2) {
		assert.Equal(t, "s2", debts[0].StudentID) // 35 415 ₸ больше 16 000 ₸
		assert.Equal(t, 35415.0, debts[0].AmountOwed)
		assert.Nil(t, debts[0].NextLessonAt)

		assert.Equal(t, "s1", debts[1].StudentID)
		assert.Equal(t, 3, debts[1].LessonsOwed)
		assert.Equal(t, 16000.0, debts[1].AmountOwed)
		assert.Equal(t, soon, *debts[1].NextLessonAt)
		assert.Equal(t, []models.DebtByCourse{
			{CourseID: "math", Subject: "Математика", LessonsOwed: 2, AmountOwed: 10000},
			{CourseID: "phys", Subject: "Физика", LessonsOwed: 1, AmountOwed: 6000},
		}, debts[1].Courses)
	}
}

// Никто не должен — пустой список, а не null: фронт рисует пустое состояние.
func TestPaymentGetDebts_EmptyIsNotNil(t *testing.T) {
	payRepo := new(mockPaymentRepo)
	svc := newPaymentSvc(payRepo, new(mockCourseRepo), new(mockEnrollmentRepo))
	payRepo.On("GetDebts", mock.Anything, tutorID).Return([]models.CourseDebt{}, nil)

	debts, err := svc.GetDebts(context.Background(), tutorID)

	assert.NoError(t, err)
	assert.NotNil(t, debts)
	assert.Empty(t, debts)
}
```

`handlers/mocks_test.go`:
```go
func (m *mockPaymentService) GetDebts(ctx context.Context, tutorID string) ([]models.StudentDebt, error) {
	args := m.Called(ctx, tutorID)
	return args.Get(0).([]models.StudentDebt), args.Error(1)
}
```

`handlers/payment_test.go`: register `r.GET("/payments/debts", h.GetDebts)` in `newPaymentRouter` and add:

```go
func TestPaymentGetDebts_Success(t *testing.T) {
	svc := new(mockPaymentService)
	r := newPaymentRouter(svc, testTutorID)

	expected := []models.StudentDebt{{StudentID: testStudentID, StudentName: "Иван", LessonsOwed: 2, AmountOwed: 10000,
		Courses: []models.DebtByCourse{{CourseID: testCourseID, Subject: "Математика", LessonsOwed: 2, AmountOwed: 10000}}}}
	svc.On("GetDebts", mock.Anything, testTutorID).Return(expected, nil)

	w := makeRequest(t, r, http.MethodGet, "/payments/debts", nil)

	assert.Equal(t, http.StatusOK, w.Code)
	var got []models.StudentDebt
	decodeJSON(t, w, &got)
	assert.Equal(t, expected, got)
	svc.AssertExpectations(t)
}
```

Run: `go vet ./...`
Expected: FAIL — `GetDebts` отсутствует у сервиса/хендлера.

- [ ] **Step 5: Сервис, хендлер, маршрут**

`service/payment.go` — add `GetDebts(ctx context.Context, tutorID string) ([]models.StudentDebt, error)` to the interface, `"sort"` to imports, and:

```go
// GetDebts складывает долги курсов в строку на ученика (спека, п. 6.5): сумма —
// уроки курса × цена урока своего курса, напомнить — к ближайшему уроку.
// Порядок — по сумме долга, при равенстве по урокам и имени, чтобы список не
// прыгал между запросами.
func (s *paymentService) GetDebts(ctx context.Context, tutorID string) ([]models.StudentDebt, error) {
	rows, err := s.repo.GetDebts(ctx, tutorID)
	if err != nil {
		return nil, err
	}

	debts := []models.StudentDebt{}
	index := map[string]int{}
	for _, row := range rows {
		i, ok := index[row.StudentID]
		if !ok {
			i = len(debts)
			index[row.StudentID] = i
			debts = append(debts, models.StudentDebt{
				StudentID:   row.StudentID,
				StudentName: row.StudentName,
				Courses:     []models.DebtByCourse{},
			})
		}
		d := &debts[i]
		amount := float64(row.LessonsOwed) * row.LessonPrice
		d.LessonsOwed += row.LessonsOwed
		d.AmountOwed += amount
		d.Courses = append(d.Courses, models.DebtByCourse{
			CourseID:    row.CourseID,
			Subject:     row.Subject,
			LessonsOwed: row.LessonsOwed,
			AmountOwed:  amount,
		})
		if row.NextLessonAt != nil && (d.NextLessonAt == nil || row.NextLessonAt.Before(*d.NextLessonAt)) {
			d.NextLessonAt = row.NextLessonAt
		}
	}

	sort.SliceStable(debts, func(a, b int) bool {
		if debts[a].AmountOwed != debts[b].AmountOwed {
			return debts[a].AmountOwed > debts[b].AmountOwed
		}
		if debts[a].LessonsOwed != debts[b].LessonsOwed {
			return debts[a].LessonsOwed > debts[b].LessonsOwed
		}
		return debts[a].StudentName < debts[b].StudentName
	})
	return debts, nil
}
```

`handlers/payment.go`:

```go
// GetDebts — «кто мне должен» (спека, п. 6.5).
func (h *PaymentHandler) GetDebts(c *gin.Context) {
	tutorID := c.GetString("tutorID")
	if tutorID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	debts, err := h.service.GetDebts(c.Request.Context(), tutorID)
	if err != nil {
		h.log.Error("Failed to get debts", slog.String("error", err.Error()))
		handleServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, debts)
}
```

`router/router.go`: after `auth.GET("/payments/balance", …)` add `auth.GET("/payments/debts", paymentHandler.GetDebts)`.

- [ ] **Step 6: Всё зелёное и коммит**

Run:
```bash
go build ./... && go vet ./... && go vet -tags=integration ./repository/
make test
TEST_DB_URL="postgres://dev:dev@localhost:5432/tutorgo_dev?sslmode=disable" go test -tags=integration -count=1 ./repository/
```
Expected: PASS, кроме известного красного.

```bash
git add models/payment.go repository/payment.go service/payment.go handlers/payment.go router/router.go service/payment_test.go handlers/mocks_test.go handlers/payment_test.go repository/debts_integration_test.go
git commit -m "feat(payments): GET /payments/debts — долги по ученикам

Co-Authored-By: Claude Opus 5 <noreply@anthropic.com>
Claude-Session: https://claude.ai/code/session_01KLiSrLKjmzZMQgDzgsUdbn"
```

---

### Task 4: Позиция в цикле — по ученику

**Files:**
- Modify: `repository/lesson.go` (`GetRanksForCourses` → `GetRanksForStudent`; `GetCalendar`, `GetAllLessonsForCycles`; новая константа `individualPair`)
- Modify: `repository/student.go` (`ListLessons`)
- Modify: `repository/payment.go` (новый `GetByStudentBatch`)
- Modify: `service/lesson.go` (`enrichLessons`, `GetByPeriod`)
- Modify: `service/student.go` (`ListLessons`)
- Modify: `service/lesson_test.go`, `service/student_test.go`, `service/payment_test.go`
- Create: `repository/cycle_integration_test.go`

**Interfaces:**
- Consumes (Task 2): `studentCoursePairs`, `lessonInParticipation`; хелперы `addStudent`, `enroll`, `addPause`.
- Produces:
  - `LessonRepository.GetRanksForStudent(ctx context.Context, courseID, studentID string) (map[string]int, error)` (заменяет `GetRanksForCourses`, других вызывающих нет)
  - `PaymentRepository.GetByStudentBatch(ctx context.Context, studentID string) (map[string][]models.Payment, error)`
  - `(*lessonService).enrichLessons(ctx context.Context, course models.Course, lessons []models.Lesson) error`

**Правило (спека, п. 3.8, 6.7):** цикл считается только там, где известен ученик. В календаре репетитора, на странице курса и на дашборде («текущие циклы») — только у индивидуальных курсов; у групповых уроков `rank` остаётся `NULL`, и бейдж не рисуется. В кабинете ученика — всегда, по его периодам участия и его платежам. Ранги везде считаются тем же `lessonInParticipation`, что сгорание в балансе.

Поля `rule_id` и `occurrence_date` в выборках `repository/lesson.go` не удалять и запросы целиком не переписывать (спека, п. 6.7) — меняются только перечисленные фрагменты.

- [ ] **Step 1: Падающие интеграционные тесты**

Create `repository/cycle_integration_test.go`:

```go
//go:build integration

package repository_test

import (
	"context"
	"testing"
	"time"

	"tutorgo/models"
	"tutorgo/repository"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Позиция в цикле считается по ученику в тех же периодах участия, что и
// сгорание в балансе (спека 2026-09-06-price-units…, п. 3.8, 6.7). Запуск:
// make test-integration.

// Индивидуальный курс: отменённый и замороженный уроки не ранжируются, ранги
// идут подряд по оставшимся.
func TestGetRanksForStudent_SkipsCancelledAndPaused(t *testing.T) {
	pool := testPool(t)
	tutorID, s := seedTutorStudent(t, pool)
	courseID := addIndividualCourse(t, pool, tutorID, s)
	first := addLessonAt(t, pool, courseID, "NOW() - interval '20 days'", "completed")
	cancelled := addLessonAt(t, pool, courseID, "NOW() - interval '10 days'", "cancelled")
	paused := addLessonAt(t, pool, courseID, "NOW() - interval '3 days'", "completed")
	next := addLessonAt(t, pool, courseID, "NOW() + interval '2 days'", "scheduled")
	addPause(t, pool, s, "CURRENT_DATE - 5", "CURRENT_DATE - 1")

	ranks, err := repository.NewLessonRepository(pool).GetRanksForStudent(context.Background(), courseID, s)
	require.NoError(t, err)

	assert.Equal(t, map[string]int{first: 1, next: 2}, ranks)
	assert.NotContains(t, ranks, cancelled)
	assert.NotContains(t, ranks, paused)
}

// Группа: ранги ученика начинаются с его записи.
func TestGetRanksForStudent_GroupStartsAtEnrollment(t *testing.T) {
	pool := testPool(t)
	tutorID, s := seedTutorStudent(t, pool)
	groupID := addGroupCourse(t, pool, tutorID, "Группа")
	addLessonAt(t, pool, groupID, "NOW() - interval '14 days'", "completed")
	mine := addLessonAt(t, pool, groupID, "NOW() - interval '7 days'", "completed")
	enroll(t, pool, groupID, s, "NOW() - interval '10 days'")

	ranks, err := repository.NewLessonRepository(pool).GetRanksForStudent(context.Background(), groupID, s)
	require.NoError(t, err)
	assert.Equal(t, map[string]int{mine: 1}, ranks)
}

// Календарь репетитора: у группового урока ранга нет, у индивидуального — есть
// (спека, п. 3.8).
func TestGetCalendar_OnlyIndividualLessonsAreRanked(t *testing.T) {
	pool := testPool(t)
	tutorID, s := seedTutorStudent(t, pool)
	courseID := addIndividualCourse(t, pool, tutorID, s)
	groupID := addGroupCourse(t, pool, tutorID, "Группа")
	individual := addLessonAt(t, pool, courseID, "NOW() + interval '1 day'", "scheduled")
	group := addLessonAt(t, pool, groupID, "NOW() + interval '1 day'", "scheduled")

	from := time.Now().Add(-time.Hour).UTC().Format(time.RFC3339)
	to := time.Now().Add(72 * time.Hour).UTC().Format(time.RFC3339)
	lessons, err := repository.NewLessonRepository(pool).GetCalendar(context.Background(), tutorID, from, to)
	require.NoError(t, err)

	byID := map[string]models.CalendarLesson{}
	for _, l := range lessons {
		byID[l.ID] = l
	}
	require.Contains(t, byID, individual)
	require.Contains(t, byID, group)
	require.NotNil(t, byID[individual].Rank)
	assert.Equal(t, 1, *byID[individual].Rank)
	assert.Nil(t, byID[group].Rank)
}

// Кабинет ученика: урок группы до записи виден, но в цикл не входит.
func TestListLessons_RanksWithinParticipation(t *testing.T) {
	pool := testPool(t)
	tutorID, s := seedTutorStudent(t, pool)
	groupID := addGroupCourse(t, pool, tutorID, "Группа")
	before := addLessonAt(t, pool, groupID, "NOW() - interval '14 days'", "completed")
	mine := addLessonAt(t, pool, groupID, "NOW() - interval '7 days'", "completed")
	enroll(t, pool, groupID, s, "NOW() - interval '10 days'")

	lessons, err := repository.NewStudentRepository(pool).ListLessons(context.Background(), s, true)
	require.NoError(t, err)

	byID := map[string]models.CalendarLesson{}
	for _, l := range lessons {
		byID[l.ID] = l
	}
	require.Contains(t, byID, before)
	require.Contains(t, byID, mine)
	assert.Nil(t, byID[before].Rank)
	require.NotNil(t, byID[mine].Rank)
	assert.Equal(t, 1, *byID[mine].Rank)
}

// Платежи ученика по его курсам — без чужих платежей той же группы.
func TestGetByStudentBatch_OnlyOwnPayments(t *testing.T) {
	pool := testPool(t)
	tutorID, s := seedTutorStudent(t, pool)
	other := addStudent(t, pool, tutorID, "Другой")
	groupID := addGroupCourse(t, pool, tutorID, "Группа")
	enroll(t, pool, groupID, s, "NOW() - interval '1 month'")
	enroll(t, pool, groupID, other, "NOW() - interval '1 month'")
	payFor(t, pool, groupID, s, 4)
	payFor(t, pool, groupID, other, 8)

	byCourse, err := repository.NewPaymentRepository(pool).GetByStudentBatch(context.Background(), s)
	require.NoError(t, err)
	require.Len(t, byCourse[groupID], 1)
	assert.Equal(t, 4, byCourse[groupID][0].LessonsCount)
}
```

Run: `go vet -tags=integration ./repository/`
Expected: FAIL — `GetRanksForStudent undefined`, `GetByStudentBatch undefined`.

- [ ] **Step 2: Репозитории**

`repository/lesson.go`:

1. Interface: replace `GetRanksForCourses(ctx context.Context, courseIDs []string) (map[string]map[string]int, error)` with `GetRanksForStudent(ctx context.Context, courseID, studentID string) (map[string]int, error)`.

2. Add after `scanLesson`:

```go
// individualPair подставляет индивидуальному курсу пару «курс + ученик» для
// lessonInParticipation (repository/payment.go): границ записи у него нет,
// паузы ученика действуют. Ожидает алиас c (courses) и фильтр
// c.student_id IS NOT NULL в самом запросе.
const individualPair = `CROSS JOIN LATERAL (SELECT c.student_id,
	                          NULL::timestamptz AS enrolled_at,
	                          NULL::timestamptz AS left_at) sc`
```

3. In `GetCalendar` replace only the `ranked` CTE and its comment:

```go
		// ranked — позиция урока в цикле, считается в БД одним запросом. Только у
		// индивидуальных курсов (спека, п. 3.8): у группового урока нет одного
		// ученика, rank остаётся NULL, и сервис бейдж не рисует. Замороженные уроки
		// не ранжируются — тот же предикат, что у сгорания в балансе (п. 6.7).
		// cal_courses is materialized so the subquery runs once and is reused by ranked.
		`WITH cal_courses AS MATERIALIZED (
		   …unchanged…
		 ),
		 ranked AS (
		   SELECT l.id,
		          ROW_NUMBER() OVER (PARTITION BY l.course_id ORDER BY l.scheduled_at)::int AS rank
		   FROM lessons l
		   JOIN courses c ON c.id = l.course_id
		   `+individualPair+`
		   WHERE l.course_id IN (SELECT course_id FROM cal_courses)
		     AND c.student_id IS NOT NULL
		     AND l.status != 'cancelled'
		     AND `+lessonInParticipation+`
		 )
		 SELECT …unchanged…`
```

4. `GetAllLessonsForCycles` — the query becomes:

```go
	// Текущие циклы дашборда — только индивидуальные курсы (спека, п. 3.8): цикл
	// группы теперь у каждого участника свой, строка на курс его не выразит.
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
		 `+individualPair+`
		 LEFT JOIN students s ON s.id = c.student_id
		 WHERE c.tutor_id = $1
		   AND c.student_id IS NOT NULL
		   AND l.status != 'cancelled'
		   AND `+lessonInParticipation+`
		 ORDER BY l.course_id, l.scheduled_at`,
		tutorID)
```

5. Replace `GetRanksForCourses` with:

```go
// GetRanksForStudent — порядковые номера уроков курса в периодах участия ученика
// (спека, п. 6.7): те же границы, что у сгорания в балансе, иначе бейдж «3 из 8»
// и «оплачено 3 из 8» разойдутся. Отменённые не ранжируются.
func (r *lessonRepository) GetRanksForStudent(ctx context.Context, courseID, studentID string) (map[string]int, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT l.id,
		        ROW_NUMBER() OVER (ORDER BY l.scheduled_at)::int AS rank
		   FROM lessons l
		   JOIN (`+studentCoursePairs+`) sc ON sc.course_id = l.course_id
		  WHERE l.course_id = $1
		    AND sc.student_id = $2
		    AND l.status != 'cancelled'
		    AND `+lessonInParticipation,
		courseID, studentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	ranks := map[string]int{}
	for rows.Next() {
		var lessonID string
		var rank int
		if err := rows.Scan(&lessonID, &rank); err != nil {
			return nil, err
		}
		ranks[lessonID] = rank
	}
	return ranks, rows.Err()
}
```

`repository/student.go` `ListLessons` — replace the two CTEs and the course filter; the rest of the query stays:

```go
	// enrollment-джойн идентичен EnrolledInLesson: индивидуальный курс (c.student_id)
	// ИЛИ групповой через course_enrollments. Фильтр активности курса намеренно
	// опущен — ученик видит все свои уроки, включая архивные (история).
	// rank считается по ВСЕМ неотменённым урокам курса в периодах участия ученика
	// (без date-фильтра, иначе позиция зависела бы от вкладки) — тем же
	// предикатом, что сгорание в балансе (спека, п. 6.7): уроки группы до записи
	// и в заморозке видны, но в цикл не входят.
	// Уроки группы после ухода (left_at) не показываются — та же граница, что
	// в burned фазы 2; прошлые остаются историей (спека, п. 5a.4).
	base := `WITH sc AS MATERIALIZED (
	           SELECT * FROM (` + studentCoursePairs + `) pr WHERE pr.student_id = $1
	         ),
	         ranked AS (
	           SELECT l.id,
	                  ROW_NUMBER() OVER (PARTITION BY l.course_id ORDER BY l.scheduled_at)::int AS rank
	           FROM lessons l
	           JOIN sc ON sc.course_id = l.course_id
	           WHERE l.status != 'cancelled'
	             AND ` + lessonInParticipation + `
	         )
	         SELECT l.id, l.course_id, l.scheduled_at, l.duration_minutes, l.status,
	                l.notes, c.subject, s.first_name, (c.student_id IS NULL) AS is_group,
	                r.rank
	         FROM lessons l
	         JOIN courses c ON c.id = l.course_id
	         LEFT JOIN students s ON s.id = c.student_id
	         LEFT JOIN ranked r ON r.id = l.id
	         LEFT JOIN course_enrollments ce ON ce.course_id = l.course_id AND ce.student_id = $1
	         WHERE l.course_id IN (SELECT course_id FROM sc)
	           AND l.scheduled_at < COALESCE(ce.left_at, 'infinity'::timestamptz)`
```

`repository/payment.go` — interface `GetByStudentBatch(ctx context.Context, studentID string) (map[string][]models.Payment, error)` and:

```go
// GetByStudentBatch — платежи ученика, разложенные по курсам, по возрастанию
// даты: позиция в цикле кабинета считается по его платежам, а не по всем
// платежам группы (спека, п. 6.7).
func (r *paymentRepository) GetByStudentBatch(ctx context.Context, studentID string) (map[string][]models.Payment, error) {
	rows, err := r.conn.Query(ctx,
		`SELECT `+paymentColumns+`
		 FROM payments p
		 WHERE p.student_id = $1
		 ORDER BY p.course_id, p.paid_at ASC`, studentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := map[string][]models.Payment{}
	for rows.Next() {
		var p models.Payment
		if err := rows.Scan(paymentDest(&p)...); err != nil {
			return nil, err
		}
		result[p.CourseID] = append(result[p.CourseID], p)
	}
	return result, rows.Err()
}
```

Run: `TEST_DB_URL="postgres://dev:dev@localhost:5432/tutorgo_dev?sslmode=disable" go test -tags=integration -count=1 ./repository/ -run 'TestGetRanksForStudent|TestGetCalendar_Only|TestListLessons|TestGetByStudentBatch' -v`
Expected: PASS. (`go vet ./...` в этот момент красный из-за моков — это Step 3.)

- [ ] **Step 3: Падающие unit-тесты сервисов**

`service/lesson_test.go`:
- mock: replace `GetRanksForCourses` with
```go
func (m *mockLessonRepo) GetRanksForStudent(ctx context.Context, courseID, studentID string) (map[string]int, error) {
	args := m.Called(ctx, courseID, studentID)
	return args.Get(0).(map[string]int), args.Error(1)
}
```
- `TestLessonGetByPeriod_Success` uses the group course `expectedCourse`: delete its `GetRanksForCourses` and `GetByCoursesBatch` expectations and add `lessonRepo.AssertNotCalled(t, "GetRanksForStudent", mock.Anything, mock.Anything, mock.Anything)` — у группы цикл не считается.
- add:
```go
// Индивидуальный курс: позиция в цикле — по рангам ученика и платежам курса.
func TestLessonGetByPeriod_IndividualCourseGetsCyclePosition(t *testing.T) {
	lessonRepo := new(mockLessonRepo)
	courseRepo := new(mockCourseRepo)
	paymentRepo := new(mockPaymentRepo)
	svc := newLessonSvcWithPayment(lessonRepo, courseRepo, paymentRepo)

	from := "2026-05-19T00:00:00Z"
	to := "2026-05-26T00:00:00Z"
	student := "student-uuid-1"
	course := expectedCourse
	course.StudentID = &student

	courseRepo.On("GetByID", mock.Anything, courseID, tutorID).Return(course, nil)
	lessonRepo.On("GetByPeriod", mock.Anything, courseID, tutorID, from, to).Return([]models.Lesson{expectedLesson}, nil)
	lessonRepo.On("GetRanksForStudent", mock.Anything, courseID, student).Return(map[string]int{expectedLesson.ID: 2}, nil)
	paymentRepo.On("GetByCoursesBatch", mock.Anything, []string{courseID}).Return(map[string][]models.Payment{
		courseID: {{LessonsCount: 8}},
	}, nil)

	lessons, err := svc.GetByPeriod(context.Background(), courseID, tutorID, from, to)

	assert.NoError(t, err)
	if assert.Len(t, lessons, 1) && assert.NotNil(t, lessons[0].CyclePosition) {
		assert.Equal(t, 2, *lessons[0].CyclePosition)
		assert.Equal(t, 8, *lessons[0].CycleSize)
	}
	lessonRepo.AssertExpectations(t)
	paymentRepo.AssertExpectations(t)
}
```
(`expectedLesson` must have a non-empty `ID`; if it is empty, set one on a local copy.)

`service/payment_test.go` — add to `mockPaymentRepo`:
```go
func (m *mockPaymentRepo) GetByStudentBatch(ctx context.Context, studentID string) (map[string][]models.Payment, error) {
	args := m.Called(ctx, studentID)
	return args.Get(0).(map[string][]models.Payment), args.Error(1)
}
```

`service/student_test.go` — in `TestListLessons_PaidFlag` and `TestStudentListLessons_CyclePositions` replace `payRepo.On("GetByCoursesBatch", mock.Anything, []string{"c1"})` with `payRepo.On("GetByStudentBatch", mock.Anything, "stu-1")` (return values unchanged).

Run: `go vet ./... && go test ./service/ -run 'TestLessonGetByPeriod|TestListLessons_PaidFlag|TestStudentListLessons_CyclePositions'`
Expected: FAIL — сервис всё ещё зовёт `GetRanksForCourses` / `GetByCoursesBatch`.

- [ ] **Step 4: Сервисы**

`service/lesson.go` — replace `enrichLessons`:

```go
// enrichLessons проставляет позицию в цикле — только индивидуальному курсу
// (спека, п. 3.8): у группового урока нет одного ученика, а цикл считается по
// платежам конкретного человека. Для группы он не считается вовсе, вместо того
// чтобы считаться неправильно. Платежи индивидуального курса все адресованы его
// ученику (спека, п. 6.1–6.3), поэтому выборка по курсу здесь точна.
func (s *lessonService) enrichLessons(ctx context.Context, course models.Course, lessons []models.Lesson) error {
	if len(lessons) == 0 || course.StudentID == nil {
		return nil
	}
	ranks, err := s.repo.GetRanksForStudent(ctx, course.ID, *course.StudentID)
	if err != nil {
		return err
	}
	paymentsMap, err := s.paymentRepo.GetByCoursesBatch(ctx, []string{course.ID})
	if err != nil {
		return err
	}
	coursePayments := paymentsMap[course.ID]
	if len(coursePayments) == 0 {
		return nil
	}
	infos := computeCyclePositions(ranks, coursePayments)
	for i, l := range lessons {
		if info, ok := infos[l.ID]; ok {
			pos, size := info.Position, info.Size
			lessons[i].CyclePosition = &pos
			lessons[i].CycleSize = &size
		}
	}
	return nil
}
```

`GetByPeriod`: keep the course — `course, err := s.courseRepo.GetByID(ctx, courseID, tutorID)` — and call `s.enrichLessons(ctx, course, lessons)`.

`service/student.go` `ListLessons` — replace the block from `// Циклы считаются от платежей` up to the `GetByCoursesBatch` error check:

```go
	// Циклы считаются от платежей самого ученика: у группы платежи адресные, и
	// чужие пакеты сдвинули бы его позицию (спека, п. 6.7). Ранги уже в периодах
	// участия — их считает репозиторий.
	hasRank := false
	for _, l := range lessons {
		if l.Rank != nil {
			hasRank = true
			break
		}
	}
	if !hasRank {
		return lessons, nil
	}
	paymentsMap, err := s.paymentRepo.GetByStudentBatch(ctx, studentID)
	if err != nil {
		return nil, err
	}
```
The loop below it stays unchanged.

- [ ] **Step 5: Всё зелёное и коммит**

Run:
```bash
go build ./... && go vet ./... && go vet -tags=integration ./repository/
make test
TEST_DB_URL="postgres://dev:dev@localhost:5432/tutorgo_dev?sslmode=disable" go test -tags=integration -count=1 ./repository/
```
Expected: PASS, кроме известного красного.

```bash
git add repository/lesson.go repository/student.go repository/payment.go service/lesson.go service/student.go service/lesson_test.go service/student_test.go service/payment_test.go repository/cycle_integration_test.go
git commit -m "feat(cycles): позиция в цикле по ученику, у групп в календаре не считается

Co-Authored-By: Claude Opus 5 <noreply@anthropic.com>
Claude-Session: https://claude.ai/code/session_01KLiSrLKjmzZMQgDzgsUdbn"
```

---

### Task 5: Одна оплата на несколько предметов — `POST /payments/bulk`

**Files:**
- Modify: `models/payment.go` (`BulkPaymentItem`, `CreateBulkPaymentRequest`)
- Modify: `repository/payment.go` (`CreateBulk`)
- Modify: `service/payment.go` (`CreateBulk`)
- Modify: `handlers/payment.go` (`CreateBulk`), `router/router.go`
- Modify: `service/payment_test.go`, `handlers/mocks_test.go`, `handlers/payment_test.go`
- Create: `repository/bulk_payment_integration_test.go`

**Interfaces:**
- Consumes (Task 1): `paymentColumns`, `paymentDest`, `studentOnCourse`, `individualCourse`, `payStudentID`, `newPaymentSvc`; хелперы `addStudent`.
- Produces:
  - `PaymentRepository.CreateBulk(ctx context.Context, req models.CreateBulkPaymentRequest) ([]models.Payment, error)`
  - `PaymentService.CreateBulk(ctx context.Context, req models.CreateBulkPaymentRequest, tutorID string) ([]models.Payment, error)`
  - маршрут `POST /payments/bulk` → `201 []Payment`

**Правило (спека, п. 6.8):** истина — строки платежей; поле «К распределению» на фронте нигде не хранится. Схема не меняется, `receipt_id` не заводим. Все строки — одним `INSERT`: оператор атомарен, ошибка на второй строке не оставляет первую в базе. Каждая строка проверяется как одиночный платёж (владение курсом + ученик на курсе) до записи.

- [ ] **Step 1: Модели**

Append to `models/payment.go`:

```go
// BulkPaymentItem — строка оплаты на несколько предметов: те же поля и та же
// проверка, что у одиночного платежа, дата — общая на все строки.
type BulkPaymentItem struct {
	CourseID     string  `json:"course_id"     validate:"required,uuid"`
	StudentID    string  `json:"student_id"    validate:"required,uuid"`
	Amount       float64 `json:"amount"        validate:"required,gt=0"`
	LessonsCount int     `json:"lessons_count" validate:"required,gt=0"`
}

// CreateBulkPaymentRequest — одна оплата, разложенная по предметам (спека,
// п. 6.8). Сумма «к распределению» сюда не входит намеренно: доход месяца
// складывается из строк, второй источник правды о деньгах не нужен.
type CreateBulkPaymentRequest struct {
	PaidAt time.Time         `json:"paid_at" validate:"required"`
	Items  []BulkPaymentItem `json:"items"   validate:"required,min=1,max=20,dive"`
}
```

- [ ] **Step 2: Падающий интеграционный тест**

Create `repository/bulk_payment_integration_test.go`:

```go
//go:build integration

package repository_test

import (
	"context"
	"testing"
	"time"

	"tutorgo/models"
	"tutorgo/repository"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Оплата на несколько предметов пишется целиком или никак (спека
// 2026-09-06-price-units…, п. 6.8). Запуск: make test-integration.
func TestCreateBulk_AllOrNothing(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	tutorID, s := seedTutorStudent(t, pool)
	math := addIndividualCourse(t, pool, tutorID, s)
	var physics string
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO courses (student_id, tutor_id, subject, price_per_cycle, lessons_per_cycle, started_at)
		 VALUES ($1, $2, 'Физика', 6000, 1, NOW() - interval '1 month') RETURNING id`,
		s, tutorID).Scan(&physics))

	repo := repository.NewPaymentRepository(pool)
	paidAt := time.Date(2026, time.September, 14, 0, 0, 0, 0, time.UTC)
	countPayments := func() int {
		var n int
		require.NoError(t, pool.QueryRow(ctx,
			`SELECT count(*) FROM payments WHERE course_id = ANY($1::uuid[])`,
			[]string{math, physics}).Scan(&n))
		return n
	}

	// Вторая строка ссылается на несуществующего ученика — внешний ключ валит
	// оператор, и первая строка не должна остаться в базе.
	_, err := repo.CreateBulk(ctx, models.CreateBulkPaymentRequest{PaidAt: paidAt, Items: []models.BulkPaymentItem{
		{CourseID: math, StudentID: s, Amount: 40000, LessonsCount: 8},
		{CourseID: physics, StudentID: uuid.NewString(), Amount: 24000, LessonsCount: 4},
	}})
	require.Error(t, err)
	assert.Equal(t, 0, countPayments())

	payments, err := repo.CreateBulk(ctx, models.CreateBulkPaymentRequest{PaidAt: paidAt, Items: []models.BulkPaymentItem{
		{CourseID: math, StudentID: s, Amount: 40000, LessonsCount: 8},
		{CourseID: physics, StudentID: s, Amount: 24000, LessonsCount: 4},
	}})
	require.NoError(t, err)
	require.Len(t, payments, 2)
	assert.Equal(t, 2, countPayments())
	for _, p := range payments {
		require.NotNil(t, p.StudentID)
		assert.Equal(t, s, *p.StudentID)
		assert.True(t, paidAt.Equal(p.PaidAt))
	}
}
```

Run: `go vet -tags=integration ./repository/`
Expected: FAIL — `CreateBulk undefined`.

- [ ] **Step 3: Репозиторий**

Add `CreateBulk(ctx context.Context, req models.CreateBulkPaymentRequest) ([]models.Payment, error)` to `PaymentRepository` and:

```go
// CreateBulk записывает строки одной оплаты одним INSERT (спека, п. 6.8):
// оператор атомарен, и ошибка на второй строке не оставит первую в базе. Два
// последовательных INSERT могли бы записать половину денег — тьютор увидел бы
// один платёж вместо двух, не поняв почему.
func (r *paymentRepository) CreateBulk(ctx context.Context, req models.CreateBulkPaymentRequest) ([]models.Payment, error) {
	courseIDs := make([]string, len(req.Items))
	studentIDs := make([]string, len(req.Items))
	amounts := make([]float64, len(req.Items))
	lessons := make([]int, len(req.Items))
	for i, item := range req.Items {
		courseIDs[i], studentIDs[i] = item.CourseID, item.StudentID
		amounts[i], lessons[i] = item.Amount, item.LessonsCount
	}

	rows, err := r.conn.Query(ctx,
		`INSERT INTO payments AS p (course_id, student_id, amount, lessons_count, paid_at)
		 SELECT u.course_id, u.student_id, u.amount, u.lessons_count, $5
		   FROM unnest($1::uuid[], $2::uuid[], $3::numeric[], $4::int[])
		        AS u(course_id, student_id, amount, lessons_count)
		 RETURNING `+paymentColumns,
		courseIDs, studentIDs, amounts, lessons, req.PaidAt)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	payments := []models.Payment{}
	for rows.Next() {
		var p models.Payment
		if err := rows.Scan(paymentDest(&p)...); err != nil {
			return nil, err
		}
		payments = append(payments, p)
	}
	// Ошибка оператора (внешний ключ, CHECK) приходит сюда, а не из Query.
	return payments, rows.Err()
}
```

Run: `TEST_DB_URL="postgres://dev:dev@localhost:5432/tutorgo_dev?sslmode=disable" go test -tags=integration -count=1 ./repository/ -run TestCreateBulk -v`
Expected: PASS.

- [ ] **Step 4: Падающие unit-тесты сервиса и хендлера**

`service/payment_test.go`:

```go
func (m *mockPaymentRepo) CreateBulk(ctx context.Context, req models.CreateBulkPaymentRequest) ([]models.Payment, error) {
	args := m.Called(ctx, req)
	return args.Get(0).([]models.Payment), args.Error(1)
}

func bulkReq(items ...models.BulkPaymentItem) models.CreateBulkPaymentRequest {
	return models.CreateBulkPaymentRequest{PaidAt: paymentReq.PaidAt, Items: items}
}

// Ученик не на курсе во второй строке — 400 до записи: ничего не пишется
// (спека, п. 6.8).
func TestPaymentCreateBulk_SecondItemRejectedNothingWritten(t *testing.T) {
	payRepo := new(mockPaymentRepo)
	courseRepo := new(mockCourseRepo)
	svc := newPaymentSvc(payRepo, courseRepo, new(mockEnrollmentRepo))

	other := "student-uuid-2"
	foreign := models.Course{ID: "course-uuid-2", TutorID: tutorID, StudentID: &other, IsActive: true}
	courseRepo.On("GetByID", mock.Anything, courseID, tutorID).Return(individualCourse, nil)
	courseRepo.On("GetByID", mock.Anything, "course-uuid-2", tutorID).Return(foreign, nil)

	_, err := svc.CreateBulk(context.Background(), bulkReq(
		models.BulkPaymentItem{CourseID: courseID, StudentID: payStudentID, Amount: 40000, LessonsCount: 8},
		models.BulkPaymentItem{CourseID: "course-uuid-2", StudentID: payStudentID, Amount: 6000, LessonsCount: 1},
	), tutorID)

	assert.ErrorIs(t, err, service.ErrBadRequest)
	payRepo.AssertNotCalled(t, "CreateBulk", mock.Anything, mock.Anything)
}

func TestPaymentCreateBulk_ChecksEveryItemThenWritesOnce(t *testing.T) {
	payRepo := new(mockPaymentRepo)
	courseRepo := new(mockCourseRepo)
	enrollRepo := new(mockEnrollmentRepo)
	svc := newPaymentSvc(payRepo, courseRepo, enrollRepo)

	group := models.Course{ID: "group-uuid", TutorID: tutorID, IsActive: true}
	req := bulkReq(
		models.BulkPaymentItem{CourseID: courseID, StudentID: payStudentID, Amount: 40000, LessonsCount: 8},
		models.BulkPaymentItem{CourseID: "group-uuid", StudentID: payStudentID, Amount: 20000, LessonsCount: 4},
	)
	courseRepo.On("GetByID", mock.Anything, courseID, tutorID).Return(individualCourse, nil)
	courseRepo.On("GetByID", mock.Anything, "group-uuid", tutorID).Return(group, nil)
	enrollRepo.On("IsEnrolled", mock.Anything, "group-uuid", payStudentID).Return(true, nil)
	payRepo.On("CreateBulk", mock.Anything, req).Return([]models.Payment{{ID: "p1"}, {ID: "p2"}}, nil)

	payments, err := svc.CreateBulk(context.Background(), req, tutorID)

	assert.NoError(t, err)
	assert.Len(t, payments, 2)
	payRepo.AssertNumberOfCalls(t, "CreateBulk", 1)
	enrollRepo.AssertExpectations(t)
}
```

`handlers/mocks_test.go`:
```go
func (m *mockPaymentService) CreateBulk(ctx context.Context, req models.CreateBulkPaymentRequest, tutorID string) ([]models.Payment, error) {
	args := m.Called(ctx, req, tutorID)
	return args.Get(0).([]models.Payment), args.Error(1)
}
```

`handlers/payment_test.go`: register `r.POST("/payments/bulk", h.CreateBulk)` in `newPaymentRouter` and add:

```go
func TestPaymentCreateBulk_Created(t *testing.T) {
	svc := new(mockPaymentService)
	r := newPaymentRouter(svc, testTutorID)

	req := models.CreateBulkPaymentRequest{
		PaidAt: time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC),
		Items: []models.BulkPaymentItem{
			{CourseID: testCourseID, StudentID: testStudentID, Amount: 40000, LessonsCount: 8},
		},
	}
	svc.On("CreateBulk", mock.Anything, req, testTutorID).Return([]models.Payment{testPayment}, nil)

	w := makeRequest(t, r, http.MethodPost, "/payments/bulk", req)

	assert.Equal(t, http.StatusCreated, w.Code)
	svc.AssertExpectations(t)
}

func TestPaymentCreateBulk_EmptyItemsRejected(t *testing.T) {
	svc := new(mockPaymentService)
	r := newPaymentRouter(svc, testTutorID)

	w := makeRequest(t, r, http.MethodPost, "/payments/bulk",
		models.CreateBulkPaymentRequest{PaidAt: time.Now(), Items: []models.BulkPaymentItem{}})

	assert.Equal(t, http.StatusBadRequest, w.Code)
	svc.AssertNotCalled(t, "CreateBulk")
}
```
(Check how `makeRequest` takes a body in this file — pass the struct or its JSON the same way `TestPaymentCreate_*` does.)

Run: `go vet ./...`
Expected: FAIL — `CreateBulk` отсутствует у сервиса/хендлера.

- [ ] **Step 5: Сервис, хендлер, маршрут**

`service/payment.go` — add `CreateBulk(ctx context.Context, req models.CreateBulkPaymentRequest, tutorID string) ([]models.Payment, error)` to the interface and:

```go
// CreateBulk — одна оплата на несколько предметов (спека, п. 6.8). Каждая
// строка проверяется как одиночный платёж и до записи: сама запись — один
// атомарный оператор, откатывать нечего.
func (s *paymentService) CreateBulk(ctx context.Context, req models.CreateBulkPaymentRequest, tutorID string) ([]models.Payment, error) {
	courses := map[string]models.Course{}
	for i, item := range req.Items {
		course, ok := courses[item.CourseID]
		if !ok {
			found, err := s.courseRepo.GetByID(ctx, item.CourseID, tutorID)
			if err != nil {
				return nil, fmt.Errorf("items[%d] course: %w", i, ErrNotFound)
			}
			course = found
			courses[item.CourseID] = found
		}
		if err := s.studentOnCourse(ctx, course, item.StudentID); err != nil {
			return nil, fmt.Errorf("items[%d]: %w", i, err)
		}
	}
	payments, err := s.repo.CreateBulk(ctx, req)
	if err != nil {
		return nil, err
	}
	globalCalendarCache.Invalidate(tutorID)
	return payments, nil
}
```

`handlers/payment.go`:

```go
// CreateBulk — оплата на несколько предметов одним сабмитом (спека, п. 6.8).
func (h *PaymentHandler) CreateBulk(c *gin.Context) {
	tutorID := c.GetString("tutorID")
	if tutorID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	var req models.CreateBulkPaymentRequest
	if !bindAndValidate(c, &req) {
		return
	}
	payments, err := h.service.CreateBulk(c.Request.Context(), req, tutorID)
	if err != nil {
		h.log.Error("Failed to create bulk payment", slog.String("error", err.Error()))
		handleServiceError(c, err)
		return
	}
	h.log.Info("Bulk payment created", slog.Int("items", len(payments)))
	c.JSON(http.StatusCreated, payments)
}
```

`router/router.go`: after `auth.POST("/payments", paymentHandler.Create)` add `auth.POST("/payments/bulk", paymentHandler.CreateBulk)`.

- [ ] **Step 6: Всё зелёное и коммит**

Run:
```bash
go build ./... && go vet ./... && go vet -tags=integration ./repository/
make test
TEST_DB_URL="postgres://dev:dev@localhost:5432/tutorgo_dev?sslmode=disable" go test -tags=integration -count=1 ./repository/
```
Expected: PASS, кроме известного красного.

```bash
git add models/payment.go repository/payment.go service/payment.go handlers/payment.go router/router.go service/payment_test.go handlers/mocks_test.go handlers/payment_test.go repository/bulk_payment_integration_test.go
git commit -m "feat(payments): POST /payments/bulk — одна оплата на несколько предметов

Co-Authored-By: Claude Opus 5 <noreply@anthropic.com>
Claude-Session: https://claude.ai/code/session_01KLiSrLKjmzZMQgDzgsUdbn"
```

---

### Task 6: Заморозка ученика — `student_pauses`

**Files:**
- Create: `models/pause.go`
- Create: `repository/pause.go`
- Create: `service/pause.go`
- Create: `handlers/pause.go`
- Modify: `router/router.go`
- Create: `service/pause_test.go`
- Create: `repository/pause_integration_test.go`

**Interfaces:**
- Consumes: таблица `student_pauses` (Task 1); `lessonInParticipation` уже исключает паузы из сгорания, прогноза и циклов (Tasks 2, 4) — денежной части здесь нет; `RecurrenceService.Materialize(ctx, ruleID string, horizon time.Time) (int, error)`, `service.RecurrenceHorizon`; хелперы `seedSeries`, `addStudent`, `enroll`.
- Produces:
  - `models.StudentPause`, `models.CreatePauseRequest`
  - `repository.PauseRepository` — `Create(ctx, studentID string, req models.CreatePauseRequest) (models.StudentPause, []string, error)` (второе значение — id правил со сдвинутым хвостом), `ListByStudent(ctx, studentID string) ([]models.StudentPause, error)`, `Delete(ctx, id, studentID string) (bool, error)`
  - `service.PauseService` — `Create(ctx, studentID, tutorID string, req models.CreatePauseRequest) (models.StudentPause, error)`, `List(ctx, studentID, tutorID string) ([]models.StudentPause, error)`, `Delete(ctx, id, studentID, tutorID string) error`
  - маршруты `GET/POST /students/:id/pauses`, `DELETE /students/:id/pauses/:pauseId`

**Правила (спека, п. 6.9):**
- Пауза висит на ученике, без курса. Денег паузы касаются только через `lessonInParticipation`.
- Расписание меняется **только у индивидуальных курсов** ученика: уроки `scheduled` в интервале → `cancelled` (иначе автозавершение закроет их как `completed`); хвост каждого затронутого правила сдвигается: `ends_on += D` (D — дней в паузе), `max_count += N` (N — отменено вхождений этого правила); бессрочное (`NULL`/`NULL`) не меняется. Групповые уроки и правила групп не трогаются никогда.
- Всё это — **один SQL-оператор** с data-modifying CTE: частичного состояния «пауза есть, уроки не отменены» или «конец сдвинут, счётчик нет» не бывает.
- Сдвинутому правилу `materialized_until` откатывается до начала паузы: иначе хвост за старым концом не материализуется никогда — `Materialize` идёт от `materialized_until`, а тот уже дальше старого конца. Дублей не будет: `(rule_id, occurrence_date)` уникален, отменённые вхождения держатся тумбстоунами. После оператора сервис сразу материализует сдвинутые правила; сбой не страшен — откат `materialized_until` возвращает правило в выборку ночной `ExtendAll`.
- Разморозка — удаление строки паузы: уроки возвращаются в счёт, отменённые остаются отменёнными, сдвинутый хвост назад не откатывается.

- [ ] **Step 1: Модели**

Create `models/pause.go`:

```go
package models

import "time"

// StudentPause — заморозка ученика с даты по дату включительно (спека, п. 6.9).
// Висит на ученике, а не на курсе: «уехал на месяц» закрывает все его предметы
// и группы одним действием.
type StudentPause struct {
	ID        string    `json:"id"`
	StudentID string    `json:"student_id"`
	StartsOn  time.Time `json:"starts_on"`
	EndsOn    time.Time `json:"ends_on"`
	Reason    *string   `json:"reason"`
	CreatedAt time.Time `json:"created_at"`
}

// CreatePauseRequest — даты приходят полночью UTC, как paid_at у платежа.
// Порядок дат проверяет сервис: ошибка «конец раньше начала» понятнее, чем
// нарушение CHECK из базы.
type CreatePauseRequest struct {
	StartsOn time.Time `json:"starts_on" validate:"required"`
	EndsOn   time.Time `json:"ends_on"   validate:"required"`
	Reason   *string   `json:"reason"    validate:"omitempty,max=500"`
}
```

- [ ] **Step 2: Падающие интеграционные тесты**

Create `repository/pause_integration_test.go`:

```go
//go:build integration

package repository_test

import (
	"context"
	"testing"
	"time"

	"tutorgo/models"
	"tutorgo/repository"
	"tutorgo/service"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Заморозка отменяет уроки индивидуальных курсов в интервале и сдвигает хвост
// их правил; расписание групп не трогает (спека 2026-09-06-price-units…,
// п. 6.9). Запуск: make test-integration.

// seedIndividualSeries — seedSeries (6 еженедельных уроков через неделю от
// сегодня), чей курс отдан новому ученику: заморозка меняет расписание только
// индивидуальных курсов.
func seedIndividualSeries(t *testing.T, pool *pgxpool.Pool) (studentID, courseID, ruleID string, first time.Time) {
	base := time.Now().UTC().AddDate(0, 0, 7)
	first = time.Date(base.Year(), base.Month(), base.Day(), 12, 0, 0, 0, time.UTC)
	tutorID, courseID, ruleID := seedSeries(t, pool, first, 6)
	studentID = addStudent(t, pool, tutorID, "Уехал")
	_, err := pool.Exec(context.Background(),
		`UPDATE courses SET student_id = $1 WHERE id = $2`, studentID, courseID)
	require.NoError(t, err)
	return studentID, courseID, ruleID, first
}

func dateOnly(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}

func countLessons(t *testing.T, pool *pgxpool.Pool, courseID, status string) int {
	var n int
	require.NoError(t, pool.QueryRow(context.Background(),
		`SELECT count(*) FROM lessons WHERE course_id = $1 AND status = $2`, courseID, status).Scan(&n))
	return n
}

// Пауза на вторую и третью недели: два урока отменены, конец правила уехал на
// 14 дней, после материализации запланированных снова шесть.
func TestPauseCreate_CancelsLessonsAndShiftsEndsOn(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	studentID, courseID, ruleID, first := seedIndividualSeries(t, pool)

	var oldEnds time.Time
	require.NoError(t, pool.QueryRow(ctx, `SELECT ends_on FROM recurrence_rules WHERE id = $1`, ruleID).Scan(&oldEnds))

	starts, ends := dateOnly(first.AddDate(0, 0, 7)), dateOnly(first.AddDate(0, 0, 20))
	pause, shifted, err := repository.NewPauseRepository(pool).Create(ctx, studentID,
		models.CreatePauseRequest{StartsOn: starts, EndsOn: ends})
	require.NoError(t, err)
	assert.Equal(t, studentID, pause.StudentID)
	assert.Equal(t, []string{ruleID}, shifted)
	assert.Equal(t, 2, countLessons(t, pool, courseID, "cancelled"))

	var newEnds, materializedUntil time.Time
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT ends_on, materialized_until FROM recurrence_rules WHERE id = $1`, ruleID).
		Scan(&newEnds, &materializedUntil))
	assert.Equal(t, oldEnds.AddDate(0, 0, 14).Format(time.DateOnly), newEnds.Format(time.DateOnly))
	assert.Equal(t, starts.Format(time.DateOnly), materializedUntil.Format(time.DateOnly))

	_, err = service.NewRecurrenceService(repository.NewRecurrenceRepository(pool)).
		Materialize(ctx, ruleID, first.AddDate(0, 0, 7*10))
	require.NoError(t, err)
	assert.Equal(t, 6, countLessons(t, pool, courseID, "scheduled"))
	assert.Equal(t, 2, countLessons(t, pool, courseID, "cancelled"))
}

// Правило со счётчиком: max_count растёт ровно на число отменённых вхождений.
func TestPauseCreate_ShiftsMaxCount(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	studentID, courseID, ruleID, first := seedIndividualSeries(t, pool)
	_, err := pool.Exec(ctx, `UPDATE recurrence_rules SET ends_on = NULL, max_count = 6 WHERE id = $1`, ruleID)
	require.NoError(t, err)

	_, shifted, err := repository.NewPauseRepository(pool).Create(ctx, studentID, models.CreatePauseRequest{
		StartsOn: dateOnly(first.AddDate(0, 0, 7)), EndsOn: dateOnly(first.AddDate(0, 0, 20)),
	})
	require.NoError(t, err)
	assert.Equal(t, []string{ruleID}, shifted)

	var maxCount int
	var endsOn *time.Time
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT max_count, ends_on FROM recurrence_rules WHERE id = $1`, ruleID).Scan(&maxCount, &endsOn))
	assert.Equal(t, 8, maxCount)
	assert.Nil(t, endsOn)

	_, err = service.NewRecurrenceService(repository.NewRecurrenceRepository(pool)).
		Materialize(ctx, ruleID, first.AddDate(0, 0, 7*10))
	require.NoError(t, err)
	assert.Equal(t, 6, countLessons(t, pool, courseID, "scheduled"))
}

// Заморозка участника группы расписание группы не меняет: занятия идут для
// остальных.
func TestPauseCreate_GroupScheduleUntouched(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	base := time.Now().UTC().AddDate(0, 0, 7)
	first := time.Date(base.Year(), base.Month(), base.Day(), 12, 0, 0, 0, time.UTC)
	tutorID, groupID, ruleID := seedSeries(t, pool, first, 6) // курс seedSeries — групповой
	s := addStudent(t, pool, tutorID, "Участник")
	enroll(t, pool, groupID, s, "NOW() - interval '1 month'")

	var endsBefore time.Time
	require.NoError(t, pool.QueryRow(ctx, `SELECT ends_on FROM recurrence_rules WHERE id = $1`, ruleID).Scan(&endsBefore))

	_, shifted, err := repository.NewPauseRepository(pool).Create(ctx, s, models.CreatePauseRequest{
		StartsOn: dateOnly(first), EndsOn: dateOnly(first.AddDate(0, 0, 20)),
	})
	require.NoError(t, err)
	assert.Empty(t, shifted)
	assert.Equal(t, 0, countLessons(t, pool, groupID, "cancelled"))

	var endsAfter time.Time
	require.NoError(t, pool.QueryRow(ctx, `SELECT ends_on FROM recurrence_rules WHERE id = $1`, ruleID).Scan(&endsAfter))
	assert.Equal(t, endsBefore, endsAfter)
}

// Заморозка задним числом статусы не трогает: прошедшие уроки уже проведены,
// их выводит из счёта только предикат участия.
func TestPauseCreate_RetroactiveKeepsStatuses(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	tutorID, s := seedTutorStudent(t, pool)
	courseID := addIndividualCourse(t, pool, tutorID, s)
	addLessonAt(t, pool, courseID, "NOW() - interval '3 days'", "completed")

	today := dateOnly(time.Now().UTC())
	_, shifted, err := repository.NewPauseRepository(pool).Create(ctx, s, models.CreatePauseRequest{
		StartsOn: today.AddDate(0, 0, -7), EndsOn: today.AddDate(0, 0, -1),
	})
	require.NoError(t, err)
	assert.Empty(t, shifted)
	assert.Equal(t, 1, countLessons(t, pool, courseID, "completed"))
}

func TestPauseListAndDelete_ScopedByStudent(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	tutorID, s := seedTutorStudent(t, pool)
	other := addStudent(t, pool, tutorID, "Другой")
	repo := repository.NewPauseRepository(pool)

	today := dateOnly(time.Now().UTC())
	reason := "сессия"
	pause, _, err := repo.Create(ctx, s, models.CreatePauseRequest{StartsOn: today, EndsOn: today.AddDate(0, 0, 3), Reason: &reason})
	require.NoError(t, err)

	list, err := repo.ListByStudent(ctx, s)
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.Equal(t, "сессия", *list[0].Reason)

	deleted, err := repo.Delete(ctx, pause.ID, other)
	require.NoError(t, err)
	assert.False(t, deleted)

	deleted, err = repo.Delete(ctx, pause.ID, s)
	require.NoError(t, err)
	assert.True(t, deleted)

	list, err = repo.ListByStudent(ctx, s)
	require.NoError(t, err)
	assert.Empty(t, list)
}
```

Run: `go vet -tags=integration ./repository/`
Expected: FAIL — `repository.NewPauseRepository undefined`.

- [ ] **Step 3: Репозиторий**

Create `repository/pause.go`:

```go
package repository

import (
	"context"
	"tutorgo/models"

	"github.com/jackc/pgx/v5/pgxpool"
)

type PauseRepository interface {
	Create(ctx context.Context, studentID string, req models.CreatePauseRequest) (models.StudentPause, []string, error)
	ListByStudent(ctx context.Context, studentID string) ([]models.StudentPause, error)
	Delete(ctx context.Context, id, studentID string) (bool, error)
}

type pauseRepository struct {
	pool *pgxpool.Pool
}

func NewPauseRepository(pool *pgxpool.Pool) PauseRepository {
	return &pauseRepository{pool: pool}
}

// Create заводит паузу, отменяет уроки индивидуальных курсов ученика в её
// интервале и сдвигает хвост их правил — одним оператором (спека, п. 6.9).
// Отдельными запросами правило на миг оказалось бы в состоянии «конец сдвинут,
// счётчик нет», а обрыв посередине оставил бы отменённые уроки без продления.
//
// Отмена нужна, иначе автозавершение закроет уроки паузы как проведённые.
// Хвост: ends_on += дней в паузе, max_count += отменённых вхождений правила;
// NULL + n остаётся NULL, поэтому бессрочное правило не меняется само.
// materialized_until откатывается к началу паузы: Materialize идёт от него, и
// без отката хвост за старым концом не появился бы никогда. Дублей не будет —
// (rule_id, occurrence_date) уникален, отменённые держатся тумбстоунами.
// Групповые уроки и правила не трогаются: занятия идут для остальных.
//
// Возвращает id правил со сдвинутым хвостом — их материализует сервис.
//
// ponytail: даты уроков сравниваются в UTC, как в lessonInParticipation. И
// вхождения, ещё не материализованные к моменту заморозки (пауза дальше
// горизонта в 6 месяцев), потом создадутся запланированными — ceiling редкий,
// апгрейд — учитывать паузы в Materialize.
func (r *pauseRepository) Create(ctx context.Context, studentID string, req models.CreatePauseRequest) (models.StudentPause, []string, error) {
	var p models.StudentPause
	var shifted []string
	err := r.pool.QueryRow(ctx,
		`WITH pause AS (
		     INSERT INTO student_pauses (student_id, starts_on, ends_on, reason)
		     VALUES ($1, $2::date, $3::date, $4)
		     RETURNING id, student_id, starts_on, ends_on, reason, created_at
		 ),
		 cancelled AS (
		     UPDATE lessons AS l SET status = 'cancelled'
		       FROM courses c, pause
		      WHERE c.id = l.course_id
		        AND c.student_id = pause.student_id
		        AND l.status = 'scheduled'
		        AND l.scheduled_at::date BETWEEN pause.starts_on AND pause.ends_on
		     RETURNING l.rule_id
		 ),
		 per_rule AS (
		     SELECT rule_id, count(*)::int AS n
		       FROM cancelled
		      WHERE rule_id IS NOT NULL
		      GROUP BY rule_id
		 ),
		 shifted AS (
		     UPDATE recurrence_rules AS r
		        SET ends_on            = r.ends_on + ($3::date - $2::date + 1),
		            max_count          = r.max_count + pr.n,
		            materialized_until = LEAST(r.materialized_until, $2::date)
		       FROM per_rule pr
		      WHERE r.id = pr.rule_id
		     RETURNING r.id::text
		 )
		 SELECT p.id, p.student_id, p.starts_on, p.ends_on, p.reason, p.created_at,
		        ARRAY(SELECT id FROM shifted)
		   FROM pause p`,
		studentID, req.StartsOn, req.EndsOn, req.Reason,
	).Scan(&p.ID, &p.StudentID, &p.StartsOn, &p.EndsOn, &p.Reason, &p.CreatedAt, &shifted)
	return p, shifted, err
}

func (r *pauseRepository) ListByStudent(ctx context.Context, studentID string) ([]models.StudentPause, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, student_id, starts_on, ends_on, reason, created_at
		 FROM student_pauses
		 WHERE student_id = $1
		 ORDER BY starts_on DESC`, studentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	pauses := []models.StudentPause{}
	for rows.Next() {
		var p models.StudentPause
		if err := rows.Scan(&p.ID, &p.StudentID, &p.StartsOn, &p.EndsOn, &p.Reason, &p.CreatedAt); err != nil {
			return nil, err
		}
		pauses = append(pauses, p)
	}
	return pauses, rows.Err()
}

// Delete — разморозка: уроки паузы возвращаются в счёт. Отменённые уроки и
// сдвинутый хвост не откатываются — расписание уже пересобрано (спека, п. 6.9).
func (r *pauseRepository) Delete(ctx context.Context, id, studentID string) (bool, error) {
	tag, err := r.pool.Exec(ctx,
		`DELETE FROM student_pauses WHERE id = $1 AND student_id = $2`, id, studentID)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}
```

If `assert.Empty(t, shifted)` fails because pgx scans an empty array into `nil` vs `[]string{}` — both satisfy `Empty`; `assert.Equal(t, []string{ruleID}, shifted)` requires a non-nil slice with one element, which pgx produces.

Run: `TEST_DB_URL="postgres://dev:dev@localhost:5432/tutorgo_dev?sslmode=disable" go test -tags=integration -count=1 ./repository/ -run 'TestPause' -v`
Expected: PASS.

- [ ] **Step 4: Падающие unit-тесты сервиса**

Before writing: `grep -n 'type mock' service/*_test.go` — имена `mockPauseRepo` и `mockMaterializer` должны быть свободны; `mockStudentRepo` уже есть в `service/student_test.go` и подходит для `GetByID`.

Create `service/pause_test.go`:

```go
package service_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"tutorgo/models"
	"tutorgo/service"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type mockPauseRepo struct{ mock.Mock }

func (m *mockPauseRepo) Create(ctx context.Context, studentID string, req models.CreatePauseRequest) (models.StudentPause, []string, error) {
	args := m.Called(ctx, studentID, req)
	return args.Get(0).(models.StudentPause), args.Get(1).([]string), args.Error(2)
}

func (m *mockPauseRepo) ListByStudent(ctx context.Context, studentID string) ([]models.StudentPause, error) {
	args := m.Called(ctx, studentID)
	return args.Get(0).([]models.StudentPause), args.Error(1)
}

func (m *mockPauseRepo) Delete(ctx context.Context, id, studentID string) (bool, error) {
	args := m.Called(ctx, id, studentID)
	return args.Bool(0), args.Error(1)
}

type mockMaterializer struct{ mock.Mock }

func (m *mockMaterializer) Materialize(ctx context.Context, ruleID string, horizon time.Time) (int, error) {
	args := m.Called(ctx, ruleID, horizon)
	return args.Int(0), args.Error(1)
}

var pauseDay = time.Date(2026, time.October, 1, 0, 0, 0, 0, time.UTC)

func TestPauseCreate_EndsBeforeStartsRejected(t *testing.T) {
	repo, students := new(mockPauseRepo), new(mockStudentRepo)
	svc := service.NewPauseService(repo, students, new(mockMaterializer))
	students.On("GetByID", mock.Anything, "stu-1", "tutor-1").Return(models.Student{ID: "stu-1"}, nil)

	_, err := svc.Create(context.Background(), "stu-1", "tutor-1",
		models.CreatePauseRequest{StartsOn: pauseDay, EndsOn: pauseDay.AddDate(0, 0, -1)})

	assert.ErrorIs(t, err, service.ErrBadRequest)
	repo.AssertNotCalled(t, "Create", mock.Anything, mock.Anything, mock.Anything)
}

func TestPauseCreate_ForeignStudentNotFound(t *testing.T) {
	repo, students := new(mockPauseRepo), new(mockStudentRepo)
	svc := service.NewPauseService(repo, students, new(mockMaterializer))
	students.On("GetByID", mock.Anything, "stu-1", "tutor-1").Return(models.Student{}, errors.New("no rows"))

	_, err := svc.Create(context.Background(), "stu-1", "tutor-1",
		models.CreatePauseRequest{StartsOn: pauseDay, EndsOn: pauseDay})

	assert.ErrorIs(t, err, service.ErrNotFound)
	repo.AssertNotCalled(t, "Create", mock.Anything, mock.Anything, mock.Anything)
}

// Каждое правило со сдвинутым хвостом материализуется сразу; сбой
// материализации паузу не отменяет — ночная ExtendAll догонит (спека, п. 6.9).
func TestPauseCreate_MaterializesShiftedRules(t *testing.T) {
	repo, students, rules := new(mockPauseRepo), new(mockStudentRepo), new(mockMaterializer)
	svc := service.NewPauseService(repo, students, rules)
	req := models.CreatePauseRequest{StartsOn: pauseDay, EndsOn: pauseDay.AddDate(0, 0, 13)}
	students.On("GetByID", mock.Anything, "stu-1", "tutor-1").Return(models.Student{ID: "stu-1"}, nil)
	repo.On("Create", mock.Anything, "stu-1", req).Return(models.StudentPause{ID: "pause-1"}, []string{"r1", "r2"}, nil)
	rules.On("Materialize", mock.Anything, "r1", mock.Anything).Return(2, nil)
	rules.On("Materialize", mock.Anything, "r2", mock.Anything).Return(0, errors.New("boom"))

	pause, err := svc.Create(context.Background(), "stu-1", "tutor-1", req)

	assert.NoError(t, err)
	assert.Equal(t, "pause-1", pause.ID)
	rules.AssertExpectations(t)
}

func TestPauseDelete_MissingIsNotFound(t *testing.T) {
	repo, students := new(mockPauseRepo), new(mockStudentRepo)
	svc := service.NewPauseService(repo, students, new(mockMaterializer))
	students.On("GetByID", mock.Anything, "stu-1", "tutor-1").Return(models.Student{ID: "stu-1"}, nil)
	repo.On("Delete", mock.Anything, "pause-1", "stu-1").Return(false, nil)

	err := svc.Delete(context.Background(), "pause-1", "stu-1", "tutor-1")

	assert.ErrorIs(t, err, service.ErrNotFound)
}
```

Run: `go vet ./service/`
Expected: FAIL — `service.NewPauseService undefined`.

- [ ] **Step 5: Сервис**

Create `service/pause.go`:

```go
package service

import (
	"context"
	"fmt"
	"time"
	"tutorgo/models"
	"tutorgo/repository"
)

// Заморозка складывает уже существующие действия — зависимости узкими
// интерфейсами, как в onboarding.go.

type pauseStudents interface {
	GetByID(ctx context.Context, id string, tutorID string) (models.Student, error)
}

type pauseMaterializer interface {
	Materialize(ctx context.Context, ruleID string, horizon time.Time) (int, error)
}

type PauseService interface {
	Create(ctx context.Context, studentID, tutorID string, req models.CreatePauseRequest) (models.StudentPause, error)
	List(ctx context.Context, studentID, tutorID string) ([]models.StudentPause, error)
	Delete(ctx context.Context, id, studentID, tutorID string) error
}

type pauseService struct {
	repo       repository.PauseRepository
	students   pauseStudents
	recurrence pauseMaterializer
}

func NewPauseService(repo repository.PauseRepository, students pauseStudents, recurrence pauseMaterializer) PauseService {
	return &pauseService{repo: repo, students: students, recurrence: recurrence}
}

// Create замораживает ученика (спека, п. 6.9). Пауза, отмена уроков и сдвиг
// хвоста правил — один оператор в репозитории; здесь — только материализация
// сдвинутых хвостов. Её сбой паузу не отменяет: репозиторий откатил
// materialized_until, и правило попадёт в ночную ExtendAll.
func (s *pauseService) Create(ctx context.Context, studentID, tutorID string, req models.CreatePauseRequest) (models.StudentPause, error) {
	if _, err := s.students.GetByID(ctx, studentID, tutorID); err != nil {
		return models.StudentPause{}, fmt.Errorf("student: %w", ErrNotFound)
	}
	if req.EndsOn.Before(req.StartsOn) {
		return models.StudentPause{}, fmt.Errorf("ends_on before starts_on: %w", ErrBadRequest)
	}
	pause, shifted, err := s.repo.Create(ctx, studentID, req)
	if err != nil {
		return models.StudentPause{}, err
	}
	// Кеш сбрасываем сразу после оператора: уроки уже отменены, и календарь
	// обязан это показать, даже если материализация ниже не удастся.
	globalCalendarCache.Invalidate(tutorID)
	horizon := time.Now().Add(RecurrenceHorizon)
	for _, ruleID := range shifted {
		_, _ = s.recurrence.Materialize(ctx, ruleID, horizon)
	}
	return pause, nil
}

func (s *pauseService) List(ctx context.Context, studentID, tutorID string) ([]models.StudentPause, error) {
	if _, err := s.students.GetByID(ctx, studentID, tutorID); err != nil {
		return nil, fmt.Errorf("student: %w", ErrNotFound)
	}
	return s.repo.ListByStudent(ctx, studentID)
}

// Delete — разморозка: уроки паузы возвращаются в счёт, отменённые и сдвинутый
// хвост остаются как есть (спека, п. 6.9).
func (s *pauseService) Delete(ctx context.Context, id, studentID, tutorID string) error {
	if _, err := s.students.GetByID(ctx, studentID, tutorID); err != nil {
		return fmt.Errorf("student: %w", ErrNotFound)
	}
	deleted, err := s.repo.Delete(ctx, id, studentID)
	if err != nil {
		return err
	}
	if !deleted {
		return fmt.Errorf("pause: %w", ErrNotFound)
	}
	globalCalendarCache.Invalidate(tutorID)
	return nil
}
```

Run: `go test ./service/ -run 'TestPause' -v`
Expected: PASS.

- [ ] **Step 6: Хендлер и маршруты**

Create `handlers/pause.go`:

```go
package handlers

import (
	"log/slog"
	"net/http"

	"tutorgo/models"
	"tutorgo/service"

	"github.com/gin-gonic/gin"
)

// PauseHandler — заморозка ученика с карточки ученика (спека, п. 6.9).
type PauseHandler struct {
	service service.PauseService
	log     *slog.Logger
}

func NewPauseHandler(svc service.PauseService, log *slog.Logger) *PauseHandler {
	return &PauseHandler{service: svc, log: log}
}

func (h *PauseHandler) List(c *gin.Context) {
	tutorID := c.GetString("tutorID")
	if tutorID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	pauses, err := h.service.List(c.Request.Context(), c.Param("id"), tutorID)
	if err != nil {
		handleServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, pauses)
}

func (h *PauseHandler) Create(c *gin.Context) {
	tutorID := c.GetString("tutorID")
	if tutorID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	var req models.CreatePauseRequest
	if !bindAndValidate(c, &req) {
		return
	}
	studentID := c.Param("id")
	pause, err := h.service.Create(c.Request.Context(), studentID, tutorID, req)
	if err != nil {
		h.log.Error("Failed to create pause", slog.String("student_id", studentID), slog.String("error", err.Error()))
		handleServiceError(c, err)
		return
	}
	h.log.Info("Student paused", slog.String("student_id", studentID), slog.String("id", pause.ID))
	c.JSON(http.StatusCreated, pause)
}

func (h *PauseHandler) Delete(c *gin.Context) {
	tutorID := c.GetString("tutorID")
	if tutorID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	if err := h.service.Delete(c.Request.Context(), c.Param("pauseId"), c.Param("id"), tutorID); err != nil {
		handleServiceError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}
```

`router/router.go`:
- repositories block: `pauseRepo := repository.NewPauseRepository(pool)`;
- services block, below `recurrenceService` and `studentService`: `pauseService := service.NewPauseService(pauseRepo, studentRepo, recurrenceService)`;
- handlers block: `pauseHandler := handlers.NewPauseHandler(pauseService, log)`;
- routes, after `auth.POST("/students/:id/invite", studentHandler.Invite)`:
```go
		auth.GET("/students/:id/pauses", pauseHandler.List)
		auth.POST("/students/:id/pauses", pauseHandler.Create)
		auth.DELETE("/students/:id/pauses/:pauseId", pauseHandler.Delete)
```

- [ ] **Step 7: Всё зелёное и коммит**

Run:
```bash
go build ./... && go vet ./... && go vet -tags=integration ./repository/
make test
TEST_DB_URL="postgres://dev:dev@localhost:5432/tutorgo_dev?sslmode=disable" go test -tags=integration -count=1 ./repository/
```
Expected: PASS, кроме известного красного.

```bash
git add models/pause.go repository/pause.go service/pause.go handlers/pause.go router/router.go service/pause_test.go repository/pause_integration_test.go
git commit -m "feat(students): заморозка ученика — пауза, отмена уроков и сдвиг хвоста серии

Co-Authored-By: Claude Opus 5 <noreply@anthropic.com>
Claude-Session: https://claude.ai/code/session_01KLiSrLKjmzZMQgDzgsUdbn"
```

---

### Task 7: Фронт — адресный платёж, баланс ученика, вкладка «Долги»

**Files:**
- Modify: `frontend/src/types/api.ts`
- Modify: `frontend/src/lib/api/payments.ts`, `frontend/src/lib/api/courses.ts`
- Modify: `frontend/src/lib/hooks/usePayments.ts`, `frontend/src/lib/hooks/useCourses.ts`
- Modify: `frontend/src/schemas/payment.ts`
- Modify: `frontend/src/components/payments/PaymentForm.tsx`
- Create: `frontend/src/components/payments/DebtsList.tsx`
- Modify: `frontend/src/app/(dashboard)/payments/page.tsx`
- Modify: `frontend/src/app/(dashboard)/courses/[id]/page.tsx`

**Interfaces:**
- Consumes: только «Контракт API фазы» из шапки плана (бэкенд пишется параллельно; запускать его не нужно).
- Produces (для Task 8):
  - типы `Payment.student_id`, `StudentDebt`, `DebtByCourse`, `StudentPause` в `@/types/api`
  - `paymentsApi.createBulk(data: BulkPaymentInput): Promise<Payment[]>`, `paymentsApi.debts(): Promise<StudentDebt[]>`, тип `BulkPaymentInput`; `PaymentInput.student_id: string` обязателен
  - `useDebts()`, `useCreateBulkPayment()`, `paymentKeys.debts`; мутации платежей инвалидируют префикс `['payments']` и балансы своих курсов
  - `PaymentForm` — необязательный проп `students?: { id: string; name: string }[]` (есть — форма спрашивает адресата)
  - `useCourseBalance(id: string, studentId: string | null | undefined)`

Браузера в окружении нет: проверка — `npm run lint` и `npx tsc --noEmit`. Стиль разметки — как у соседних экранов (`SectionCard`, `EmptyState`, `ErrorState`, инлайн-стили токенами `var(--…)` на `/payments`, Tailwind на странице курса).

- [ ] **Step 1: Базовая линия проверок**

Run: `cd /home/dragonbrn/tutorgo/frontend && npm run lint 2>&1 | tail -5; npx tsc --noEmit 2>&1 | tail -5`
Записать в отчёт число ошибок до правок: после задачи в затронутых файлах новых быть не должно.

- [ ] **Step 2: Типы и API**

`frontend/src/types/api.ts` — заменить `Payment`, удалить неиспользуемый `PaymentBalance` (проверить: `grep -rn PaymentBalance src` — только `types/api.ts` и `lib/api/payments.ts`), добавить новые типы рядом:

```ts
export interface Payment {
  id: string
  course_id: string
  /** null — легаси-платёж группы без адресата (спека 2026-09-06, п. 3.4). */
  student_id: string | null
  amount: number
  lessons_count: number
  paid_at: string
  /** Только в списках по репетитору (/payments, /payments/recent). */
  subject?: string
  /** Имя адресата; null у платежа без адресата. */
  student_name?: string | null
}

/** Долг по одному предмету. */
export interface DebtByCourse {
  course_id: string
  subject: string
  lessons_owed: number
  amount_owed: number
}

/** Должник: долг по всем его курсам одной строкой (спека 2026-09-06, п. 6.5). */
export interface StudentDebt {
  student_id: string
  student_name: string
  lessons_owed: number
  amount_owed: number
  next_lesson_at: string | null
  courses: DebtByCourse[]
}

/** Заморозка ученика с даты по дату включительно (спека 2026-09-06, п. 6.9). */
export interface StudentPause {
  id: string
  student_id: string
  starts_on: string
  ends_on: string
  reason: string | null
  created_at: string
}
```

Replace `frontend/src/lib/api/payments.ts` with:

```ts
import { api } from './client'
import { Payment, PagedResponse, StudentDebt } from '@/types/api'

export interface PaymentInput {
  course_id: string
  /** Обязателен: платёж адресный (спека 2026-09-06, п. 6.2). */
  student_id: string
  amount: number
  lessons_count: number
  paid_at?: string
}

export type PaymentUpdateInput = Omit<PaymentInput, 'course_id'>

/** Одна оплата на несколько предметов. В БД уезжают только строки — сумма
 *  «к распределению» нигде не хранится (спека 2026-09-06, п. 6.8). */
export interface BulkPaymentInput {
  /** YYYY-MM-DD */
  paid_at: string
  items: { course_id: string; student_id: string; amount: number; lessons_count: number }[]
}

export interface PaymentListParams {
  page:  number
  limit: number
}

export const paymentsApi = {
  list: (courseId: string) =>
    api.get<PagedResponse<Payment>>('/payments', { params: { course_id: courseId } })
      .then((r) => r.data.data ?? []),
  listPaged: (p: PaymentListParams) =>
    api.get<PagedResponse<Payment>>('/payments', { params: p }).then((r) => r.data),
  listRecent: () =>
    api.get<Payment[]>('/payments/recent').then((r) => r.data ?? []),
  create: (data: PaymentInput) => {
    const payload = { ...data, paid_at: data.paid_at ? data.paid_at + 'T00:00:00Z' : undefined }
    return api.post<Payment>('/payments', payload).then((r) => r.data)
  },
  createBulk: (data: BulkPaymentInput) =>
    api.post<Payment[]>('/payments/bulk', { ...data, paid_at: data.paid_at + 'T00:00:00Z' })
      .then((r) => r.data ?? []),
  update: (id: string, data: PaymentUpdateInput) => {
    const payload = { ...data, paid_at: data.paid_at ? data.paid_at + 'T00:00:00Z' : undefined }
    return api.put<Payment>(`/payments/${id}`, payload).then((r) => r.data)
  },
  delete: (id: string) =>
    api.delete(`/payments/${id}`).then(() => id),
  debts: () =>
    api.get<StudentDebt[]>('/payments/debts').then((r) => r.data ?? []),
  monthlyIncome: () =>
    api.get<{ total: number }>('/payments/monthly-income').then((r) => r.data.total),
  monthlyExpected: () =>
    api.get<{ total: number }>('/payments/monthly-expected').then((r) => r.data.total),
}
```

`frontend/src/lib/api/courses.ts` — replace `getBalance`:

```ts
  getBalance: (id: string, studentId: string) =>
    api.get<CourseBalance>('/payments/balance', { params: { course_id: id, student_id: studentId } })
      .then((r) => r.data),
```

- [ ] **Step 3: Хуки и схема**

`frontend/src/lib/hooks/useCourses.ts` — replace `useCourseBalance`:

```ts
// Баланс — свойство пары «курс + ученик» (спека 2026-09-06, п. 6.4): у группы
// без выбранного ученика его нет, и запрос не уходит. Ключ начинается с
// courseKeys.balance(id) — инвалидация по курсу сбрасывает балансы всех учеников.
export function useCourseBalance(id: string, studentId: string | null | undefined) {
  return useQuery({
    queryKey: [...courseKeys.balance(id), studentId],
    queryFn:  () => coursesApi.getBalance(id, studentId as string),
    enabled:  !!id && !!studentId,
  })
}
```

Replace `frontend/src/lib/hooks/usePayments.ts` with:

```ts
import { useQuery, useMutation, useQueryClient, QueryClient } from '@tanstack/react-query'
import { paymentsApi, PaymentListParams, PaymentUpdateInput } from '@/lib/api/payments'
import { courseKeys } from '@/lib/hooks/useCourses'

export const paymentKeys = {
  byCourse:        (courseId: string) => ['payments', 'course', courseId] as const,
  paged:           (p: PaymentListParams) => ['payments', 'list', p] as const,
  recent:          ['payments', 'recent'] as const,
  monthlyIncome:   ['payments', 'monthly-income'] as const,
  monthlyExpected: ['payments', 'monthly-expected'] as const,
  debts:           ['payments', 'debts'] as const,
}

// Любой платёж меняет всё денежное разом: историю, доход, прогноз и долги.
// Все эти ключи начинаются с ['payments'] — сбрасываем префиксом, плюс балансы
// затронутых курсов (у них свой корень ['courses']).
function invalidateMoney(qc: QueryClient, courseIds: string[]) {
  qc.invalidateQueries({ queryKey: ['payments'] })
  for (const id of courseIds) qc.invalidateQueries({ queryKey: courseKeys.balance(id) })
}

export function useMonthlyIncome() {
  return useQuery({ queryKey: paymentKeys.monthlyIncome, queryFn: paymentsApi.monthlyIncome })
}

export function useMonthlyExpected() {
  return useQuery({ queryKey: paymentKeys.monthlyExpected, queryFn: paymentsApi.monthlyExpected })
}

export function useRecentPayments() {
  return useQuery({ queryKey: paymentKeys.recent, queryFn: paymentsApi.listRecent })
}

export function useDebts() {
  return useQuery({ queryKey: paymentKeys.debts, queryFn: paymentsApi.debts })
}

export function usePayments(courseId: string) {
  return useQuery({
    queryKey: paymentKeys.byCourse(courseId),
    queryFn:  () => paymentsApi.list(courseId),
    enabled:  !!courseId,
  })
}

export function useCreatePayment(courseId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: paymentsApi.create,
    onSuccess:  () => invalidateMoney(qc, [courseId]),
  })
}

export function useCreateBulkPayment() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: paymentsApi.createBulk,
    onSuccess:  (_data, input) => invalidateMoney(qc, input.items.map((i) => i.course_id)),
  })
}

export function useUpdatePayment(courseId?: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ id, data }: { id: string; data: PaymentUpdateInput }) =>
      paymentsApi.update(id, data),
    onSuccess: () => invalidateMoney(qc, courseId ? [courseId] : []),
  })
}

export function useDeletePayment(courseId?: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: paymentsApi.delete,
    onSuccess:  () => invalidateMoney(qc, courseId ? [courseId] : []),
  })
}

export function usePaymentsPaged(params: PaymentListParams) {
  return useQuery({
    queryKey: paymentKeys.paged(params),
    queryFn:  () => paymentsApi.listPaged(params),
  })
}
```

`frontend/src/schemas/payment.ts` — add the field to the object:

```ts
  // Адресат: форма группы спрашивает его явно, у индивидуального курса он
  // известен и не вводится — поэтому не обязателен в схеме (спека 2026-09-06, п. 6.2).
  student_id:    z.string().optional(),
```

- [ ] **Step 4: `PaymentForm` — выбор адресата**

In `frontend/src/components/payments/PaymentForm.tsx`:
- import `Select, SelectContent, SelectItem, SelectTrigger, SelectValue` from `@/components/ui/select` (usage pattern: `courses/[id]/page.tsx`, `onValueChange={(v) => …(v ?? '')}`, `SelectItem` takes `label`);
- props: add
```ts
  /** Участники группы: форма спрашивает, кто заплатил. Индивидуальному курсу не
   *  передаётся — адресат и так известен (спека 2026-09-06, п. 6.2). */
  students?:        { id: string; name: string }[]
```
- `useForm` destructure additionally `setError`; `const studentId = watch('student_id')`;
- in the create branch of the `reset` effect add `student_id: undefined`;
- at the start of `submit`:
```ts
    if (students && !values.student_id) {
      setError('student_id', { message: 'Выберите ученика' })
      return
    }
```
- first field of the form, rendered only when `students` is passed:
```tsx
          {students && (
            <div className="space-y-1.5">
              <Label>Ученик</Label>
              <Select
                value={studentId ?? ''}
                onValueChange={(v) => setValue('student_id', v || undefined, { shouldValidate: true })}
              >
                <SelectTrigger className="w-full">
                  <SelectValue placeholder="Кто заплатил?" />
                </SelectTrigger>
                <SelectContent>
                  {students.map((s) => (
                    <SelectItem key={s.id} value={s.id} label={s.name}>{s.name}</SelectItem>
                  ))}
                </SelectContent>
              </Select>
              {errors.student_id && (
                <p className="text-xs text-destructive">{errors.student_id.message}</p>
              )}
            </div>
          )}
```

- [ ] **Step 5: Страница курса**

In `frontend/src/app/(dashboard)/courses/[id]/page.tsx`:

1. `const { data: balance } = useCourseBalance(id, course?.student_id)`.
2. Replace `handlePaymentSubmit`:
```tsx
  async function handlePaymentSubmit(values: PaymentFormValues) {
    // Индивидуальный курс — адресат его ученик; группа — участник, выбранный в
    // форме (без него форма не отправит).
    const studentId = course?.student_id ?? values.student_id
    if (!studentId) return
    if (editingPayment) {
      await updatePayment.mutateAsync({
        id:   editingPayment.id,
        data: {
          student_id:    studentId,
          amount:        values.amount,
          lessons_count: values.lessons_count,
          paid_at:       values.paid_at,
        },
      })
      toast.success('Платёж обновлён')
    } else {
      await createPayment.mutateAsync({
        course_id:     id,
        student_id:    studentId,
        amount:        values.amount,
        lessons_count: values.lessons_count,
        paid_at:       values.paid_at,
      })
    }
  }
```
3. Wrap the «Баланс уроков» card in `{!isGroup && ( … )}` with the comment `{/* Баланс курса целиком у группы ничего не значит (спека 2026-09-06, п. 1.4): он у каждого участника свой — вкладка «Долги» на /payments. */}`. If the surrounding grid then leaves an empty column for groups, let the info card span it (`md:col-span-2` or equivalent for the grid used there).
4. In the payments list row, for groups show the addressee between the amount and the lessons count:
```tsx
                {isGroup && (
                  <span
                    className="truncate"
                    style={{ color: p.student_id ? 'var(--muted-foreground)' : 'var(--warning)' }}
                  >
                    {p.student_name ?? 'без адресата'}
                  </span>
                )}
```
5. `PaymentForm` gets
```tsx
        // ponytail: адресата выбирают из текущего состава — ушедшего из группы
        // легаси-платежу не назначить без повторной записи. Прод: ушедших нет.
        students={isGroup
          ? enrollments.map((e) => ({
              id:   e.student_id,
              name: e.student_last_name ? `${e.student_first_name} ${e.student_last_name}` : e.student_first_name,
            }))
          : undefined}
```
(the comment goes as a JSX comment above the prop if the linter rejects `//` there) and `student_id: editingPayment.student_id ?? undefined` inside `initialValues`.

- [ ] **Step 6: Вкладка «Долги»**

Create `frontend/src/components/payments/DebtsList.tsx`:

```tsx
'use client'

import { useRouter } from 'next/navigation'
import { HandCoins } from 'lucide-react'

import { useDebts } from '@/lib/hooks/usePayments'
import { SectionCard } from '@/components/common/SectionCard'
import { EmptyState } from '@/components/common/EmptyState'
import { ErrorState } from '@/components/common/ErrorState'

const fmtAmt = (n: number) => '₸' + Math.round(n).toLocaleString('ru-RU')

const COLS = '1fr 110px 120px'

/** «Кто мне должен» (спека 2026-09-06, п. 6.5): строка на ученика с разбивкой по
 *  предметам, клик ведёт в карточку. Кнопки «Напомнить» нет — канал доставки не
 *  выбран. Ушедшие из группы и архивные тоже здесь: архив долг не прощает. */
export function DebtsList() {
  const router = useRouter()
  const { data: debts = [], isLoading, isError, refetch } = useDebts()

  return (
    <SectionCard>
      <div style={{
        display: 'grid', gridTemplateColumns: COLS, alignItems: 'baseline', gap: 12,
        padding: '10px 18px 8px', borderBottom: '1px solid var(--border)',
      }}>
        {(['Ученик', 'Урок', 'Долг'] as const).map((label, i) => (
          <span key={label} style={{
            fontSize: 11.5, fontWeight: 500, color: 'var(--muted-foreground)',
            letterSpacing: '0.05em', textTransform: 'uppercase',
            ...(i === 2 ? { textAlign: 'right' } : {}),
          }}>{label}</span>
        ))}
      </div>

      {isLoading ? (
        <div className="space-y-2" style={{ padding: '10px 18px' }}>
          {[...Array(3)].map((_, i) => <div key={i} className="h-4 rounded bg-muted animate-pulse" />)}
        </div>
      ) : isError ? (
        <ErrorState what="долги" onRetry={() => refetch()} />
      ) : debts.length === 0 ? (
        <EmptyState
          icon={HandCoins}
          title="Никто не должен"
          description="Здесь появятся ученики, у которых проведённых уроков больше, чем оплачено"
        />
      ) : (
        <>
          {debts.map((d, i) => (
            <div
              key={d.student_id}
              role="button"
              tabIndex={0}
              className="hover:bg-muted/30"
              style={{
                display: 'grid', gridTemplateColumns: COLS, alignItems: 'center', gap: 12,
                padding: '10px 18px', cursor: 'pointer',
                borderTop: i === 0 ? 'none' : '1px solid var(--row-border)',
              }}
              onClick={() => router.push(`/students/${d.student_id}`)}
              onKeyDown={(e) => {
                if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); router.push(`/students/${d.student_id}`) }
              }}
            >
              <div style={{ minWidth: 0 }}>
                <div style={{ fontSize: 14, fontWeight: 600, color: 'var(--foreground)', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                  {d.student_name}
                </div>
                <div style={{ fontSize: 12, color: 'var(--muted-foreground)', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                  {d.courses.map((c) => `${c.subject} — ${c.lessons_owed} ур.`).join(' · ')}
                </div>
              </div>
              <span style={{ fontSize: 13, color: 'var(--muted-foreground)', fontVariantNumeric: 'tabular-nums' }}>
                {d.next_lesson_at
                  ? new Date(d.next_lesson_at).toLocaleDateString('ru-RU', { day: 'numeric', month: 'short' })
                  : '—'}
              </span>
              <div style={{ textAlign: 'right' }}>
                <div style={{ fontSize: 14, fontWeight: 600, color: 'var(--foreground)', fontVariantNumeric: 'tabular-nums' }}>
                  {fmtAmt(d.amount_owed)}
                </div>
                <div style={{ fontSize: 12, color: 'var(--muted-foreground)', fontVariantNumeric: 'tabular-nums' }}>
                  {d.lessons_owed} ур.
                </div>
              </div>
            </div>
          ))}
          <p style={{ fontSize: 12, color: 'var(--muted-foreground)', padding: '8px 18px 12px', margin: 0 }}>
            Сумма — уроки × цена урока курса; у пакетов, которые не делятся на уроки нацело, она приблизительна.
          </p>
        </>
      )}
    </SectionCard>
  )
}
```
If `HandCoins` is missing from the installed `lucide-react`, use `Wallet`. If `EmptyState`'s props differ from `/payments` usage, adapt to its actual signature.

In `frontend/src/app/(dashboard)/payments/page.tsx`:
1. Tabs from `@/components/ui/tabs` (`Tabs`, `TabsList`, `TabsTrigger`, `TabsContent`; check the base-ui `Tabs` props in `components/ui/tabs.tsx` — `value` / `onValueChange`). The active tab lives in the URL, like `page`:
```tsx
  const tab = searchParams.get('tab') === 'debts' ? 'debts' : 'history'

  function handleTabChange(next: string) {
    const p = new URLSearchParams(searchParams.toString())
    if (next === 'debts') p.set('tab', 'debts')
    else p.delete('tab')
    p.delete('page')
    router push(`/payments?${p}`)
  }
```
(write the last line as `router.push(\`/payments?${p}\`)`). Under the KPI block: `TabsList` with «История» (`history`) and «Долги» (`debts`); the history `TabsContent` holds the existing `SectionCard` and pagination; the debts `TabsContent` holds `<DebtsList />`. Keep the pagination-clamp effect working only for the history tab (skip it when `tab === 'debts'`).
2. `openEdit` — guard first:
```tsx
    // Адресата легаси-платежу группы выбирают из её состава — он есть только на
    // странице курса.
    if (!p.student_id) {
      toast.info('У платежа нет адресата — назначьте ученика на странице курса')
      router.push(`/courses/${p.course_id}`)
      return
    }
```
3. `handleEdit` — `if (!editingPayment?.student_id) return` and add `student_id: editingPayment.student_id` to `data`.
4. Row second line: `{p.student_name ?? 'Группа · без адресата'}`.

- [ ] **Step 7: Проверки и коммит**

Run: `cd /home/dragonbrn/tutorgo/frontend && npm run lint && npx tsc --noEmit`
Expected: ни одной новой ошибки относительно шага 1 (все места, где `paymentsApi.create`/`update`/`useCourseBalance` вызывались по-старому, исправлены: `grep -rn "useCourseBalance\|paymentsApi\.\(create\|update\)\|mutateAsync({" src` для сверки).

```bash
cd /home/dragonbrn/tutorgo
git add frontend/src/types/api.ts frontend/src/lib/api/payments.ts frontend/src/lib/api/courses.ts frontend/src/lib/hooks/usePayments.ts frontend/src/lib/hooks/useCourses.ts frontend/src/schemas/payment.ts frontend/src/components/payments/PaymentForm.tsx frontend/src/components/payments/DebtsList.tsx "frontend/src/app/(dashboard)/payments/page.tsx" "frontend/src/app/(dashboard)/courses/[id]/page.tsx"
git commit -m "feat(frontend): адресный платёж, баланс ученика, вкладка «Долги»

Co-Authored-By: Claude Opus 5 <noreply@anthropic.com>
Claude-Session: https://claude.ai/code/session_01KLiSrLKjmzZMQgDzgsUdbn"
```

---

### Task 8: Фронт — оплата и заморозка на карточке ученика

**Files:**
- Create: `frontend/src/lib/money.ts`, `frontend/src/lib/money.test.ts`
- Modify: `frontend/src/lib/api/students.ts`
- Modify: `frontend/src/lib/hooks/useStudents.ts`
- Create: `frontend/src/components/payments/BulkPaymentDialog.tsx`
- Create: `frontend/src/components/students/PauseDialog.tsx`
- Modify: `frontend/src/app/(dashboard)/students/[id]/page.tsx`

**Interfaces:**
- Consumes (Task 7): типы `Course`, `Student`, `StudentPause`, `ApiError` из `@/types/api`; `useDebts()`, `useCreateBulkPayment()`, `useCreatePayment(courseId)` (вход требует `student_id`); `PaymentForm` (`open`, `onClose`, `onSubmit`, `pricePerLesson`, `lessonsPerCycle`); `PaymentFormValues` из `@/schemas/payment`.
- Consumes (API): `GET/POST /students/:id/pauses`, `DELETE /students/:id/pauses/:pauseId`.
- Produces: `amountFor(lessons: number, pricePerCycle: number, lessonsPerCycle: number): number`; `studentsApi.pauses|createPause|deletePause`, `PauseInput`; `studentKeys.pauses(id)`, `usePauses(id)`, `useCreatePause(id)`, `useDeletePause(id)`; компоненты `BulkPaymentDialog`, `PauseDialog`.

**Правила (спека, п. 6.8, 6.9):**
- Кнопка «Оплата» — у активного ученика с активными курсами. Один курс — прежняя простая форма `PaymentForm`. Два и больше — диалог с разбивкой: большинство учеников на одном предмете, облагать их таблицей ради редкого случая не надо.
- В диалоге вводятся **уроки**; сумма строки считается как уроки × цена урока в целых тенге, кратное пакету — ровно пакеты; правленная руками сумма больше не пересчитывается. Поля стартуют с нулей при каждом открытии. Рядом с предметом — текущий долг и «погасить» (ставит уроки = долгу).
- «К распределению» только считает: в БД уезжают строки. Строка «Записывается X из Y · разница Z ₸» информирует и не блокирует сохранение.
- «Заморозить» — у активного ученика: даты «с/по» и причина. Разморозка — удаление паузы из списка «Заморозки» на карточке.

- [ ] **Step 1: Денежный хелпер с проверкой**

Look at `frontend/src/lib/retry.test.ts` first and mirror its import style (Node 24 TS-strip, `.ts` extension in the import).

Create `frontend/src/lib/money.ts`:

```ts
/** Сумма за уроки курса в целых тенге. Цена хранится пакетом «N уроков за X ₸»,
 *  цена урока — производная и бывает дробной (85 000 / 12 = 7 083,33…). Поэтому
 *  кратное пакету — ровно пакеты, остальное — уроки × цена урока с округлением
 *  до тенге (спека 2026-09-06, п. 4.4). */
export function amountFor(lessons: number, pricePerCycle: number, lessonsPerCycle: number): number {
  if (lessons <= 0) return 0
  const n = lessonsPerCycle > 0 ? lessonsPerCycle : 1
  if (lessons % n === 0) return (lessons / n) * pricePerCycle
  return Math.round((lessons * pricePerCycle) / n)
}
```

Create `frontend/src/lib/money.test.ts`:

```ts
import { test } from 'node:test'
import assert from 'node:assert/strict'

import { amountFor } from './money.ts'

test('кратное пакету — ровно пакеты, даже при дробной цене урока', () => {
  assert.equal(amountFor(12, 85000, 12), 85000)
  assert.equal(amountFor(24, 85000, 12), 170000)
})

test('неполный пакет — уроки × цена урока в целых тенге', () => {
  assert.equal(amountFor(5, 85000, 12), 35417)
  assert.equal(amountFor(4, 6000, 1), 24000)
})

test('ноль уроков — ноль', () => {
  assert.equal(amountFor(0, 40000, 8), 0)
})
```

Run: `cd /home/dragonbrn/tutorgo/frontend && node --test src/lib/money.test.ts`
Expected: 3 passing.

- [ ] **Step 2: API и хуки заморозки**

`frontend/src/lib/api/students.ts` — import `StudentPause` and add:

```ts
export interface PauseInput {
  /** YYYY-MM-DD */
  starts_on: string
  /** YYYY-MM-DD, включительно */
  ends_on:   string
  reason?:   string
}
```
and to `studentsApi`:
```ts
  pauses: (id: string) =>
    api.get<StudentPause[]>(`/students/${id}/pauses`).then((r) => r.data ?? []),
  createPause: (id: string, data: PauseInput) =>
    api.post<StudentPause>(`/students/${id}/pauses`, {
      starts_on: data.starts_on + 'T00:00:00Z',
      ends_on:   data.ends_on + 'T00:00:00Z',
      reason:    data.reason?.trim() || undefined,
    }).then((r) => r.data),
  deletePause: (id: string, pauseId: string) =>
    api.delete(`/students/${id}/pauses/${pauseId}`).then(() => pauseId),
```

`frontend/src/lib/hooks/useStudents.ts` — add `pauses: (id: string) => ['students', id, 'pauses'] as const` to `studentKeys`, import `QueryClient` and `PauseInput`, and:

```ts
// Заморозка отменяет уроки индивидуальных курсов, продлевает их серии и выводит
// уроки периода из сгорания (спека 2026-09-06, п. 6.9): устаревают календарь,
// уроки курсов, балансы, долги и прогноз.
function invalidateAfterPause(qc: QueryClient, studentId: string) {
  qc.invalidateQueries({ queryKey: studentKeys.pauses(studentId) })
  qc.invalidateQueries({ queryKey: ['payments'] })
  qc.invalidateQueries({ queryKey: ['courses'] })
  qc.invalidateQueries({ queryKey: ['lessons'] })
  qc.invalidateQueries({ queryKey: ['calendar'] })
}

export function usePauses(studentId: string) {
  return useQuery({
    queryKey: studentKeys.pauses(studentId),
    queryFn:  () => studentsApi.pauses(studentId),
    enabled:  !!studentId,
  })
}

export function useCreatePause(studentId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (data: PauseInput) => studentsApi.createPause(studentId, data),
    onSuccess:  () => invalidateAfterPause(qc, studentId),
  })
}

export function useDeletePause(studentId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (pauseId: string) => studentsApi.deletePause(studentId, pauseId),
    onSuccess:  () => invalidateAfterPause(qc, studentId),
  })
}
```
Verify the calendar query keys really start with `['calendar']` and lessons with `['lessons']`: `grep -rn "queryKey" src/lib/hooks/useLessons.ts src/lib/hooks/useCalendar*.ts 2>/dev/null`; if a key root differs, invalidate that root instead.

- [ ] **Step 3: Диалог оплаты на несколько предметов**

Create `frontend/src/components/payments/BulkPaymentDialog.tsx`:

```tsx
'use client'

import { useEffect, useState } from 'react'
import { toast } from 'sonner'

import { useCreateBulkPayment, useDebts } from '@/lib/hooks/usePayments'
import { amountFor } from '@/lib/money'
import type { ApiError, Course, Student } from '@/types/api'

import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Dialog, DialogContent, DialogHeader, DialogTitle } from '@/components/ui/dialog'

interface Row {
  lessons: number
  amount:  number
  /** Сумму правили руками — пересчёт от уроков её больше не трогает. */
  amountEdited: boolean
}

const EMPTY_ROW: Row = { lessons: 0, amount: 0, amountEdited: false }

const fmt = (n: number) => Math.round(n).toLocaleString('ru-RU')
const today = () => new Date().toISOString().slice(0, 10)

function priceLabel(c: Course) {
  return c.lessons_per_cycle > 1
    ? `${fmt(c.price_per_cycle)} ₸ за ${c.lessons_per_cycle} ур.`
    : `${fmt(c.price_per_cycle)} ₸ / урок`
}

interface BulkPaymentDialogProps {
  open:    boolean
  onClose: () => void
  student: Student
  /** Активные курсы ученика — их два и больше (спека 2026-09-06, п. 6.8). */
  courses: Course[]
}

/** Одна оплата на несколько предметов (спека 2026-09-06, п. 6.8). Истина —
 *  строки: в БД уезжают только они, поле «К распределению» лишь считает. */
export function BulkPaymentDialog({ open, onClose, student, courses }: BulkPaymentDialogProps) {
  const createBulk = useCreateBulkPayment()
  const { data: debts = [] } = useDebts()

  const [paidAt, setPaidAt] = useState(today)
  const [toDistribute, setToDistribute] = useState('')
  const [rows, setRows] = useState<Record<string, Row>>({})

  // Каждое открытие — с нулей: раскладывать чужие деньги по предметам за
  // тьютора форма не должна, в деньгах предсказуемость дороже клика.
  useEffect(() => {
    if (open) {
      setPaidAt(today())
      setToDistribute('')
      setRows({})
    }
  }, [open])

  const owedByCourse = new Map(
    (debts.find((d) => d.student_id === student.id)?.courses ?? [])
      .map((c) => [c.course_id, c.lessons_owed] as const),
  )

  const rowOf = (courseId: string) => rows[courseId] ?? EMPTY_ROW

  function setLessons(course: Course, value: number) {
    const lessons = Number.isFinite(value) && value > 0 ? Math.floor(value) : 0
    setRows((prev) => {
      const row = prev[course.id] ?? EMPTY_ROW
      const amount = row.amountEdited
        ? row.amount
        : amountFor(lessons, course.price_per_cycle, course.lessons_per_cycle)
      return { ...prev, [course.id]: { ...row, lessons, amount } }
    })
  }

  function setAmount(courseId: string, value: number) {
    const amount = Number.isFinite(value) && value > 0 ? value : 0
    setRows((prev) => ({ ...prev, [courseId]: { ...(prev[courseId] ?? EMPTY_ROW), amount, amountEdited: true } }))
  }

  const recorded   = courses.reduce((sum, c) => sum + rowOf(c.id).amount, 0)
  const target     = Number(toDistribute)
  const hasTarget  = toDistribute.trim() !== '' && Number.isFinite(target) && target > 0

  async function submit() {
    const items = courses
      .filter((c) => rowOf(c.id).lessons > 0 && rowOf(c.id).amount > 0)
      .map((c) => ({
        course_id:     c.id,
        student_id:    student.id,
        amount:        rowOf(c.id).amount,
        lessons_count: rowOf(c.id).lessons,
      }))
    if (items.length === 0) {
      toast.error('Укажите уроки хотя бы по одному предмету')
      return
    }
    try {
      await createBulk.mutateAsync({ paid_at: paidAt, items })
      toast.success('Оплата записана')
      onClose()
    } catch (err) {
      toast.error((err as ApiError).message ?? 'Не удалось записать оплату')
    }
  }

  const name = student.last_name ? `${student.first_name} ${student.last_name}` : student.first_name

  return (
    <Dialog open={open} onOpenChange={onClose}>
      <DialogContent className="max-w-lg">
        <DialogHeader>
          <DialogTitle>Оплата — {name}</DialogTitle>
        </DialogHeader>

        <div className="space-y-4 pt-2">
          <div className="grid grid-cols-2 gap-3">
            <div className="space-y-1.5">
              {/* Не «Получено»: эта сумма нигде не хранится, в отчёт идут строки ниже. */}
              <Label htmlFor="bulk-to-distribute">К распределению, ₸</Label>
              <Input
                id="bulk-to-distribute"
                type="number"
                min={0}
                placeholder="необязательно"
                value={toDistribute}
                onChange={(e) => setToDistribute(e.target.value)}
              />
            </div>
            <div className="space-y-1.5">
              <Label htmlFor="bulk-paid-at">Дата оплаты</Label>
              <Input id="bulk-paid-at" type="date" value={paidAt} onChange={(e) => setPaidAt(e.target.value)} />
            </div>
          </div>

          <div>
            {courses.map((course) => {
              const row  = rowOf(course.id)
              const owed = owedByCourse.get(course.id) ?? 0
              return (
                <div key={course.id} className="flex items-center justify-between gap-3 py-2 border-b last:border-0">
                  <div className="min-w-0">
                    <p className="text-sm font-medium truncate">{course.subject}</p>
                    <p className="text-xs text-muted-foreground">
                      {priceLabel(course)}
                      {owed > 0 && (
                        <>
                          {' · '}долг {owed} ур.{' '}
                          <button
                            type="button"
                            className="underline hover:text-foreground"
                            onClick={() => setLessons(course, owed)}
                          >
                            погасить
                          </button>
                        </>
                      )}
                    </p>
                  </div>
                  <div className="flex items-center gap-1.5 shrink-0">
                    <Input
                      type="number"
                      min={0}
                      aria-label={`Уроков: ${course.subject}`}
                      className="h-8 w-16 text-right"
                      placeholder="0"
                      value={row.lessons || ''}
                      onChange={(e) => setLessons(course, Number(e.target.value))}
                    />
                    <span className="text-xs text-muted-foreground">ур. =</span>
                    <Input
                      type="number"
                      min={0}
                      aria-label={`Сумма: ${course.subject}`}
                      className="h-8 w-24 text-right"
                      placeholder="0"
                      value={row.amount || ''}
                      onChange={(e) => setAmount(course.id, Number(e.target.value))}
                    />
                    <span className="text-xs text-muted-foreground">₸</span>
                  </div>
                </div>
              )
            })}
          </div>

          {/* Информирует, но не блокирует: округлили, простили тысячу, доплатят потом. */}
          <p className="text-xs text-muted-foreground">
            {hasTarget
              ? `Записывается ${fmt(recorded)} из ${fmt(target)} ₸` +
                (Math.round(recorded) !== Math.round(target) ? ` · разница ${fmt(Math.abs(target - recorded))} ₸` : '')
              : `Записывается ${fmt(recorded)} ₸`}
          </p>

          <div className="flex justify-end gap-2">
            <Button type="button" variant="outline" onClick={onClose}>Отмена</Button>
            <Button type="button" onClick={submit} disabled={createBulk.isPending}>
              {createBulk.isPending ? 'Сохранение...' : 'Записать'}
            </Button>
          </div>
        </div>
      </DialogContent>
    </Dialog>
  )
}
```

- [ ] **Step 4: Диалог заморозки**

Create `frontend/src/components/students/PauseDialog.tsx`:

```tsx
'use client'

import { useEffect, useState } from 'react'
import { toast } from 'sonner'

import { useCreatePause } from '@/lib/hooks/useStudents'
import type { ApiError } from '@/types/api'

import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Dialog, DialogContent, DialogHeader, DialogTitle } from '@/components/ui/dialog'

const today = () => new Date().toISOString().slice(0, 10)

interface PauseDialogProps {
  open:        boolean
  onClose:     () => void
  studentId:   string
  studentName: string
}

/** Заморозка «с даты по дату» (спека 2026-09-06, п. 6.9). Работает и задним
 *  числом: «он же болел всю прошлую неделю» — то же действие. */
export function PauseDialog({ open, onClose, studentId, studentName }: PauseDialogProps) {
  const createPause = useCreatePause(studentId)
  const [startsOn, setStartsOn] = useState('')
  const [endsOn, setEndsOn]     = useState('')
  const [reason, setReason]     = useState('')

  useEffect(() => {
    if (open) {
      const t = today()
      setStartsOn(t)
      setEndsOn(t)
      setReason('')
    }
  }, [open])

  // Даты ISO (YYYY-MM-DD) сравниваются строками.
  const reversed = !!startsOn && !!endsOn && endsOn < startsOn
  const invalid  = !startsOn || !endsOn || reversed

  async function submit() {
    if (invalid) return
    try {
      await createPause.mutateAsync({ starts_on: startsOn, ends_on: endsOn, reason })
      toast.success('Ученик заморожен')
      onClose()
    } catch (err) {
      toast.error((err as ApiError).message ?? 'Не удалось заморозить')
    }
  }

  return (
    <Dialog open={open} onOpenChange={onClose}>
      <DialogContent className="max-w-sm">
        <DialogHeader>
          <DialogTitle>Заморозить — {studentName}</DialogTitle>
        </DialogHeader>

        <div className="space-y-4 pt-2">
          <div className="grid grid-cols-2 gap-3">
            <div className="space-y-1.5">
              <Label htmlFor="pause-starts">С</Label>
              <Input id="pause-starts" type="date" value={startsOn} onChange={(e) => setStartsOn(e.target.value)} />
            </div>
            <div className="space-y-1.5">
              <Label htmlFor="pause-ends">По</Label>
              <Input id="pause-ends" type="date" value={endsOn} onChange={(e) => setEndsOn(e.target.value)} />
            </div>
          </div>
          {reversed && <p className="text-xs text-destructive">Дата окончания раньше начала</p>}

          <div className="space-y-1.5">
            <Label htmlFor="pause-reason">Причина</Label>
            <Input
              id="pause-reason"
              placeholder="уехал, болеет, сессия"
              maxLength={500}
              value={reason}
              onChange={(e) => setReason(e.target.value)}
            />
          </div>

          <p className="text-xs text-muted-foreground">
            Уроки этого периода не спишутся с оплаты — ни индивидуальные, ни в группах.
            Запланированные индивидуальные уроки отменятся, а их серия продлится на столько же.
            Расписание групп не меняется.
          </p>

          <div className="flex justify-end gap-2">
            <Button type="button" variant="outline" onClick={onClose}>Отмена</Button>
            <Button type="button" onClick={submit} disabled={invalid || createPause.isPending}>
              {createPause.isPending ? 'Сохранение...' : 'Заморозить'}
            </Button>
          </div>
        </div>
      </DialogContent>
    </Dialog>
  )
}
```

- [ ] **Step 5: Карточка ученика**

In `frontend/src/app/(dashboard)/students/[id]/page.tsx`:

1. Imports: `Snowflake`, `Wallet` from `lucide-react`; `useCreatePayment` from `@/lib/hooks/usePayments`; `usePauses`, `useDeletePause` from `@/lib/hooks/useStudents`; `PaymentForm` from `@/components/payments/PaymentForm`; `BulkPaymentDialog`; `PauseDialog`; `PaymentFormValues` from `@/schemas/payment`; type `StudentPause`.
2. Hooks next to the existing ones (before the early returns):
```tsx
  const { data: pauses = [] } = usePauses(id)
  const deletePause = useDeletePause(id)
  const [paymentOpen, setPaymentOpen] = useState(false)
  const [pauseOpen, setPauseOpen]     = useState(false)
  // Один курс — прежняя простая форма; разбивка по предметам — только с двух
  // курсов: большинство учеников на одном предмете (спека 2026-09-06, п. 6.8).
  const single        = courses.length === 1 ? courses[0] : undefined
  const createPayment = useCreatePayment(single?.id ?? '')
```
3. Handlers:
```tsx
  async function handleSinglePayment(values: PaymentFormValues) {
    if (!single) return
    await createPayment.mutateAsync({
      course_id:     single.id,
      student_id:    id,
      amount:        values.amount,
      lessons_count: values.lessons_count,
      paid_at:       values.paid_at,
    })
    toast.success('Оплата записана')
  }

  async function handleUnpause(p: StudentPause) {
    if (!confirm('Разморозить? Уроки этого периода снова спишутся с оплаты. Отменённые уроки и продлённая серия останутся как есть.')) return
    try {
      await deletePause.mutateAsync(p.id)
      toast.success('Заморозка снята')
    } catch {
      toast.error('Не удалось снять заморозку')
    }
  }
```
(`PaymentForm` itself catches the submit error and shows `toast.error`.)
4. Header actions — before «Редактировать», only for an active student:
```tsx
            {student.active && courses.length > 0 && (
              <Button size="sm" onClick={() => setPaymentOpen(true)}>
                <Wallet className="h-4 w-4 mr-1.5" /> Оплата
              </Button>
            )}
            {student.active && (
              <Button size="sm" variant="outline" onClick={() => setPauseOpen(true)}>
                <Snowflake className="h-4 w-4 mr-1.5" /> Заморозить
              </Button>
            )}
```
5. After the «Курсы» card — the pauses card (shown for archived students too: it is history):
```tsx
      <div className="border rounded-xl bg-card p-4 mt-4">
        <h2 className="text-sm font-semibold mb-3">Заморозки</h2>
        {pauses.length === 0 ? (
          <p className="text-sm text-muted-foreground">Не замораживался</p>
        ) : (
          <div className="space-y-1">
            {pauses.map((p) => (
              <div key={p.id} className="flex items-center justify-between py-2 border-b last:border-0 text-sm group">
                <span>
                  {fmtDay(p.starts_on)} — {fmtDay(p.ends_on)}
                  {p.reason && <span className="text-muted-foreground"> · {p.reason}</span>}
                </span>
                <Button
                  size="icon"
                  variant="ghost"
                  aria-label="Разморозить"
                  className="h-7 w-7 text-destructive hover:text-destructive opacity-0 group-hover:opacity-100 focus-visible:opacity-100"
                  onClick={() => handleUnpause(p)}
                >
                  <Trash2 className="h-3.5 w-3.5" />
                </Button>
              </div>
            ))}
          </div>
        )}
      </div>
```
with a module-level helper
```tsx
/** Даты паузы приходят полночью UTC — показываем в UTC, иначе на западе от
 *  Гринвича день уехал бы на вчера. */
const fmtDay = (iso: string) => new Date(iso).toLocaleDateString('ru-RU', { timeZone: 'UTC' })
```
6. Dialogs next to `StudentForm`:
```tsx
      {single && (
        <PaymentForm
          open={paymentOpen}
          onClose={() => setPaymentOpen(false)}
          onSubmit={handleSinglePayment}
          pricePerLesson={single.price_per_cycle / single.lessons_per_cycle}
          lessonsPerCycle={single.lessons_per_cycle}
        />
      )}
      {courses.length >= 2 && (
        <BulkPaymentDialog
          open={paymentOpen}
          onClose={() => setPaymentOpen(false)}
          student={student}
          courses={courses}
        />
      )}
      <PauseDialog
        open={pauseOpen}
        onClose={() => setPauseOpen(false)}
        studentId={id}
        studentName={student.last_name ? `${student.first_name} ${student.last_name}` : student.first_name}
      />
```

- [ ] **Step 6: Проверки и коммит**

Run: `cd /home/dragonbrn/tutorgo/frontend && npm run lint && npx tsc --noEmit && node --test src/lib/money.test.ts`
Expected: без новых ошибок lint/tsc относительно состояния после Task 7; 3 теста проходят.

```bash
cd /home/dragonbrn/tutorgo
git add frontend/src/lib/money.ts frontend/src/lib/money.test.ts frontend/src/lib/api/students.ts frontend/src/lib/hooks/useStudents.ts frontend/src/components/payments/BulkPaymentDialog.tsx frontend/src/components/students/PauseDialog.tsx "frontend/src/app/(dashboard)/students/[id]/page.tsx"
git commit -m "feat(frontend): оплата на несколько предметов и заморозка на карточке ученика

Co-Authored-By: Claude Opus 5 <noreply@anthropic.com>
Claude-Session: https://claude.ai/code/session_01KLiSrLKjmzZMQgDzgsUdbn"
```

---

### Task 9: Полная проверка, API-smoke, статус спеки

**Files:**
- Modify: `docs/specs/2026-09-06-price-units-and-student-centric-money.md` (строка «Статус», заметки о реализации в п. 6.7 и 6.9)
- Scratch (не коммитить): `/tmp/claude-1000/smoke-phase2.sh`, `/tmp/claude-1000/smoke-phase2.sql`

**Interfaces:**
- Consumes: всё из Tasks 1–8, «Контракт API фазы».

- [ ] **Step 1: Бэкенд целиком**

Run:
```bash
cd /home/dragonbrn/tutorgo
go build ./... && go vet ./... && go vet -tags=integration ./repository/
make test
TEST_DB_URL="postgres://dev:dev@localhost:5432/tutorgo_dev?sslmode=disable" go test -tags=integration -count=1 ./...
```
Expected: PASS, кроме известного `TestGetAllByTutor_CarriesSubjectAndStudentName`.

- [ ] **Step 2: Миграция 039 — вниз и вверх на тестовой БД**

Run:
```bash
export TEST_DB_URL="postgres://dev:dev@localhost:5432/tutorgo_dev?sslmode=disable"
goose -dir migrations postgres "$TEST_DB_URL" down && goose -dir migrations postgres "$TEST_DB_URL" up && goose -dir migrations postgres "$TEST_DB_URL" status | tail -2
```
Expected: `039_payments_student.sql` применена последней.

- [ ] **Step 3: Фронт целиком**

Run: `cd /home/dragonbrn/tutorgo/frontend && npm run lint && npx tsc --noEmit && node --test src/lib/money.test.ts`
Expected: чисто (или только ошибки, существовавшие до фазы, — сверить с базовой линией из отчёта Task 7), 3 теста проходят.

- [ ] **Step 4: API-smoke на локальном бэкенде**

Поднять API на тестовой БД в фоне и запомнить PID (`pkill -f` не использовать — убивает собственную оболочку):
```bash
cd /home/dragonbrn/tutorgo
env DB_URL="postgres://dev:dev@localhost:5432/tutorgo_dev?sslmode=disable" SERVER_PORT=:8099 JWT_SECRET=x ROLE=api \
  REDIS_URL= S3_ENDPOINT= RESEND_API_KEY= LIVEKIT_URL= go run . > /tmp/claude-1000/smoke-api.log 2>&1 &
echo $! > /tmp/claude-1000/smoke-api.pid
```
Дождаться `curl -s localhost:8099/public/lessons/00000000-0000-0000-0000-000000000000/room-status` (любой ответ HTTP = сервер жив).

Засеять `/tmp/claude-1000/smoke-phase2.sql` через psql: репетитор с фиксированным UUID, строка `subscriptions` с `period_end > now()` (колонки — по миграциям `migrations/*subscription*`), ученики и курсы под сценарии ниже. Токен репетитора — HS256 с секретом `x`, claim `id` = UUID репетитора:
```bash
TOKEN=$(node -e '
const c=require("crypto"),b=o=>Buffer.from(JSON.stringify(o)).toString("base64url");
const h=b({alg:"HS256",typ:"JWT"}),p=b({id:process.argv[1],exp:Math.floor(Date.now()/1000)+3600});
console.log(h+"."+p+"."+c.createHmac("sha256","x").update(h+"."+p).digest("base64url"))' "<tutor-uuid>")
```
(если `middleware.Auth` требует других claims — сверить с `middleware/auth.go` и добавить).

Сценарии — каждый проверить HTTP-запросом, в отчёт записать запрос, код ответа и ключевые поля:

| # | Сценарий | Ожидание |
|---|---|---|
| 1 | `POST /payments` на ученика, не записанного в группу | `400` |
| 2 | Группа из трёх, каждый оплатил 4 (`POST /payments`), 2 урока `completed`, одному `absent` на первом | `GET /payments/balance` у всех: `lessons_paid 4, lessons_completed 2, lessons_remaining 2` |
| 3 | Ученик записан в группу после третьего проведённого урока (`enrolled_at` сидом) | баланс `0/0/0` |
| 4 | `GET /payments/balance?course_id=` без `student_id` | `400` |
| 5 | Ученик с двумя индивидуальными курсами: `POST /payments/bulk`, вторая строка — чужой курс | `400`, в `GET /payments?course_id=` первого курса платежа нет |
| 6 | Тот же ученик: `POST /payments/bulk` с двумя валидными строками | `201`, две строки с одинаковым `paid_at`; `GET /payments/monthly-income` вырос на сумму строк |
| 7 | Ученик с проведёнными неоплаченными уроками по двум курсам | `GET /payments/debts`: одна строка ученика, `courses` из двух элементов, `amount_owed` = Σ уроки × `ROUND(price/n)` |
| 8 | Группа из пяти по 5000 за урок, один урок в текущем месяце | `GET /payments/monthly-expected` ≥ 25000 и растёт на 5000 × число участников |
| 9 | Легаси-платёж группы (`student_id NULL`, вставлен сидом) | в `GET /payments?course_id=` с `student_id: null`, в балансах участников не учтён; `PUT /payments/:id` с участником — `200`, с чужим — `400` |
| 10 | Убранный из группы с долгом (`DELETE /courses/:id/enrollments/:studentId`) | остаётся в `/payments/debts`, долг не растёт от урока после ухода; `POST /payments` на него — `201` |
| 11 | `POST /students/:id/pauses` на две недели вперёд у ученика с индивидуальной серией | `201`; уроки периода `cancelled`; у правила `ends_on` сдвинут на 14 дней (psql); `GET /students/:id/pauses` содержит паузу |
| 12 | Пауза задним числом на неделю с проведённым уроком | статус урока не меняется, `lessons_completed` в балансе уменьшился |
| 13 | `DELETE /students/:id/pauses/:pauseId` | `204`, урок сценария 12 снова в `lessons_completed` |
| 14 | `POST /students/:id/pauses` с `ends_on < starts_on` | `400` |
| 15 | `DELETE /students/:id` у участника группы с адресным платежом без отметок | `409` |

Остановить сервер: `kill $(cat /tmp/claude-1000/smoke-api.pid)`; если `go run` оставил дочерний бинарь на `:8099` — `lsof -ti :8099 | xargs -r kill`. Удалить сид: `DELETE FROM tutors WHERE id = '<tutor-uuid>'`.

Любое расхождение с ожиданием — не чинить, а записать в отчёт со статусом DONE_WITH_CONCERNS.

- [ ] **Step 5: Статус спеки и заметки о реализации**

In `docs/specs/2026-09-06-price-units-and-student-centric-money.md`:

1. Строка 4 → `**Статус:** фазы 0, 1 и 1.5 в main (фаза 1.5 — 2026-09-14, миграция 038 накатана до деплоя); фаза 2 реализована на ветке `feat/money-on-student` (2026-09-15, план `docs/superpowers/plans/2026-09-15-money-on-student.md`), ждёт ревью, ручной smoke и мерж; фазы 3–4 к реализации`.

2. В конец п. 6.7 (перед `### 6.8`) добавить абзац:
```markdown
**Реализация (2026-09-15).** Правило «цикл только там, где известен ученик» распространено на «текущие циклы» дашборда (`GetAllLessonsForCycles` — только индивидуальные курсы) и на кабинет ученика: ранги там считаются по периодам участия ученика, платежи — только его (`GetByStudentBatch`). Все четыре денежных места строятся от `studentCoursePairs` и `lessonInParticipation` в `repository/payment.go`.
```

3. В конец п. 6.9 (перед `**Критерии приёмки**`) добавить абзац:
```markdown
**Реализация (2026-09-15)** отличается от описания выше в трёх местах. Метода `ShiftTail` нет: пауза, отмена уроков и сдвиг хвоста — один SQL-оператор с data-modifying CTE (`repository/pause.go`), атомарный целиком. Сдвинутому правилу `materialized_until` откатывается к началу паузы — без этого `Materialize`, идущий от `materialized_until`, хвост за старым концом не создал бы, и ExtendAll тоже. Сервис материализует сдвинутые правила сразу; сбой лечит ночная ExtendAll. Ceiling: вхождения дальше горизонта материализации (6 месяцев) к моменту заморозки ещё не существуют и потом создадутся запланированными.
```

- [ ] **Step 6: Commit**

```bash
cd /home/dragonbrn/tutorgo
git add docs/specs/2026-09-06-price-units-and-student-centric-money.md
git commit -m "docs(specs): фаза 2 реализована — статус и заметки о реализации

Co-Authored-By: Claude Opus 5 <noreply@anthropic.com>
Claude-Session: https://claude.ai/code/session_01KLiSrLKjmzZMQgDzgsUdbn"
```

## Деплой (после мержа, за человеком)

- Backend и frontend — одновременно: `GET /payments/balance` без `student_id` отвечает `400`, `POST /payments` без `student_id` — `400`; старый фронт сломается на странице курса и в форме оплаты.
- Миграция 039 накатывается API при старте (вкомпилированные goose-миграции); ручной `make migrate-up` — только чтобы поймать ошибку до деплоя.
- В релизных заметках: прогноз «ожидается» у тьюторов с группами вырастет кратно составу — это починка, а не баг (спека, п. 6.6). До разметки легаси-платежей групп (5 строк на проде) их участники покажутся должниками.
- После разметки легаси-платежей тьютором — один раз руками уточняющий `UPDATE course_enrollments … enrolled_at` из п. 6.1 спеки.
- Долг ушедшего из группы ученика (`/payments/debts`) не гасится повторной записью в ту же группу: `enrollmentRepository.Add` не сбрасывает `enrolled_at`, а `Remove` при следующем уходе поставит новый `left_at` — интервал между уходами тоже засчитается в сгоревшее, и долг вырастет вместо того чтобы погаситься. Пока такого ученика нет в UI, где ему можно принять оплату (обе формы платежа отдают только активных/не-ушедших участников), — фиксировать оплату можно только напрямую через `POST /payments` с его `student_id` (curl/Postman), это известное ограничение, продукт для него ещё не сделан.

