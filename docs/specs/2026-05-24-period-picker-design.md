# Period Picker для списка уроков

**Дата:** 2026-05-24  
**Статус:** Утверждён

## Проблема

Текущий список уроков использует числовую пагинацию (`page=1,2,3…`) с сортировкой `ORDER BY scheduled_at DESC`. В результате первая страница показывает самые дальние будущие уроки — бесполезно для ежедневной работы тьютора.

## Решение

Заменить числовую пагинацию на **навигацию по периодам**. По умолчанию — текущая неделя. Переключение через стрелки или выбор произвольного диапазона в dropdown-миникалендаре.

---

## Бэкенд

### Новый метод репозитория

Файл: `repository/lesson.go`

```go
GetByPeriod(ctx context.Context, courseID, tutorID, from, to string) ([]models.Lesson, error)
```

SQL:
```sql
SELECT id, course_id, tutor_id, scheduled_at, duration_minutes, status, notes, series_id, room_started_at, room_ended_at
FROM lessons
WHERE course_id = $1
  AND tutor_id = $2
  AND scheduled_at >= $3
  AND scheduled_at < $4
ORDER BY scheduled_at ASC
```

Параметры `from` и `to` — ISO-строки (`2026-05-19T00:00:00Z`). Конец диапазона не включается (`<`), поэтому для недели `to = следующий_пн`.

### Изменения в сервисе

Файл: `service/lesson.go`

Новый метод:
```go
GetByPeriod(ctx context.Context, courseID, tutorID, from, to string) ([]models.Lesson, error)
```
Проксирует вызов в репозиторий. Валидация дат не нужна — фронтенд всегда передаёт корректный ISO-формат.

### Изменения в хэндлере

Файл: `handlers/lesson.go` → `GetByCourse`

Условие (до существующей ветки `?page=`):
```go
if from := c.Query("from"); from != "" {
    to := c.Query("to")
    if to == "" {
        c.JSON(http.StatusBadRequest, gin.H{"error": "to is required when from is set"})
        return
    }
    lessons, err := h.service.GetByPeriod(ctx, courseID, tutorID, from, to)
    ...
    c.JSON(http.StatusOK, lessons)
    return
}
```

**Маршрутизация не меняется** — тот же `GET /lessons?course_id=X&from=Y&to=Z`.

**Миграции не нужны.**

---

## Фронтенд

### Новые компоненты

#### `components/lessons/PeriodPicker.tsx`

Props:
```ts
interface PeriodPickerProps {
  from: Date
  to: Date          // эксклюзивный конец (первый день следующего периода)
  onChange: (from: Date, to: Date) => void
}
```

Рендер:
```
[‹]  [📅  19–25 мая]  [›]
```

- Стрелка `‹` — сдвигает `from` и `to` на `-7` дней
- Стрелка `›` — сдвигает на `+7` дней
- Клик на центральную зону — открывает `MiniCalendar` как dropdown под кнопкой

Метка периода:
- Если диапазон равен ровно одной неделе: `«19–25 мая»`
- Если в одном месяце: `«1–15 мая»`
- Если переходит месяц: `«28 апр – 4 мая»`

#### `components/lessons/MiniCalendar.tsx`

Props:
```ts
interface MiniCalendarProps {
  from: Date
  to: Date          // эксклюзивный конец
  onSelect: (from: Date, to: Date) => void
  onClose: () => void
}
```

**Дизайн:** Shadcn/Modern (вариант B)
- Начало и конец диапазона — тёмные круги (`bg-foreground text-background`)
- Середина диапазона — серая заливка (`bg-muted`)
- Суббота/воскресенье — приглушённый цвет текста

**Выбор диапазона (два клика):**
1. Первый клик → устанавливает `startDate`, сбрасывает `endDate`
2. Hover после первого клика → предварительный показ диапазона
3. Второй клик → если дата ≥ `startDate`, вызывает `onSelect(start, dayAfterEnd)`; если < — сброс и новый первый клик

**Шорткаты (нижняя полоска):**
- «Эта неделя» — устанавливает Пн–Вс текущей недели, сразу вызывает `onSelect`
- «Этот месяц» — первый и последний день текущего месяца, сразу вызывает `onSelect`

**Навигация по месяцам:** стрелки `‹` / `›`, внутреннее состояние компонента.

**Закрытие:** по клику вне (через `useEffect` + `mousedown` listener) или после выбора конечной даты.

### Изменения в существующих файлах

#### `lib/api/lessons.ts`

Добавить метод:
```ts
listByPeriod: (courseId: string, from: string, to: string) =>
  api.get<Lesson[]>('/lessons', { params: { course_id: courseId, from, to } })
     .then((r) => r.data ?? [])
```

#### `lib/hooks/useLessons.ts`

Добавить хук:
```ts
export function useLessonsByPeriod(courseId: string, from: string, to: string) {
  return useQuery({
    queryKey: [...lessonKeys.byCourse(courseId), 'period', from, to] as const,
    queryFn:  () => lessonsApi.listByPeriod(courseId, from, to),
    enabled:  !!courseId && !!from && !!to,
  })
}
```

#### `app/(dashboard)/courses/[id]/page.tsx`

- Убрать: `lessonPage`, `lessonsTotalPages`, `useLessonsPaged`, `useEffect` для коррекции страницы
- Заголовок блока «Уроки (N)» → `N` = `lessons.length` (уроки текущего периода, не общее количество)
- Добавить:
  ```ts
  const [period, setPeriod] = useState(() => currentWeekRange()) // { from: Date, to: Date }
  const { data: lessons = [], isLoading: lessonsLoading } =
    useLessonsByPeriod(id, period.from.toISOString(), period.to.toISOString())
  ```
- Заменить числовую пагинацию в JSX на `<PeriodPicker>` в заголовке блока «Уроки»

### Вспомогательная функция

```ts
// Возвращает { from: Date (Пн), to: Date (следующий Пн) } для текущей недели
function currentWeekRange(): { from: Date; to: Date } {
  const now = new Date()
  const day = now.getDay()                        // 0=Вс, 1=Пн...
  const diff = day === 0 ? -6 : 1 - day          // сдвиг до Пн
  const from = new Date(now)
  from.setDate(now.getDate() + diff)
  from.setHours(0, 0, 0, 0)
  const to = new Date(from)
  to.setDate(from.getDate() + 7)
  return { from, to }
}
```

---

## UX-сценарии

| Сценарий | Поведение |
|---|---|
| Первый вход на страницу | Показывается текущая неделя |
| Уроков в периоде нет | «Нет уроков в этом периоде» |
| Нажать `›` | Период сдвигается на 7 дней вперёд, запрос переотправляется |
| Клик на иконку календаря | Открывается MiniCalendar с текущим диапазоном |
| Выбрать «Эта неделя» | Возврат к текущей неделе, dropdown закрывается |
| Создать/удалить урок | `invalidateQueries` по ключу периода, список обновляется |

---

## Что не меняется

- Числовая пагинация `GetByCoursePaged` и хук `useLessonsPaged` — **остаются** (не удаляем, просто не используются на этой странице)
- Маршруты в `router/router.go` — без изменений
- Все остальные части страницы курса — без изменений
