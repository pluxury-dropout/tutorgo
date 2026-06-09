# Дизайн: виджет «Текущие циклы» на дашборде

**Дата:** 2026-06-08  
**Статус:** утверждён

---

## Суть задачи

Добавить виджет на главную страницу (`/dashboard`), который показывает текущий платёжный цикл для каждого курса тьютора. Одна строка = один курс. Строка содержит: предмет, имя ученика (или «Группа»), прогресс цикла (`X / N`), дата последнего урока цикла.

**Определение «текущего цикла»:** самый последний цикл курса, в котором есть хотя бы один урок со статусом `scheduled`.

---

## Визуальный дизайн

Виджет соответствует стилю существующих виджетов дашборда (`WIDGET_HEAD`, `WIDGET_TITLE`, `WIDGET_LINK`). Три колонки на строку:

```
Предмет · Ученик       [прогресс]   дата
Математика             [2 / 4]      15 июн
  Азиз

Английский             [4 / 4]      18 июн    ← зелёный badge
  Группа

Физика                 [1 / 4]      20 июн
  Айгерим
```

- Badge `X / N` — серый для промежуточных значений
- Badge `N / N` — зелёный (`var(--success)`), когда цикл полностью завершён
- Групповые курсы: вместо имени ученика — нет подписи (или пусто)
- Пустое состояние: «Нет активных циклов»

---

## Архитектура

### 1. Новая модель — `models/lesson.go`

```go
type CurrentCycleInfo struct {
    CourseID    string
    Subject     string
    StudentName *string // nil для групповых курсов
    Progress    int     // кол-во completed+missed уроков в цикле
    CycleSize   int     // lessons_per_cycle
    LastAt      string  // scheduled_at последнего урока цикла
}
```

### 2. Новый repo метод — `repository/lesson.go`

`GetAllLessonsForCycles(ctx context.Context, tutorID string) ([]CalendarLesson, error)`

SQL аналогичен `GetCalendar`, но без фильтра по дате. Возвращает все не-отменённые (`status != 'cancelled'`) уроки тьютора с рангом (`ROW_NUMBER() OVER (PARTITION BY course_id ORDER BY scheduled_at)`), subject, student_name, is_group.

Интерфейс `LessonRepository` расширяется этим методом.

### 3. Новый service метод — `service/lesson.go`

`GetCurrentCycles(ctx context.Context, tutorID string) ([]models.CurrentCycleInfo, error)`

Логика:
1. Вызывает `repo.GetAllLessonsForCycles(ctx, tutorID)` → все уроки с рангами
2. Собирает список course IDs из результата
3. Вызывает `paymentRepo.GetByCoursesBatch(ctx, courseIDs)` → платежи по курсам
4. Для каждого курса вызывает существующий `computeCyclePositions()` — логика не дублируется
5. Группирует уроки по курсу; для каждого курса находит цикл с хотя бы одним `scheduled` уроком (если таких циклов несколько — берёт последний по `scheduled_at`)
6. Для найденного цикла: считает `Progress` (completed+missed), берёт `CycleSize`, `LastAt` (max `scheduled_at` в цикле)
7. Возвращает `[]CurrentCycleInfo`, отсортированный по `LastAt` убыванию

Интерфейс `LessonService` расширяется этим методом.

### 4. Новый endpoint — `handlers/lesson.go`

`GET /dashboard/cycles` → `LessonHandler.GetCurrentCycles`

```go
func (h *LessonHandler) GetCurrentCycles(c *gin.Context) {
    tutorID := c.GetString("tutorID")
    if tutorID == "" { /* 401 */ }
    cycles, err := h.service.GetCurrentCycles(c.Request.Context(), tutorID)
    // ...
    c.JSON(200, cycles)
}
```

Регистрируется в `router/router.go` как `GET /dashboard/cycles` под Auth middleware.

JSON ответ (массив):
```json
[
  { "course_id": "...", "subject": "Математика", "student_name": "Азиз", "progress": 2, "cycle_size": 4, "last_at": "2026-06-15T10:00:00Z" },
  { "course_id": "...", "subject": "Английский", "student_name": null, "progress": 4, "cycle_size": 4, "last_at": "2026-06-18T10:00:00Z" }
]
```

---

## Frontend

### Новый хук — `frontend/src/lib/hooks/useCalendar.ts`

```ts
export function useCurrentCycles() {
  return useQuery({
    queryKey: ['dashboard', 'cycles'],
    queryFn:  () => calendarApi.getCurrentCycles(),
  })
}
```

Новый метод `getCurrentCycles()` в `frontend/src/lib/api/calendar.ts`.

Новый тип `CurrentCycleInfo` в `frontend/src/types/api.ts`.

### Новый виджет — `frontend/src/app/(dashboard)/dashboard/page.tsx`

Добавляется третьей секцией в grid `grid-cols-1 md:grid-cols-2`. Три колонки:
`gridTemplateColumns: '1fr auto 52px'`

Badge рендерится компонентом прямо в странице (не выносится отдельно — слишком мал).

---

## Что не входит в скоуп

- Навигация по клику на строку курса (можно добавить позже)
- Фильтрация по статусу курса (активный/завершённый)
- Тесты на новый service метод (опционально — добавить при желании)
