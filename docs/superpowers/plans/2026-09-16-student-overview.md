# Фаза 3 — карточка ученика как рабочее место: план реализации

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** открытие карточки ученика — один запрос (`GET /students/:id/overview`), а не шесть; на карточке видно баланс и долг по каждому предмету, ближайший и последние уроки, последние оплаты — не уходя на страницу курса; оплата предлагает и курсы, с которых ученик ушёл, но остался должен.

**Architecture:** новый агрегирующий метод `studentService.Overview` компонует уже существующие узкие зависимости (`studentCourses`, `paymentRepo`, новый `debtsSource`) — без новых репозиториев, без миграций. Балансы всех курсов ученика — один batch-запрос (`GetBalancesByStudent`), построенный на тех же `studentCoursePairs`/`lessonInParticipation`, что баланс, долги и прогноз фазы 2. Фронт получает всё для вкладки «Обзор» одним хуком; вкладки «Уроки», «Оплаты», «ДЗ и материалы» — самостоятельные ленивые запросы по существующим ручкам, включаются только при открытии вкладки (см. «Почему не всё агрегируем» ниже).

**Tech Stack:** Go + Gin + pgx/v5, testify/mock, интеграционные тесты с build-тегом `integration`; фронт — Next.js App Router + React Query + shadcn `Tabs`.

**Spec:** `docs/specs/2026-09-06-price-units-and-student-centric-money.md`, раздел **7** (фаза 3), особенно п. 7.0 (известный пробел с ушедшими из группы) и п. 7.1–7.3.

## Global Constraints

- Правки файлов — **только через Edit/Write**, не через `sed`/`python`/heredoc в Bash (CLAUDE.md).
- **Никогда не запускать `make migrate-up` / `make migrate-down` / `make migrate-status`**: бьют в прод. У этой фазы миграций нет вообще.
- Тестовая БД — `TEST_DB_URL="postgres://dev:dev@localhost:5432/tutorgo_dev?sslmode=disable"`. Интеграционный прогон: `TEST_DB_URL=... go test -tags=integration -count=1 ./repository/ -run '<regex>' -v`.
- Известное красное, не связанное с фазой: `TestGetAllByTutor_CarriesSubjectAndStudentName`. Любое другое падение — стоп.
- Ветка `feat/student-overview`.
- Каждый коммит заканчивается строками:
  ```
  Co-Authored-By: Claude Sonnet 5 <noreply@anthropic.com>
  Claude-Session: https://claude.ai/code/session_01P2LhhXJjYViPqRtttuo9cB
  ```
- Комментарии в коде — по-русски, объясняют «почему», ссылаются на спеку как `(спека, п. 7.N)`.
- Главный footgun проекта: расширил интерфейс — обнови мок в том же коммите. Сверка: `go vet ./...` и `go vet -tags=integration ./repository/`.
- Денежные правила (спека, п. 3.6, 3.9) не меняются в этой фазе — только читаются существующими методами.
- Фронт: тест-раннера нет; проверка — `cd frontend && npm run lint`. Ошибки axios — `ApiError { message, status }`. Весь текст интерфейса — по-русски.
- `courseRepository.GetByStudent`, `paymentRepo.GetBalance`, `paymentService.GetDebts`, `studentService.ListLessons`, `paymentRepo.GetByStudentBatch` — **не трогать сигнатуры**, только читать. Фаза 3 ничего не меняет в деньгах фазы 2, только компонует их для одного экрана.

### Почему не всё агрегируем в `/overview`

Спека (п. 7.1) требует убрать шесть round-trip'ов **на открытие экрана**. Открытие экрана — это вкладка «Обзор» (она же вкладка по умолчанию). Вкладки «Уроки», «Оплаты», «ДЗ и материалы» переключаются кликом, не при первом рендере: каждая лениво (`enabled: activeTab === '...'`) вызывает уже существующие per-course ручки (`GET /payments?course_id=`, `GET /lessons?course_id=&from=&to=`, `GET /courses/:id/homework`), по разу на каждый из 1–3 курсов ученика. Это на порядок меньше «шести на открытие», не требует новых ручек и не тащит в один ответ данные, которые 90% открытий карточки не увидят вообще (ученик приходит посмотреть баланс перед уроком, не читать ДЗ месячной давности).

## Контракт API фазы (фронт опирается только на него)

| Метод и путь | Ответ |
|---|---|
| `GET /students/:id/overview` | `StudentOverview` (ниже); 404, если ученик не тьютора |

```go
// models/student.go — добавляется в конец файла. Файл сегодня не импортирует
// "time" — добавить в блок import.

// StudentCourseSummary — строка предмета на карточке ученика: цена и баланс
// одним запросом, без похода на страницу курса (спека, п. 7.1). StartedAt и
// EndedAt нужны инлайн-правке цены на карточке ученика (п. 7.2, CoursePrice):
// UpdateCourseRequest.StartedAt обязателен (models/course.go:62), и без него
// сохранить цену будет нечем.
type StudentCourseSummary struct {
	CourseID        string        `json:"course_id"`
	Subject         string        `json:"subject"`
	IsGroup         bool          `json:"is_group"`
	PricePerCycle   float64       `json:"price_per_cycle"`
	LessonsPerCycle int           `json:"lessons_per_cycle"`
	StartedAt       time.Time     `json:"started_at"`
	EndedAt         *time.Time    `json:"ended_at"`
	Balance         CourseBalance `json:"balance"`
}

// StudentOverview — карточка ученика одним запросом вместо шести (спека,
// п. 7.1). Courses — только активные предметы, для отображения. PayableCourses
// — те же плюс архивные/ушедшие-из-группы с положительным долгом: форма
// оплаты обязана предложить и их, иначе такой долг невозможно погасить
// (спека, п. 7.0).
type StudentOverview struct {
	Student        Student          `json:"student"`
	Courses        []StudentCourseSummary `json:"courses"`
	PayableCourses []Course         `json:"payable_courses"`
	NextLesson     *CalendarLesson  `json:"next_lesson"`
	RecentLessons  []CalendarLesson `json:"recent_lessons"`
	Payments       []Payment        `json:"payments"`
	TotalOwed      float64          `json:"total_owed"`
}
```

```ts
// frontend/src/types/api.ts
export interface StudentCourseSummary {
  course_id:         string
  subject:           string
  is_group:          boolean
  price_per_cycle:   number
  lessons_per_cycle: number
  started_at:        string
  ended_at:          string | null
  balance:           CourseBalance
}

export interface StudentOverview {
  student:         Student
  courses:         StudentCourseSummary[]
  payable_courses: Course[]
  next_lesson:     CalendarLesson | null
  recent_lessons:  CalendarLesson[]
  payments:        Payment[]
  total_owed:      number
}
```

## Карта файлов

| Файл | Задачи |
|---|---|
| `repository/payment.go` | 1 |
| `repository/balance_integration_test.go` | 1 |
| `models/student.go` | 2 |
| `service/student.go` | 3 |
| `service/student_test.go` | 3 |
| `handlers/student.go` | 4 |
| `handlers/student_test.go` | 4 |
| `router/router.go` | 4 |
| `frontend/src/types/api.ts` | 5 |
| `frontend/src/lib/api/students.ts` | 5 |
| `frontend/src/lib/hooks/useStudents.ts` | 5 |
| `frontend/src/lib/hooks/usePayments.ts` | 5 |
| `frontend/src/lib/hooks/useLessons.ts` | 5 |
| `frontend/src/components/students/CourseBalanceStat.tsx` | 6 (создать) |
| `frontend/src/app/(dashboard)/courses/[id]/page.tsx` | 6 |
| `frontend/src/app/(dashboard)/students/[id]/page.tsx` | 7, 8 |
| `frontend/src/components/students/StudentLessonsTab.tsx` | 8 (создать) |
| `frontend/src/components/students/StudentPaymentsTab.tsx` | 8 (создать) |
| `frontend/src/components/students/StudentHomeworkTab.tsx` | 8 (создать) |
| `frontend/src/components/homework/HomeworkEditPopover.tsx` | 8 (экспортировать `HomeworkForm`) |

---

## Task 1: `GetBalancesByStudent` — батч-баланс всех курсов ученика

**Files:**
- Modify: `repository/payment.go:12-28` (интерфейс), после `GetBalance` (строка 363)
- Test: `repository/balance_integration_test.go`

**Interfaces:**
- Produces: `PaymentRepository.GetBalancesByStudent(ctx, studentID, tutorID string) (map[string]models.CourseBalance, error)` — ключ карты `course_id`; курса нет в карте, если пары «курс+ученик» с этим `tutor_id` не существует.

- [ ] **Шаг 1: добавить метод в интерфейс**

`repository/payment.go`, в `PaymentRepository` сразу после строки 21 (`GetBalance(...)`):

```go
	// GetBalancesByStudent — баланс сразу по всем курсам ученика (спека, п. 7.1):
	// открытие карточки не должно бить в БД по разу на курс.
	GetBalancesByStudent(ctx context.Context, studentID, tutorID string) (map[string]models.CourseBalance, error)
```

- [ ] **Шаг 2: реализация**

После `GetBalance` (после строки 363, перед `func (r *paymentRepository) Update`):

```go
// GetBalancesByStudent — то же самое, что GetBalance, но одним запросом по
// всем парам «курс + ученик» этого тьютора (спека, п. 7.1). c.tutor_id — та же
// защита, что в GetByStudent: studentCoursePairs сам по себе tutor_id не
// фильтрует.
func (r *paymentRepository) GetBalancesByStudent(ctx context.Context, studentID, tutorID string) (map[string]models.CourseBalance, error) {
	rows, err := r.conn.Query(ctx,
		`WITH sc AS (
		     SELECT pr.* FROM (`+studentCoursePairs+`) pr
		     JOIN courses c ON c.id = pr.course_id
		     WHERE pr.student_id = $1 AND c.tutor_id = $2
		 )
		 SELECT sc.course_id,
		     COALESCE((SELECT SUM(lessons_count) FROM payments
		                WHERE course_id = sc.course_id AND student_id = $1), 0),
		     (SELECT count(*) FROM lessons l
		       WHERE l.course_id = sc.course_id
		         AND l.status IN ('completed', 'missed')
		         AND `+lessonInParticipation+`)
		 FROM sc`,
		studentID, tutorID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	balances := map[string]models.CourseBalance{}
	for rows.Next() {
		var courseID string
		var paid, burned int
		if err := rows.Scan(&courseID, &paid, &burned); err != nil {
			return nil, err
		}
		balances[courseID] = models.CourseBalance{
			LessonsPaid:      paid,
			LessonsCompleted: burned,
			LessonsRemaining: paid - burned,
		}
	}
	return balances, rows.Err()
}
```

- [ ] **Шаг 3: интеграционный тест**

Добавить в конец `repository/balance_integration_test.go`:

```go
// Один batch-запрос вместо N по числу курсов (спека, п. 7.1): математика и
// физика ученика — разные балансы одним вызовом.
func TestGetBalancesByStudent_ReturnsAllCoursesInOneQuery(t *testing.T) {
	pool := testPool(t)
	tutorID, s := seedTutorStudent(t, pool)
	math := addIndividualCourse(t, pool, tutorID, s)
	var physics string
	require.NoError(t, pool.QueryRow(context.Background(),
		`INSERT INTO courses (student_id, tutor_id, subject, price_per_cycle, lessons_per_cycle, started_at)
		 VALUES ($1, $2, 'Физика', 6000, 1, NOW() - interval '1 month') RETURNING id`,
		s, tutorID).Scan(&physics))
	addLessonAt(t, pool, math, "NOW() - interval '1 day'", "completed")
	payFor(t, pool, math, s, 8)
	addLessonAt(t, pool, physics, "NOW() - interval '1 day'", "completed")

	balances, err := repository.NewPaymentRepository(pool).GetBalancesByStudent(context.Background(), s, tutorID)

	require.NoError(t, err)
	assert.Equal(t, models.CourseBalance{LessonsPaid: 8, LessonsCompleted: 1, LessonsRemaining: 7}, balances[math])
	assert.Equal(t, models.CourseBalance{LessonsPaid: 0, LessonsCompleted: 1, LessonsRemaining: -1}, balances[physics])
}

// Чужой тьютор не видит пару «курс + ученик» — карта пуста, а не паника или
// чужие деньги.
func TestGetBalancesByStudent_ScopedToTutor(t *testing.T) {
	pool := testPool(t)
	tutorID, s := seedTutorStudent(t, pool)
	addIndividualCourse(t, pool, tutorID, s)
	otherTutorID, _ := seedTutorStudent(t, pool)

	balances, err := repository.NewPaymentRepository(pool).GetBalancesByStudent(context.Background(), s, otherTutorID)

	require.NoError(t, err)
	assert.Empty(t, balances)
}
```

- [ ] **Шаг 4: прогнать**

`TEST_DB_URL="postgres://dev:dev@localhost:5432/tutorgo_dev?sslmode=disable" go test -tags=integration -count=1 ./repository/ -run TestGetBalancesByStudent -v`
Ожидается: PASS, 2 теста.

- [ ] **Шаг 5: коммит**

```bash
git add repository/payment.go repository/balance_integration_test.go
git commit -m "$(cat <<'EOF'
feat(payments): батч-баланс всех курсов ученика одним запросом

GetBalancesByStudent — основа агрегирующего /students/:id/overview фазы 3:
карточка ученика не должна бить в БД по разу на курс (спека, п. 7.1).

Co-Authored-By: Claude Sonnet 5 <noreply@anthropic.com>
Claude-Session: https://claude.ai/code/session_01P2LhhXJjYViPqRtttuo9cB
EOF
)"
```

---

## Task 2: Модели `StudentCourseSummary` / `StudentOverview`

**Files:**
- Modify: `models/student.go` (добавить в конец файла)

**Interfaces:**
- Produces: `models.StudentCourseSummary`, `models.StudentOverview` — см. «Контракт API фазы» выше, точный код оттуда.

- [ ] **Шаг 1: добавить типы**

Вставить в конец `models/student.go` (после `StudentChangePasswordRequest`) ровно блок `StudentCourseSummary`/`StudentOverview` из раздела «Контракт API фазы» этого плана. Добавить `"time"` в блок `import` файла — сейчас его там нет, а `StartedAt time.Time` этого требует.

- [ ] **Шаг 2: убедиться, что пакет компилируется**

`go build ./models/...`
Ожидается: без ошибок.

- [ ] **Шаг 3: коммит**

```bash
git add models/student.go
git commit -m "$(cat <<'EOF'
feat(students): модели StudentOverview и StudentCourseSummary

Ответ агрегирующего /students/:id/overview (спека, п. 7.1).

Co-Authored-By: Claude Sonnet 5 <noreply@anthropic.com>
Claude-Session: https://claude.ai/code/session_01P2LhhXJjYViPqRtttuo9cB
EOF
)"
```

---

## Task 3: `studentService.Overview`

**Files:**
- Modify: `service/student.go`
- Test: `service/student_test.go`

**Interfaces:**
- Consumes: `repository.StudentRepository.GetByID`, `studentCourses.GetByStudent`/`GetByID` (новый метод в узком интерфейсе), `repository.PaymentRepository.GetBalancesByStudent`/`GetByStudentBatch` (уже в `s.paymentRepo`), новый узкий интерфейс `debtsSource.GetDebts`, существующий self-метод `s.ListLessons`.
- Produces: `StudentService.Overview(ctx, id, tutorID string) (models.StudentOverview, error)`.

### Почему `debtsSource`, а не прямой вызов `paymentService`

`studentService` уже держит `paymentRepo repository.PaymentRepository` — прямого доступа к бизнес-логике `paymentService.GetDebts` (группировка по ученику, `NextLessonAt`) у него нет и заново её тут писать нельзя (спека, п. 7.1: «не пересчитанный заново SQL»). Узкий интерфейс — тот же приём, что `studentCourses` (комментарий в коде: «в проде это courseService, а не courseRepo»): `paymentService` уже реализует ровно эту сигнатуру, роутер прокидывает его как есть, тесты подставляют мок.

- [ ] **Шаг 1: расширить `studentCourses`, добавить `debtsSource`**

`service/student.go`, заменить блок интерфейсов (строки 14–28):

```go
// studentCourses — курсы ученика, их архивация и точечный доступ по ID. В
// проде это courseService, а не courseRepo: у репозитория тот же Delete, но
// без закрытия правил и удаления будущих уроков — архивный курс продолжал бы
// материализовать расписание. GetByID нужен Overview (спека, п. 7.0): курс,
// с которого ученик ушёл или который архивирован, GetByStudent не отдаёт, а
// оплатить долг по нему всё равно нужно.
type studentCourses interface {
	GetByStudent(ctx context.Context, studentID string, tutorID string) ([]models.Course, error)
	GetByID(ctx context.Context, id string, tutorID string) (models.Course, error)
	Delete(ctx context.Context, id string, tutorID string) error
}

type enrollmentLeaver interface {
	LeaveAllByStudent(ctx context.Context, studentID string) error
}

type studentSessions interface {
	DeleteByStudentID(ctx context.Context, studentID string) error
}

// debtsSource — долг ученика без пересчёта денежного SQL (спека, п. 7.1): то,
// что уже возвращает paymentService.GetDebts, отфильтрованное по student_id.
type debtsSource interface {
	GetDebts(ctx context.Context, tutorID string) ([]models.StudentDebt, error)
}
```

- [ ] **Шаг 2: добавить `Overview` в интерфейс, поле и конструктор**

В `StudentService` (после `ListCourses`, строка 49):

```go
	Overview(ctx context.Context, id string, tutorID string) (models.StudentOverview, error)
```

Заменить структуру и конструктор (строки 52–63):

```go
type studentService struct {
	repo        repository.StudentRepository
	paymentRepo repository.PaymentRepository
	courses     studentCourses
	enrollments enrollmentLeaver
	sessions    studentSessions
	debts       debtsSource
}

func NewStudentService(repo repository.StudentRepository, paymentRepo repository.PaymentRepository,
	courses studentCourses, enrollments enrollmentLeaver, sessions studentSessions, debts debtsSource) StudentService {
	return &studentService{repo: repo, paymentRepo: paymentRepo, courses: courses, enrollments: enrollments, sessions: sessions, debts: debts}
}
```

- [ ] **Шаг 3: реализация `Overview`**

Добавить в конец `service/student.go`, добавить `"sort"` в импорты:

```go
// Overview — карточка ученика одним запросом вместо шести (спека, п. 7.1).
// PayableCourses дополняет активные курсы архивными/ушедшими с положительным
// долгом: без этого оплатить такой долг не из чего (спека, п. 7.0).
//
// ponytail: s.ListLessons вызывается дважды (будущие и прошедшие) и каждый
// раз может сходить за paymentRepo.GetByStudentBatch для расчёта цикла —
// итого до трёх обращений к payments вместо одного. Всё это один API-вызов
// вместо шести с фронта, и запросы дешёвые (индекс по student_id); объединять
// с рассылкой Payments ниже, если профилирование покажет, что это заметно.
func (s *studentService) Overview(ctx context.Context, id, tutorID string) (models.StudentOverview, error) {
	student, err := s.repo.GetByID(ctx, id, tutorID)
	if err != nil {
		return models.StudentOverview{}, fmt.Errorf("student: %w", ErrNotFound)
	}

	courses, err := s.courses.GetByStudent(ctx, id, tutorID)
	if err != nil {
		return models.StudentOverview{}, err
	}

	balances, err := s.paymentRepo.GetBalancesByStudent(ctx, id, tutorID)
	if err != nil {
		return models.StudentOverview{}, err
	}

	summaries := make([]models.StudentCourseSummary, len(courses))
	for i, c := range courses {
		summaries[i] = models.StudentCourseSummary{
			CourseID:        c.ID,
			Subject:         c.Subject,
			IsGroup:         c.StudentID == nil,
			PricePerCycle:   c.PricePerCycle,
			LessonsPerCycle: c.LessonsPerCycle,
			StartedAt:       c.StartedAt,
			EndedAt:         c.EndedAt,
			Balance:         balances[c.ID],
		}
	}

	debts, err := s.debts.GetDebts(ctx, tutorID)
	if err != nil {
		return models.StudentOverview{}, err
	}
	var totalOwed float64
	var owedCourses []models.DebtByCourse
	for _, d := range debts {
		if d.StudentID == id {
			totalOwed = d.AmountOwed
			owedCourses = d.Courses
			break
		}
	}

	// Оплата обязана предложить и курс, с которого ученик ушёл или который
	// архивирован, если по нему остался долг (спека, п. 7.0) — иначе такой
	// долг невозможно погасить.
	payable := append([]models.Course{}, courses...)
	seen := make(map[string]bool, len(courses))
	for _, c := range courses {
		seen[c.ID] = true
	}
	for _, dc := range owedCourses {
		if seen[dc.CourseID] {
			continue
		}
		extra, err := s.courses.GetByID(ctx, dc.CourseID, tutorID)
		if err != nil {
			continue // не блокировать весь экран из-за одной осиротевшей строки долга
		}
		payable = append(payable, extra)
		seen[dc.CourseID] = true
	}

	upcoming, err := s.ListLessons(ctx, id, false)
	if err != nil {
		return models.StudentOverview{}, err
	}
	var nextLesson *models.CalendarLesson
	if len(upcoming) > 0 {
		nextLesson = &upcoming[0]
	}

	past, err := s.ListLessons(ctx, id, true)
	if err != nil {
		return models.StudentOverview{}, err
	}
	recent := past
	if len(recent) > 5 {
		recent = recent[:5]
	}

	paymentsByCourse, err := s.paymentRepo.GetByStudentBatch(ctx, id)
	if err != nil {
		return models.StudentOverview{}, err
	}
	subjectOf := make(map[string]string, len(payable))
	for _, c := range payable {
		subjectOf[c.ID] = c.Subject
	}
	var payments []models.Payment
	for courseID, ps := range paymentsByCourse {
		for _, p := range ps {
			p.Subject = subjectOf[courseID]
			payments = append(payments, p)
		}
	}
	sort.Slice(payments, func(i, j int) bool { return payments[i].PaidAt.After(payments[j].PaidAt) })
	if len(payments) > 5 {
		payments = payments[:5]
	}

	return models.StudentOverview{
		Student:        student,
		Courses:        summaries,
		PayableCourses: payable,
		NextLesson:     nextLesson,
		RecentLessons:  recent,
		Payments:       payments,
		TotalOwed:      totalOwed,
	}, nil
}
```

- [ ] **Шаг 4: обновить существующие вызовы `NewStudentService` в тестах**

`service/student_test.go` — во всех ~19 местах `service.NewStudentService(...)` дописать шестой аргумент `nil` (тип `debtsSource` не экспортирован, `nil` подходит везде, где `Overview` не тестируется). Пример замены (повторить для каждого вхождения в файле):

```go
// Было:
svc := service.NewStudentService(repo, new(mockPaymentRepo), nil, nil, nil)
// Стало:
svc := service.NewStudentService(repo, new(mockPaymentRepo), nil, nil, nil, nil)
```

и для строки с `courses`/`enrollments`/`sessions` (например, строка 405):

```go
// Было:
svc := service.NewStudentService(repo, new(mockPaymentRepo), courses, enrollments, sessions)
// Стало:
svc := service.NewStudentService(repo, new(mockPaymentRepo), courses, enrollments, sessions, nil)
```

`courses := new(mockCourseRepo)` уже реализует `GetByID` (используется в `service/course_test.go:35`) — новый метод в `studentCourses` не требует новых моков для существующих тестов.

- [ ] **Шаг 5: добавить `mockDebtsSource` и тесты `Overview`**

В конец `service/student_test.go`:

```go
type mockDebtsSource struct{ mock.Mock }

func (m *mockDebtsSource) GetDebts(ctx context.Context, tutorID string) ([]models.StudentDebt, error) {
	args := m.Called(ctx, tutorID)
	return args.Get(0).([]models.StudentDebt), args.Error(1)
}

// Обзор компонует существующие методы одним ответом: активные курсы с
// балансом, долг этого ученика из общего списка, ближайший и последние уроки,
// последние платежи (спека, п. 7.1).
func TestStudentOverview_ComposesExistingData(t *testing.T) {
	repo := new(mockStudentRepo)
	payRepo := new(mockPaymentRepo)
	courses := new(mockCourseRepo)
	debts := new(mockDebtsSource)
	svc := service.NewStudentService(repo, payRepo, courses, nil, nil, debts)

	studentID := "stu-1"
	course := models.Course{ID: "c1", StudentID: &studentID, Subject: "Математика", PricePerCycle: 40000, LessonsPerCycle: 8}
	repo.On("GetByID", mock.Anything, studentID, "tutor-1").Return(models.Student{ID: studentID, FirstName: "Айгерим"}, nil)
	courses.On("GetByStudent", mock.Anything, studentID, "tutor-1").Return([]models.Course{course}, nil)
	payRepo.On("GetBalancesByStudent", mock.Anything, studentID, "tutor-1").
		Return(map[string]models.CourseBalance{"c1": {LessonsPaid: 8, LessonsCompleted: 3, LessonsRemaining: 5}}, nil)
	debts.On("GetDebts", mock.Anything, "tutor-1").Return([]models.StudentDebt{
		{StudentID: studentID, AmountOwed: 12000, Courses: []models.DebtByCourse{{CourseID: "c1", AmountOwed: 12000}}},
	}, nil)
	repo.On("ListLessons", mock.Anything, studentID, false).Return([]models.CalendarLesson{}, nil)
	repo.On("ListLessons", mock.Anything, studentID, true).Return([]models.CalendarLesson{}, nil)
	payRepo.On("GetByStudentBatch", mock.Anything, studentID).Return(map[string][]models.Payment{}, nil)

	got, err := svc.Overview(context.Background(), studentID, "tutor-1")

	assert.NoError(t, err)
	assert.Equal(t, "Айгерим", got.Student.FirstName)
	assert.Len(t, got.Courses, 1)
	assert.Equal(t, 5, got.Courses[0].Balance.LessonsRemaining)
	assert.Equal(t, 12000.0, got.TotalOwed)
	assert.Len(t, got.PayableCourses, 1) // c1 уже активен, дублировать не должен
	repo.AssertExpectations(t)
	payRepo.AssertExpectations(t)
	courses.AssertExpectations(t)
	debts.AssertExpectations(t)
}

// Ушедший из группы с долгом: курс не в активных, но обязан попасть в
// PayableCourses — иначе долг невозможно погасить (спека, п. 7.0).
func TestStudentOverview_PayableCoursesIncludeLeftCourseWithDebt(t *testing.T) {
	repo := new(mockStudentRepo)
	payRepo := new(mockPaymentRepo)
	courses := new(mockCourseRepo)
	debts := new(mockDebtsSource)
	svc := service.NewStudentService(repo, payRepo, courses, nil, nil, debts)

	studentID := "stu-1"
	leftCourse := models.Course{ID: "c-left", Subject: "Группа"}
	repo.On("GetByID", mock.Anything, studentID, "tutor-1").Return(models.Student{ID: studentID}, nil)
	courses.On("GetByStudent", mock.Anything, studentID, "tutor-1").Return([]models.Course{}, nil) // активных нет
	payRepo.On("GetBalancesByStudent", mock.Anything, studentID, "tutor-1").Return(map[string]models.CourseBalance{}, nil)
	debts.On("GetDebts", mock.Anything, "tutor-1").Return([]models.StudentDebt{
		{StudentID: studentID, AmountOwed: 5000, Courses: []models.DebtByCourse{{CourseID: "c-left", AmountOwed: 5000}}},
	}, nil)
	courses.On("GetByID", mock.Anything, "c-left", "tutor-1").Return(leftCourse, nil)
	repo.On("ListLessons", mock.Anything, studentID, false).Return([]models.CalendarLesson{}, nil)
	repo.On("ListLessons", mock.Anything, studentID, true).Return([]models.CalendarLesson{}, nil)
	payRepo.On("GetByStudentBatch", mock.Anything, studentID).Return(map[string][]models.Payment{}, nil)

	got, err := svc.Overview(context.Background(), studentID, "tutor-1")

	assert.NoError(t, err)
	assert.Empty(t, got.Courses) // на «Обзоре» его нет
	require.Len(t, got.PayableCourses, 1)
	assert.Equal(t, "c-left", got.PayableCourses[0].ID) // но заплатить есть чем
}
```

`"github.com/stretchr/testify/require"` в `service/student_test.go` уже импортирован (используется в `TestStudentArchive_StepsInOrder` и др.) — ничего добавлять не нужно.

- [ ] **Шаг 6: прогнать unit-тесты**

`go test ./service/... -run TestStudentOverview -v`
Ожидается: PASS, 2 теста.

`go test ./service/... -v 2>&1 | tail -30` — весь пакет `service` зелёный (проверка, что дописанный `nil` не сломал прочие тесты студента).

- [ ] **Шаг 7: `go vet`**

`go vet ./...` — без ошибок (иначе где-то забыт шестой аргумент).

- [ ] **Шаг 8: коммит**

```bash
git add service/student.go service/student_test.go
git commit -m "$(cat <<'EOF'
feat(students): studentService.Overview — карточка ученика одним вызовом

Компонует курсы с балансом, долг, ближайший/последние уроки и последние
платежи; PayableCourses закрывает пробел фазы 2 — курс с долгом предлагается
к оплате, даже если ученик с него ушёл или он архивирован (спека, п. 7.0–7.1).

Co-Authored-By: Claude Sonnet 5 <noreply@anthropic.com>
Claude-Session: https://claude.ai/code/session_01P2LhhXJjYViPqRtttuo9cB
EOF
)"
```

---

## Task 4: Хендлер, роут, wiring

**Files:**
- Modify: `handlers/student.go`, `handlers/mocks_test.go`, `handlers/student_test.go`, `router/router.go`

**Interfaces:**
- Consumes: `service.StudentService.Overview`.
- Produces: `GET /students/:id/overview` → `200 models.StudentOverview` | `404`.

- [ ] **Шаг 1: хендлер**

`handlers/student.go`, добавить после `GetByID` (после строки 82, перед `Update`):

```go
// Overview — карточка ученика одним запросом вместо шести (спека, п. 7.1).
func (h *StudentHandler) Overview(c *gin.Context) {
	tutorID := c.GetString("tutorID")
	if tutorID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	overview, err := h.service.Overview(c.Request.Context(), c.Param("id"), tutorID)
	if err != nil {
		h.log.Error("Failed to get student overview", slog.String("error", err.Error()))
		handleServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, overview)
}
```

- [ ] **Шаг 2: мок сервиса**

`handlers/mocks_test.go`, в `mockStudentService` добавить после `GetByID` (после строки 128):

```go
func (m *mockStudentService) Overview(ctx context.Context, id string, tutorID string) (models.StudentOverview, error) {
	args := m.Called(ctx, id, tutorID)
	return args.Get(0).(models.StudentOverview), args.Error(1)
}
```

- [ ] **Шаг 3: тест хендлера**

`handlers/student_test.go` строит роутер общим хелпером `newStudentRouter` (строка 20) — дописать в него регистрацию нового маршрута, сразу после `r.GET("/students/:id", h.GetByID)`:

```go
	r.GET("/students/:id/overview", h.Overview)
```

Тесты — тем же паттерном, что `TestStudentGetByID_Success`/`TestStudentGetByID_NotFound` (строки 150–174), с реальными хелперами файла (`makeRequest`, `decodeJSON`, `testTutorID`, `testStudentID`, `testStudent` — все объявлены в `handlers/mocks_test.go`). Добавить после блока `// GetByID`:

```go
// Overview

func TestStudentOverview_Success(t *testing.T) {
	svc := new(mockStudentService)
	r := newStudentRouter(svc, testTutorID)

	overview := models.StudentOverview{Student: testStudent, TotalOwed: 5000}
	svc.On("Overview", mock.Anything, testStudentID, testTutorID).Return(overview, nil)

	w := makeRequest(t, r, http.MethodGet, "/students/"+testStudentID+"/overview", nil)

	assert.Equal(t, http.StatusOK, w.Code)
	var got models.StudentOverview
	decodeJSON(t, w, &got)
	assert.Equal(t, 5000.0, got.TotalOwed)
	svc.AssertExpectations(t)
}

func TestStudentOverview_NotFound(t *testing.T) {
	svc := new(mockStudentService)
	r := newStudentRouter(svc, testTutorID)

	svc.On("Overview", mock.Anything, testStudentID, testTutorID).
		Return(models.StudentOverview{}, fmt.Errorf("student: %w", service.ErrNotFound))

	w := makeRequest(t, r, http.MethodGet, "/students/"+testStudentID+"/overview", nil)

	assert.Equal(t, http.StatusNotFound, w.Code)
	svc.AssertExpectations(t)
}
```

- [ ] **Шаг 4: роут и wiring**

`router/router.go`, строка 63 — добавить шестой аргумент:

```go
// Было:
studentService := service.NewStudentService(studentRepo, paymentRepo, courseService, enrollmentRepo, studentRefreshRepo)
// Стало:
studentService := service.NewStudentService(studentRepo, paymentRepo, courseService, enrollmentRepo, studentRefreshRepo, paymentService)
```

(`paymentService` уже создан строкой 55 — выше по файлу, переставлять порядок не нужно.)

После строки 217 (`auth.GET("/students/:id", studentHandler.GetByID)`) добавить:

```go
		auth.GET("/students/:id/overview", studentHandler.Overview)
```

- [ ] **Шаг 5: прогнать**

`go build ./...` — компилируется.
`go test ./handlers/... -run TestStudentHandler_Overview -v` — PASS.
`go vet ./...` — чисто.

- [ ] **Шаг 6: коммит**

```bash
git add handlers/student.go handlers/mocks_test.go handlers/student_test.go router/router.go
git commit -m "$(cat <<'EOF'
feat(students): GET /students/:id/overview

Карточка ученика одним HTTP-запросом (спека, п. 7.1).

Co-Authored-By: Claude Sonnet 5 <noreply@anthropic.com>
Claude-Session: https://claude.ai/code/session_01P2LhhXJjYViPqRtttuo9cB
EOF
)"
```

---

## Task 5: Фронт — типы, API-клиент, хук, инвалидация

**Files:**
- Modify: `frontend/src/types/api.ts`, `frontend/src/lib/api/students.ts`, `frontend/src/lib/hooks/useStudents.ts`, `frontend/src/lib/hooks/usePayments.ts`, `frontend/src/lib/hooks/useLessons.ts`

**Interfaces:**
- Produces: `useStudentOverview(id)` — `useQuery<StudentOverview>`; `studentKeys.overview(id)`.

- [ ] **Шаг 1: типы**

Вставить в `frontend/src/types/api.ts` после `export interface StudentCourse { ... }` (после строки 107) ровно блок `StudentCourseSummary`/`StudentOverview` из раздела «Контракт API фазы» этого плана.

- [ ] **Шаг 2: API-клиент**

`frontend/src/lib/api/students.ts` — добавить импорт `StudentOverview` в строку 2 и метод в `studentsApi`:

```ts
import { Student, PagedResponse, OnboardingStudentInput, OnboardingResult, StudentPause, StudentOverview } from '@/types/api'
// ...
  overview: (id: string) => api.get<StudentOverview>(`/students/${id}/overview`).then((r) => r.data),
```

- [ ] **Шаг 3: хук + ключ**

`frontend/src/lib/hooks/useStudents.ts`:

```ts
// studentKeys, добавить:
  overview: (id: string) => ['students', id, 'overview'] as const,
```

```ts
// после useStudent:
export function useStudentOverview(id: string) {
  return useQuery({
    queryKey: studentKeys.overview(id),
    queryFn:  () => studentsApi.overview(id),
    enabled:  !!id,
  })
}
```

`studentKeys.overview(id)` = `['students', id, 'overview']` — React Query инвалидирует по префиксу, поэтому уже существующие `qc.invalidateQueries({ queryKey: studentKeys.all })` (в `useUpdateStudent`, `useDeleteStudent`, `useArchiveStudent`, `useRestoreStudent`) и `qc.invalidateQueries({ queryKey: courseKeys.all })` **не** зацепят `['students', id, 'overview']` — только `qc.invalidateQueries({ queryKey: studentKeys.detail(id) })` или `.overview(id)` зацепят. Явные добавления — следующие два шага.

- [ ] **Шаг 4: инвалидация после заморозки/платежей/уроков**

`frontend/src/lib/hooks/useStudents.ts`, `invalidateAfterPause` (строки 147–153) — добавить строку:

```ts
function invalidateAfterPause(qc: QueryClient, studentId: string) {
  qc.invalidateQueries({ queryKey: studentKeys.pauses(studentId) })
  qc.invalidateQueries({ queryKey: studentKeys.overview(studentId) })
  qc.invalidateQueries({ queryKey: ['payments'] })
  qc.invalidateQueries({ queryKey: ['courses'] })
  qc.invalidateQueries({ queryKey: ['lessons'] })
  qc.invalidateQueries({ queryKey: ['calendar'] })
}
```

`frontend/src/lib/hooks/usePayments.ts` — `invalidateMoney` принимает ID учеников тоже:

```ts
import { studentKeys } from '@/lib/hooks/useStudents'
// ...
function invalidateMoney(qc: QueryClient, courseIds: string[], studentIds: string[] = []) {
  qc.invalidateQueries({ queryKey: ['payments'] })
  for (const id of courseIds) qc.invalidateQueries({ queryKey: courseKeys.balance(id) })
  for (const id of studentIds) qc.invalidateQueries({ queryKey: studentKeys.overview(id) })
}
```

Обновить вызовы:

```ts
export function useCreatePayment(courseId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: paymentsApi.create,
    onSuccess:  (_data, input) => invalidateMoney(qc, [courseId], [input.student_id]),
  })
}

export function useCreateBulkPayment() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: paymentsApi.createBulk,
    onSuccess:  (_data, input) => invalidateMoney(
      qc,
      input.items.map((i) => i.course_id),
      [...new Set(input.items.map((i) => i.student_id))],
    ),
  })
}

export function useUpdatePayment(courseId?: string, studentId?: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ id, data }: { id: string; data: PaymentUpdateInput }) =>
      paymentsApi.update(id, data),
    onSuccess: () => invalidateMoney(qc, courseId ? [courseId] : [], studentId ? [studentId] : []),
  })
}

export function useDeletePayment(courseId?: string, studentId?: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: paymentsApi.delete,
    onSuccess:  () => invalidateMoney(qc, courseId ? [courseId] : [], studentId ? [studentId] : []),
  })
}
```

Проверить `CreatePaymentRequest`/`BulkPaymentItem` в `frontend/src/lib/api/payments.ts` действительно несут `student_id` в теле, которое получает `mutationFn` (по бэкенду — да, `models.CreatePaymentRequest.StudentID`/`models.BulkPaymentItem.StudentID` обязательны). Вызовы `useUpdatePayment(id)`/`useDeletePayment(id)` на странице курса (`courses/[id]/page.tsx`) не ломаются — новый параметр опциональный, там просто не будет инвалидации overview (там его и не может быть — это чужая страница).

`frontend/src/lib/hooks/useLessons.ts` — `invalidateLessonViews` тоже получает опциональный `studentId` тем же способом:

```ts
function invalidateLessonViews(qc: QueryClient, courseId: string, studentId?: string) {
  qc.invalidateQueries({ queryKey: lessonKeys.byCourse(courseId) })
  qc.invalidateQueries({ queryKey: ['calendar'] })
  if (studentId) qc.invalidateQueries({ queryKey: studentKeys.overview(studentId) })
}
```

Добавить `import { studentKeys } from '@/lib/hooks/useStudents'` и передавать `studentId` только там, где Task 8 создаёт урок со страницы ученика (`useCreateLesson(courseId, studentId?)`) — сигнатуры `useUpdateLesson`/`useDeleteLesson`, используемые исключительно на странице курса, не трогать без необходимости: там своего `studentId` в скоупе нет и оверью с той страницы не открыт.

- [ ] **Шаг 5: прогнать**

`cd frontend && npm run lint` — чисто (или без новых ошибок сверх существующих 22 `react-hooks` в доске, см. критерии приёмки спеки).
`grep -rn "useUpdatePayment(\|useDeletePayment(" frontend/src/app` — свериться, что вызовы на странице курса (`courses/[id]/page.tsx`) передают только `courseId`, без второго аргумента — компилируется, т.к. `studentId` опционален.

- [ ] **Шаг 6: коммит**

```bash
git add frontend/src/types/api.ts frontend/src/lib/api/students.ts frontend/src/lib/hooks/useStudents.ts frontend/src/lib/hooks/usePayments.ts frontend/src/lib/hooks/useLessons.ts
git commit -m "$(cat <<'EOF'
feat(students): хук useStudentOverview и инвалидация карточки ученика

Оплата, заморозка и создание урока со страницы ученика теперь сбрасывают её
агрегированные данные, а не только списки курса (спека, п. 7.1).

Co-Authored-By: Claude Sonnet 5 <noreply@anthropic.com>
Claude-Session: https://claude.ai/code/session_01P2LhhXJjYViPqRtttuo9cB
EOF
)"
```

---

## Task 6: Вынести `CourseBalanceStat` — общий компонент баланса

**Files:**
- Create: `frontend/src/components/students/CourseBalanceStat.tsx`
- Modify: `frontend/src/app/(dashboard)/courses/[id]/page.tsx:333-353`

**Interfaces:**
- Produces: `<CourseBalanceStat balance={CourseBalance} />` — три числа «Оплачено / Проведено / Осталось», используется страницей курса и вкладкой «Обзор» карточки ученика.

- [ ] **Шаг 1: создать компонент**

```tsx
// frontend/src/components/students/CourseBalanceStat.tsx
import { CourseBalance } from '@/types/api'

/** Баланс курса — три числа (спека 2026-09-06, п. 7.2): общий и для страницы
 *  курса, и для вкладки «Обзор» карточки ученика — расхождения тут недопустимы,
 *  бейдж цикла и эта строка обязаны читать одно и то же. */
export function CourseBalanceStat({ balance }: { balance: CourseBalance }) {
  return (
    <div className="grid grid-cols-3 gap-3 text-center">
      <div>
        <p className="text-2xl font-bold">{balance.lessons_paid}</p>
        <p className="text-xs text-muted-foreground mt-1">Оплачено</p>
      </div>
      <div>
        <p className="text-2xl font-bold">{balance.lessons_completed}</p>
        <p className="text-xs text-muted-foreground mt-1">Проведено</p>
      </div>
      <div>
        <p className="text-2xl font-bold text-primary">{balance.lessons_remaining}</p>
        <p className="text-xs text-muted-foreground mt-1">Осталось</p>
      </div>
    </div>
  )
}
```

- [ ] **Шаг 2: подключить на странице курса**

`frontend/src/app/(dashboard)/courses/[id]/page.tsx` — добавить импорт `import { CourseBalanceStat } from '@/components/students/CourseBalanceStat'`, заменить строки 335–352:

```tsx
// Было: <div className="grid grid-cols-3 gap-3 text-center">...</div> (336-349) внутри if(balance)/else «Загрузка...»
{balance ? <CourseBalanceStat balance={balance} /> : <p className="text-sm text-muted-foreground">Загрузка...</p>}
```

- [ ] **Шаг 3: проверить**

`cd frontend && npm run lint` — чисто. Визуально страница курса не меняется (тот же JSX, вынесенный в компонент).

- [ ] **Шаг 4: коммит**

```bash
git add frontend/src/components/students/CourseBalanceStat.tsx "frontend/src/app/(dashboard)/courses/[id]/page.tsx"
git commit -m "$(cat <<'EOF'
refactor(students): вынести CourseBalanceStat из страницы курса

Общий компонент для страницы курса и вкладки «Обзор» карточки ученика — без
него бейдж цикла и цифры баланса могли бы разъехаться (спека, п. 7.2).

Co-Authored-By: Claude Sonnet 5 <noreply@anthropic.com>
Claude-Session: https://claude.ai/code/session_01P2LhhXJjYViPqRtttuo9cB
EOF
)"
```

---

## Task 7: Карточка ученика — вкладки, вкладка «Обзор»

**Files:**
- Modify: `frontend/src/app/(dashboard)/students/[id]/page.tsx` (переписывается)

**Interfaces:**
- Consumes: `useStudentOverview(id)`, `CourseBalanceStat`, существующие `PaymentForm`, `BulkPaymentDialog`, `PauseDialog`, `LessonForm`, `useCreateLesson`.
- Produces: экран с вкладками `Обзор | Уроки | Оплаты | ДЗ и материалы` (`Tabs` из `@/components/ui/tabs`).

### Что меняется относительно нынешней страницы

- `useStudent` + `useStudentCourses` + `usePauses` (три запроса) → один `useStudentOverview`. `usePauses`/`useDeletePause` остаются — список заморозок и их снятие не входит в `StudentOverview` (компактный список, отдельная ненагруженная секция, п. 7.0 уже её сделал; трогать незачем).
- Кнопка «Оплата» теперь смотрит на `overview.payable_courses`, а не на `overview.courses` — закрывает пробел п. 7.0: single/bulk выбирается по числу *оплачиваемых* курсов, а не только активных.
- Добавляются: баланс и «оплачено N из M» на строке предмета, банер долга, «Ближайший урок» с кнопками «Войти в комнату»/«Поставить урок», «Последние уроки» (5, только чтение), «Последние оплаты» (5, с правкой/удалением).
- Инлайн-правка цены (`CoursePrice`) теперь работает с `StudentCourseSummary`, а не с `Course` — `useUpdateCourse` ожидает `subject`/`price_per_cycle`/`lessons_per_cycle`/`started_at`/`ended_at` (`UpdateCourseRequest.StartedAt` обязателен, `validate:"required"`, `models/course.go:62`). `StudentCourseSummary` уже несёт все пять полей (Task 2/3) — `CoursePrice` берёт их из сводки без похода за курсом отдельно.

- [ ] **Шаг 1: переписать `students/[id]/page.tsx`**

```tsx
'use client'

import { useState } from 'react'
import { useParams, useRouter } from 'next/navigation'
import { toast } from 'sonner'
import { ArrowLeft, Pencil, RotateCcw, Snowflake, Trash2, Wallet, Video, Plus } from 'lucide-react'

import { useStudentOverview, useUpdateStudent, useRemoveStudent, useRestoreStudent, usePauses, useDeletePause } from '@/lib/hooks/useStudents'
import { useUpdateCourse } from '@/lib/hooks/useCourses'
import { useCreatePayment, useUpdatePayment, useDeletePayment } from '@/lib/hooks/usePayments'
import { useCreateLesson } from '@/lib/hooks/useLessons'
import { StudentForm } from '@/components/students/StudentForm'
import { PauseDialog } from '@/components/students/PauseDialog'
import { PaymentForm } from '@/components/payments/PaymentForm'
import { BulkPaymentDialog } from '@/components/payments/BulkPaymentDialog'
import { LessonForm } from '@/components/lessons/LessonForm'
import { toRecurrenceInput, RecurrenceOptions } from '@/lib/recurrence'
import { CourseBalanceStat } from '@/components/students/CourseBalanceStat'
import { StudentLessonsTab } from '@/components/students/StudentLessonsTab'
import { StudentPaymentsTab } from '@/components/students/StudentPaymentsTab'
import { StudentHomeworkTab } from '@/components/students/StudentHomeworkTab'
import { PageHeader, HeaderMetric } from '@/components/common/PageHeader'
import { CourseTypeBadge } from '@/components/common/CourseTypeBadge'
import { STATUS_LABELS, STATUS_COLORS } from '@/lib/lessonStatus'
import { StudentFormValues } from '@/schemas/student'
import { PaymentFormValues } from '@/schemas/payment'
import { LessonFormValues } from '@/schemas/lesson'
import { StudentCourseSummary, StudentPause, Payment } from '@/types/api'

import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Tabs, TabsList, TabsTrigger, TabsContent } from '@/components/ui/tabs'

export default function StudentDetailPage() {
  const { id } = useParams<{ id: string }>()
  const router  = useRouter()

  const { data: overview, isLoading } = useStudentOverview(id)
  const student = overview?.student
  const courses = overview?.courses ?? []
  const payableCourses = overview?.payable_courses ?? []

  const updateStudent = useUpdateStudent(id)
  const removeStudent  = useRemoveStudent()
  const restoreStudent = useRestoreStudent()

  const { data: pauses = [] } = usePauses(id)
  const deletePause = useDeletePause(id)
  const [paymentOpen, setPaymentOpen] = useState(false)
  const [pauseOpen, setPauseOpen]     = useState(false)
  const [lessonOpen, setLessonOpen]   = useState(false)
  const [editingPayment, setEditingPayment] = useState<Payment | null>(null)
  // Один оплачиваемый курс — прежняя простая форма; разбивка — с двух и
  // больше, включая архивные/ушедшие с долгом (спека, п. 6.8, 7.0).
  const single        = payableCourses.length === 1 ? payableCourses[0] : undefined
  const createPayment = useCreatePayment(single?.id ?? '')
  const updatePayment  = useUpdatePayment(editingPayment?.course_id, id)
  const deletePayment  = useDeletePayment(editingPayment?.course_id, id)
  // Урок ставится на первый активный курс по умолчанию — на карточке ученика
  // курсов обычно 1-2, выбор предмета в форме не нужен для частого случая.
  const createLesson = useCreateLesson(courses[0]?.course_id ?? '')

  const [formOpen, setFormOpen] = useState(false)

  async function handleUpdate(values: StudentFormValues) {
    await updateStudent.mutateAsync(values)
    toast.success('Ученик обновлён')
  }

  async function handleDelete() {
    if (!student) return
    const result = await removeStudent(student)
    if (result === 'deleted') {
      toast.success('Ученик удалён')
      router.push('/students')
    }
    if (result === 'archived') toast.success('Ученик перенесён в архив')
  }

  async function handleRestore() {
    try {
      await restoreStudent.mutateAsync(id)
      toast.success('Ученик восстановлен')
    } catch {
      toast.error('Не удалось восстановить ученика')
    }
  }

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

  async function handlePaymentEdit(values: PaymentFormValues) {
    if (!editingPayment) return
    await updatePayment.mutateAsync({
      id:   editingPayment.id,
      data: { student_id: id, amount: values.amount, lessons_count: values.lessons_count, paid_at: values.paid_at },
    })
    toast.success('Оплата обновлена')
  }

  async function handlePaymentDelete(p: Payment) {
    if (!confirm('Удалить оплату?')) return
    await deletePayment.mutateAsync(p.id)
    toast.success('Оплата удалена')
  }

  async function handleLessonSubmit(values: LessonFormValues, recurrence?: RecurrenceOptions) {
    const baseISO  = new Date(values.scheduled_at).toISOString()
    const courseId = courses[0].course_id
    if (recurrence) {
      // Даты раскатывает сервер — тот же приём, что на странице курса
      // (courses/[id]/page.tsx:161-174).
      await createLesson.mutateAsync({
        course_id:        courseId,
        scheduled_at:     baseISO,
        duration_minutes: values.duration_minutes,
        notes:            values.notes,
        recurrence:       { ...toRecurrenceInput(recurrence), ends_on: courses[0].ended_at ?? undefined },
      })
      toast.success('Серия создана')
    } else {
      await createLesson.mutateAsync({ ...values, scheduled_at: baseISO, course_id: courseId })
      toast.success('Урок добавлен')
    }
    setLessonOpen(false)
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

  if (isLoading) {
    return <div className="h-32 rounded-lg bg-muted animate-pulse" />
  }

  if (!student) {
    return <p className="text-sm text-muted-foreground">Ученик не найден</p>
  }

  const name = student.last_name ? `${student.first_name} ${student.last_name}` : student.first_name

  return (
    <>
      <button
        onClick={() => router.push('/students')}
        className="flex items-center gap-1.5 text-sm text-muted-foreground hover:text-foreground mb-4"
      >
        <ArrowLeft className="h-4 w-4" /> Все ученики
      </button>

      <PageHeader
        title={name}
        meta={!student.active ? <HeaderMetric color="var(--muted-foreground)">В архиве</HeaderMetric> : undefined}
        actions={
          <div className="flex gap-2">
            {student.active && payableCourses.length > 0 && (
              <Button size="sm" onClick={() => setPaymentOpen(true)}>
                <Wallet className="h-4 w-4 mr-1.5" /> Оплата
              </Button>
            )}
            {student.active && (
              <Button size="sm" variant="outline" onClick={() => setPauseOpen(true)}>
                <Snowflake className="h-4 w-4 mr-1.5" /> Заморозить
              </Button>
            )}
            <Button size="sm" variant="outline" onClick={() => setFormOpen(true)}>
              <Pencil className="h-4 w-4 mr-1.5" /> Редактировать
            </Button>
            {student.active ? (
              <Button size="sm" variant="destructive" onClick={handleDelete}>
                <Trash2 className="h-4 w-4 mr-1.5" /> Удалить
              </Button>
            ) : (
              <Button size="sm" variant="outline" onClick={handleRestore}>
                <RotateCcw className="h-4 w-4 mr-1.5" /> Восстановить
              </Button>
            )}
          </div>
        }
      />

      <div className="border rounded-xl bg-card p-5 max-w-md space-y-3 mt-4">
        <Row label="Email"   value={student.email} />
        <Row label="Телефон" value={student.phone || '—'} />
      </div>

      <Tabs defaultValue="overview" className="mt-4">
        <TabsList>
          <TabsTrigger value="overview">Обзор</TabsTrigger>
          <TabsTrigger value="lessons">Уроки</TabsTrigger>
          <TabsTrigger value="payments">Оплаты</TabsTrigger>
          <TabsTrigger value="homework">ДЗ и материалы</TabsTrigger>
        </TabsList>

        <TabsContent value="overview" className="space-y-4 mt-4">
          {overview.total_owed > 0 && (
            <div className="border border-destructive/30 bg-destructive/5 rounded-xl p-4 text-sm">
              Долг: <span className="font-semibold">{Math.round(overview.total_owed).toLocaleString()} ₸</span>
            </div>
          )}

          <div className="border rounded-xl bg-card p-4">
            <h2 className="text-sm font-semibold mb-3">Предметы ({courses.length})</h2>
            {courses.length === 0 ? (
              <p className="text-sm text-muted-foreground">Нет курсов</p>
            ) : (
              <div className="space-y-3">
                {courses.map((c) => (
                  <div key={c.course_id} className="border-b last:border-0 pb-3 last:pb-0">
                    <div className="flex items-center justify-between text-sm mb-2">
                      <span className="font-medium flex items-center gap-2">
                        {c.subject}
                        <CourseTypeBadge isGroup={c.is_group} />
                      </span>
                      <CoursePrice course={c} />
                    </div>
                    <CourseBalanceStat balance={c.balance} />
                  </div>
                ))}
              </div>
            )}
          </div>

          <div className="border rounded-xl bg-card p-4">
            <div className="flex items-center justify-between mb-3">
              <h2 className="text-sm font-semibold">Ближайший урок</h2>
              <Button size="sm" variant="outline" onClick={() => setLessonOpen(true)}>
                <Plus className="h-4 w-4 mr-1.5" /> Поставить урок
              </Button>
            </div>
            {overview.next_lesson ? (
              <div className="flex items-center justify-between text-sm">
                <span>
                  {new Date(overview.next_lesson.scheduled_at).toLocaleString('ru-RU', {
                    day: '2-digit', month: 'short', hour: '2-digit', minute: '2-digit',
                  })} · {overview.next_lesson.subject}
                </span>
                <Button size="sm" onClick={() => router.push(`/lessons/${overview.next_lesson!.id}/call`)}>
                  <Video className="h-4 w-4 mr-1.5" /> Войти в комнату
                </Button>
              </div>
            ) : (
              <p className="text-sm text-muted-foreground">Нет запланированных уроков</p>
            )}
          </div>

          <div className="border rounded-xl bg-card p-4">
            <h2 className="text-sm font-semibold mb-3">Последние уроки</h2>
            {overview.recent_lessons.length === 0 ? (
              <p className="text-sm text-muted-foreground">Уроков ещё не было</p>
            ) : (
              <div className="space-y-1">
                {overview.recent_lessons.map((l) => (
                  <div key={l.id} className="flex items-center justify-between py-1.5 border-b last:border-0 text-sm">
                    <span className="text-muted-foreground">
                      {new Date(l.scheduled_at).toLocaleDateString('ru-RU')} · {l.subject}
                    </span>
                    <span className={`text-xs px-2 py-0.5 rounded-full ${STATUS_COLORS[l.status] ?? ''}`}>
                      {STATUS_LABELS[l.status] ?? l.status}
                    </span>
                  </div>
                ))}
              </div>
            )}
          </div>

          <div className="border rounded-xl bg-card p-4">
            <h2 className="text-sm font-semibold mb-3">Последние оплаты</h2>
            {overview.payments.length === 0 ? (
              <p className="text-sm text-muted-foreground">Оплат ещё не было</p>
            ) : (
              <div className="space-y-1">
                {overview.payments.map((p) => (
                  <div key={p.id} className="flex items-center justify-between py-1.5 border-b last:border-0 text-sm group">
                    <span className="text-muted-foreground">{new Date(p.paid_at).toLocaleDateString('ru-RU')} · {p.subject}</span>
                    <span className="font-medium">{p.amount.toLocaleString()} ₸ · {p.lessons_count} ур.</span>
                    <div className="flex items-center gap-1 opacity-0 group-hover:opacity-100 transition-opacity">
                      <Button size="icon" variant="ghost" className="h-7 w-7"
                        onClick={() => { setEditingPayment(p); setPaymentOpen(true) }}>
                        <Pencil className="h-3.5 w-3.5" />
                      </Button>
                      <Button size="icon" variant="ghost" className="h-7 w-7 text-destructive hover:text-destructive"
                        onClick={() => handlePaymentDelete(p)}>
                        <Trash2 className="h-3.5 w-3.5" />
                      </Button>
                    </div>
                  </div>
                ))}
              </div>
            )}
          </div>

          <div className="border rounded-xl bg-card p-4">
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
                      size="icon" variant="ghost" aria-label="Разморозить"
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
        </TabsContent>

        <TabsContent value="lessons" className="mt-4">
          <StudentLessonsTab courses={courses} />
        </TabsContent>
        <TabsContent value="payments" className="mt-4">
          <StudentPaymentsTab courses={courses} />
        </TabsContent>
        <TabsContent value="homework" className="mt-4">
          <StudentHomeworkTab courses={courses} />
        </TabsContent>
      </Tabs>

      <StudentForm open={formOpen} onClose={() => setFormOpen(false)} onSubmit={handleUpdate} initial={student} />
      {single && (
        <PaymentForm
          open={paymentOpen}
          onClose={() => { setPaymentOpen(false); setEditingPayment(null) }}
          onSubmit={editingPayment ? handlePaymentEdit : handleSinglePayment}
          pricePerLesson={single.price_per_cycle / single.lessons_per_cycle}
          lessonsPerCycle={single.lessons_per_cycle}
          initialValues={editingPayment ? {
            amount: editingPayment.amount, lessons_count: editingPayment.lessons_count,
            paid_at: new Date(editingPayment.paid_at).toISOString().slice(0, 10),
          } : undefined}
          paymentId={editingPayment?.id}
        />
      )}
      {payableCourses.length >= 2 && !editingPayment && (
        <BulkPaymentDialog open={paymentOpen} onClose={() => setPaymentOpen(false)} student={student} courses={payableCourses} />
      )}
      <PauseDialog open={pauseOpen} onClose={() => setPauseOpen(false)} studentId={id} studentName={name} />
      {courses[0] && (
        <LessonForm
          open={lessonOpen}
          onClose={() => setLessonOpen(false)}
          onSubmit={handleLessonSubmit}
          courseEndAt={courses[0].ended_at ?? undefined}
        />
      )}
    </>
  )
}

const fmtDay = (iso: string) => new Date(iso).toLocaleDateString('ru-RU', { timeZone: 'UTC' })

function CoursePrice({ course }: { course: StudentCourseSummary }) {
  const update = useUpdateCourse(course.course_id)
  const [editing, setEditing] = useState(false)
  const [draft, setDraft]     = useState('')

  async function save() {
    setEditing(false)
    if (draft.trim() === '') return
    const next = Number(draft)
    if (!Number.isFinite(next) || next < 0 || next === course.price_per_cycle) return
    try {
      await update.mutateAsync({
        subject:           course.subject,
        price_per_cycle:   next,
        lessons_per_cycle: course.lessons_per_cycle,
        started_at:        course.started_at,
        ended_at:          course.ended_at ?? undefined,
      })
      toast.success('Цена обновлена')
    } catch {
      toast.error('Не удалось обновить цену')
    }
  }

  if (editing) {
    return (
      <Input autoFocus type="number" min={0} value={draft}
        onChange={(e) => setDraft(e.target.value)} onBlur={save}
        onKeyDown={(e) => { if (e.key === 'Enter') save(); if (e.key === 'Escape') setEditing(false) }}
        className="h-6 w-24 text-right" />
    )
  }

  return (
    <button type="button"
      onClick={() => { setDraft(String(course.price_per_cycle)); setEditing(true) }}
      className="text-sm hover:text-foreground hover:underline text-right"
    >
      {course.price_per_cycle.toLocaleString()} ₸
      <span className="text-muted-foreground ml-1">
        {course.lessons_per_cycle === 1 ? 'за урок' : `за ${course.lessons_per_cycle} ур.`}
      </span>
    </button>
  )
}

function Row({ label, value }: { label: string; value: string }) {
  return (
    <div className="flex items-center gap-4">
      <span className="text-sm text-muted-foreground w-20 shrink-0">{label}</span>
      <span className="text-sm font-medium">{value}</span>
    </div>
  )
}
```

- [ ] **Шаг 2: `useCreateLesson` — опциональный `studentId` для инвалидации оверью**

`frontend/src/lib/hooks/useLessons.ts`:

```ts
export function useCreateLesson(courseId: string, studentId?: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (data: LessonInput) => lessonsApi.create(data),
    onSuccess:  () => invalidateLessonViews(qc, courseId, studentId),
  })
}
```

Обновить вызов на странице ученика (шаг 1 выше) на `useCreateLesson(courses[0]?.course_id ?? '', id)`.

- [ ] **Шаг 3: прогнать линт**

`cd frontend && npm run lint` — 0 новых ошибок сверх существующих 22 в доске.

- [ ] **Шаг 4: коммит**

```bash
git add "frontend/src/app/(dashboard)/students/[id]/page.tsx" frontend/src/lib/hooks/useLessons.ts
git commit -m "$(cat <<'EOF'
feat(students): карточка ученика — вкладки, баланс, долг, ближайший урок

Обзор собирается одним useStudentOverview: предметы с балансом, банер долга,
ближайший урок с входом в комнату, последние уроки и оплаты. Оплата теперь
предлагает и курсы с долгом, с которых ученик ушёл или которые архивированы
(спека, п. 7.0–7.2).

Co-Authored-By: Claude Sonnet 5 <noreply@anthropic.com>
Claude-Session: https://claude.ai/code/session_01P2LhhXJjYViPqRtttuo9cB
EOF
)"
```

---

## Task 8: Вкладки «Уроки», «Оплаты», «ДЗ и материалы»

**Files:**
- Create: `frontend/src/components/students/StudentLessonsTab.tsx`
- Create: `frontend/src/components/students/StudentPaymentsTab.tsx`
- Create: `frontend/src/components/students/StudentHomeworkTab.tsx`
- Modify: `frontend/src/components/homework/HomeworkEditPopover.tsx:40` (экспортировать `HomeworkForm`)

**Interfaces:**
- Consumes: `StudentCourseSummary[]`, существующие `useLessonsByPeriod`, `usePayments`, `HomeworkForm` (из `HomeworkEditPopover.tsx`).
- Produces: три компонента, каждый — секция по курсу, запросы включаются только когда вкладка реально отрендерена. `TabsContent` в `components/ui/tabs.tsx` оборачивает Base UI `Tabs.Panel` с `keepMounted` по умолчанию `false` (`node_modules/@base-ui/react/tabs/panel/TabsPanel.js`): неактивная панель не рендерится, дочерние хуки этих трёх компонентов не выполняются до первого клика на вкладку — ничего дополнительно оборачивать не нужно.

- [ ] **Шаг 1: `StudentLessonsTab` — уроки по каждому курсу**

```tsx
// frontend/src/components/students/StudentLessonsTab.tsx
'use client'

import { useState } from 'react'
import { useLessonsByPeriod } from '@/lib/hooks/useLessons'
import { PeriodPicker } from '@/components/lessons/PeriodPicker'
import { STATUS_LABELS, STATUS_COLORS } from '@/lib/lessonStatus'
import { CycleBadge } from '@/components/lessons/CycleBadge'
import type { StudentCourseSummary } from '@/types/api'

function currentWeekRange(): { from: Date; to: Date } {
  const now = new Date()
  const day = now.getDay()
  const diff = day === 0 ? -6 : 1 - day
  const from = new Date(now)
  from.setDate(now.getDate() + diff)
  from.setHours(0, 0, 0, 0)
  const to = new Date(from)
  to.setDate(from.getDate() + 7)
  return { from, to }
}

/** Уроки одного курса ученика — период листается независимо для каждого
 *  предмета: у математики и физики разное расписание (спека, п. 7.2). */
function CourseLessons({ course }: { course: StudentCourseSummary }) {
  const [period, setPeriod] = useState(currentWeekRange)
  const { data: lessons = [] } = useLessonsByPeriod(course.course_id, period.from.toISOString(), period.to.toISOString())

  return (
    <div className="border rounded-xl bg-card p-4">
      <div className="flex items-center justify-between mb-3">
        <h3 className="text-sm font-semibold">{course.subject}</h3>
        <PeriodPicker from={period.from} to={period.to} onChange={(from, to) => setPeriod({ from, to })} />
      </div>
      {lessons.length === 0 ? (
        <p className="text-sm text-muted-foreground">Уроков в этом периоде нет</p>
      ) : (
        <div className="space-y-1">
          {lessons.map((l) => (
            <div key={l.id} className="flex items-center justify-between py-1.5 border-b last:border-0 text-sm">
              <span className="text-muted-foreground">
                {new Date(l.scheduled_at).toLocaleString('ru-RU', { day: '2-digit', month: 'short', hour: '2-digit', minute: '2-digit' })}
              </span>
              <div className="flex items-center gap-2">
                {l.cycle_position != null && l.cycle_size != null && <CycleBadge position={l.cycle_position} size={l.cycle_size} />}
                <span className={`text-xs px-2 py-0.5 rounded-full ${STATUS_COLORS[l.status] ?? ''}`}>{STATUS_LABELS[l.status] ?? l.status}</span>
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  )
}

export function StudentLessonsTab({ courses }: { courses: StudentCourseSummary[] }) {
  if (courses.length === 0) return <p className="text-sm text-muted-foreground">Нет курсов</p>
  return <div className="space-y-4">{courses.map((c) => <CourseLessons key={c.course_id} course={c} />)}</div>
}
```

Правка/удаление урока прямо с этой вкладки — вне объёма фазы (критерий приёмки требует «поставить урок», что уже есть на «Обзоре»); список читается, детальные операции — на странице курса по клику (перейти можно, если понадобится, отдельным пунктом, не блокирующим приёмку).

- [ ] **Шаг 2: `StudentPaymentsTab` — вся история оплат по каждому курсу**

```tsx
// frontend/src/components/students/StudentPaymentsTab.tsx
'use client'

import { usePayments } from '@/lib/hooks/usePayments'
import type { StudentCourseSummary } from '@/types/api'

function CoursePayments({ course }: { course: StudentCourseSummary }) {
  const { data: payments = [] } = usePayments(course.course_id)
  return (
    <div className="border rounded-xl bg-card p-4">
      <h3 className="text-sm font-semibold mb-3">{course.subject} ({payments.length})</h3>
      {payments.length === 0 ? (
        <p className="text-sm text-muted-foreground">Оплат нет</p>
      ) : (
        <div className="space-y-1">
          {payments.map((p) => (
            <div key={p.id} className="flex items-center justify-between py-1.5 border-b last:border-0 text-sm">
              <span className="text-muted-foreground">{new Date(p.paid_at).toLocaleDateString('ru-RU')}</span>
              <span className="font-medium">{p.amount.toLocaleString()} ₸ · {p.lessons_count} ур.</span>
            </div>
          ))}
        </div>
      )}
    </div>
  )
}

export function StudentPaymentsTab({ courses }: { courses: StudentCourseSummary[] }) {
  if (courses.length === 0) return <p className="text-sm text-muted-foreground">Нет курсов</p>
  return <div className="space-y-4">{courses.map((c) => <CoursePayments key={c.course_id} course={c} />)}</div>
}
```

Правка/удаление — уже на «Обзоре» для последних пяти; полная история здесь — только чтение, редактирование старой оплаты (не из последних пяти) остаётся через страницу курса до отдельного запроса пользователей.

- [ ] **Шаг 3: `StudentHomeworkTab` — ДЗ каждого курса**

`HomeworkEditPopover.tsx` уже содержит весь нужный код в приватной `HomeworkForm` (строка 40: запрос `coursesApi.getHomework`, черновик, мутация `updateHomework`, textarea, кнопка «Сохранить») — она не экспортирована только потому, что раньше её вызывал исключительно поповер. Не дублировать эту логику: экспортировать `HomeworkForm` и переиспользовать без изменений.

`frontend/src/components/homework/HomeworkEditPopover.tsx:40` — заменить:

```ts
// Было:
function HomeworkForm({ courseId, onClose }: { courseId: string; onClose: () => void }) {
// Стало:
export function HomeworkForm({ courseId, onClose }: { courseId: string; onClose: () => void }) {
```

`onClose` внутри `HomeworkForm` вызывается только из `save.onSuccess` (закрыть поповер после сохранения) и из кнопки «Отмена» — на вкладке без поповера это безопасный no-op.

```tsx
// frontend/src/components/students/StudentHomeworkTab.tsx
'use client'

import { HomeworkForm } from '@/components/homework/HomeworkEditPopover'
import type { StudentCourseSummary } from '@/types/api'

export function StudentHomeworkTab({ courses }: { courses: StudentCourseSummary[] }) {
  if (courses.length === 0) return <p className="text-sm text-muted-foreground">Нет курсов</p>
  return (
    <div className="space-y-4">
      {courses.map((c) => (
        <div key={c.course_id} className="border rounded-xl bg-card p-4">
          <h3 className="text-sm font-semibold mb-3">{c.subject}</h3>
          <HomeworkForm courseId={c.course_id} onClose={() => {}} />
        </div>
      ))}
    </div>
  )
}
```

- [ ] **Шаг 4: прогнать линт**

`cd frontend && npm run lint`.

- [ ] **Шаг 5: коммит**

```bash
git add frontend/src/components/students/StudentLessonsTab.tsx frontend/src/components/students/StudentPaymentsTab.tsx frontend/src/components/students/StudentHomeworkTab.tsx frontend/src/components/homework/HomeworkEditPopover.tsx
git commit -m "$(cat <<'EOF'
feat(students): вкладки «Уроки», «Оплаты», «ДЗ и материалы»

Каждая — секция по курсу поверх уже существующих ручек, запрашивается только
при открытии вкладки: агрегат /overview остаётся маленьким и быстрым для
самого частого случая — посмотреть баланс перед уроком (спека, п. 7.2).

Co-Authored-By: Claude Sonnet 5 <noreply@anthropic.com>
Claude-Session: https://claude.ai/code/session_01P2LhhXJjYViPqRtttuo9cB
EOF
)"
```

---

## Task 9: Ручной smoke по критериям приёмки (п. 7.3 спеки)

- [ ] Локальный API на тестовой БД: `env DB_URL=<local> SERVER_PORT=:8099 JWT_SECRET=x ROLE=api REDIS_URL= S3_ENDPOINT= RESEND_API_KEY= LIVEKIT_URL= go run .`
- [ ] `curl -s localhost:8099/students/<id>/overview -H "Authorization: Bearer <tutor jwt>" | jq` — один запрос отдаёт `student`, `courses` (с `balance`), `payable_courses`, `next_lesson`, `recent_lessons`, `payments`, `total_owed`.
- [ ] `frontend && npm run dev` — открыть `/students/<id>`: вкладка «Обзор» открывается без видимой последовательной догрузки (Network — один запрос `overview` вместо `student`+`courses`+`balance×N`).
- [ ] Ученик с двумя предметами — оба видны с раздельными балансами (критерий приёмки спеки).
- [ ] Ученик, ушедший из группы с долгом (искусственно создать: `left_at` в прошлом + непогашенный долг) — кнопка «Оплата» открывает `BulkPaymentDialog`/`PaymentForm` с этим курсом в списке, платёж проходит и долг гасится (закрытие пробела п. 7.0).
- [ ] «Поставить урок» с карточки ученика создаёт урок на первом курсе, виден в календаре.
- [ ] «Войти в комнату» на ближайшем уроке ведёт на `/lessons/:id/call`.
- [ ] Инлайн-правка цены на «Обзоре» сохраняет пакет, а не цену урока (регрессия к фазе 0-1, п. 4.2).
- [ ] Вкладки «Уроки»/«Оплаты»/«ДЗ и материалы» — данные по каждому курсу, запросы видны в Network только после клика на вкладку, не при первом рендере.
- [ ] `go build ./...`, `go test ./...` зелёные (кроме известного `TestGetAllByTutor_CarriesSubjectAndStudentName`).
- [ ] `TEST_DB_URL=... go test -tags=integration -count=1 ./repository/...` зелёный.
- [ ] `cd frontend && npm run lint` — без новых ошибок.
- [ ] `go vet ./...` и `go vet -tags=integration ./repository/` чисты.
