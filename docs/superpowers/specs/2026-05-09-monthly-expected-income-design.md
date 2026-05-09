# Дизайн: Ожидаемый доход за месяц

**Дата:** 2026-05-09  
**Статус:** Утверждён

---

## Постановка задачи

Добавить метрику «Ожидаемый доход за текущий месяц» на страницы Dashboard и Payments.

**Формула:**
```
expected = SUM over all tutor's active courses of
           (course.price_per_lesson × COUNT(lessons WHERE status IN ('scheduled','completed','missed')
                                            AND scheduled_at IN current month))
```

Отменённые уроки (`cancelled`) не учитываются. Пропущенные (`missed`) учитываются — урок со стороны тьютора состоялся.

---

## Backend

### 1. `repository/payment.go`

Добавить метод в интерфейс `PaymentRepository` и реализацию:

```go
GetMonthlyExpected(ctx context.Context, tutorID string) (float64, error)
```

SQL-запрос:

```sql
SELECT COALESCE(SUM(c.price_per_lesson * lc.cnt), 0)
FROM courses c
JOIN (
    SELECT course_id, COUNT(*) AS cnt
    FROM lessons
    WHERE status IN ('scheduled', 'completed', 'missed')
      AND scheduled_at >= date_trunc('month', NOW())
      AND scheduled_at <  date_trunc('month', NOW()) + interval '1 month'
    GROUP BY course_id
) lc ON lc.course_id = c.id
WHERE c.tutor_id = $1
```

### 2. `service/payment.go`

Добавить в интерфейс `PaymentService` и реализацию:

```go
GetMonthlyExpected(ctx context.Context, tutorID string) (float64, error)
```

Реализация делегирует вызов в репозиторий без дополнительной логики.

### 3. `handlers/payment.go`

Новый метод `GetMonthlyExpected` — аналогично существующему `GetMonthlyIncome`:

```go
func (h *PaymentHandler) GetMonthlyExpected(c *gin.Context) {
    // читает tutorID, вызывает service.GetMonthlyExpected, возвращает { "total": float64 }
}
```

### 4. `router/router.go`

Новый маршрут в защищённой группе:

```
GET /payments/monthly-expected
```

---

## Frontend

### 5. `lib/api/payments.ts`

Добавить функцию:

```ts
monthlyExpected: () =>
  api.get<{ total: number }>('/payments/monthly-expected').then((r) => r.data.total),
```

### 6. `lib/hooks/usePayments.ts`

Добавить ключ и хук:

```ts
export const paymentKeys = {
  ...
  monthlyExpected: ['payments', 'monthly-expected'] as const,
}

export function useMonthlyExpected() {
  return useQuery({
    queryKey: paymentKeys.monthlyExpected,
    queryFn:  paymentsApi.monthlyExpected,
  })
}
```

### 7. Dashboard (`app/(dashboard)/dashboard/page.tsx`)

Сегмент «Доход» обновить:
- **value** = ожидаемый доход (`formatAmount(monthlyExpected)`)
- **meta** = `"получено ₸ " + monthlyIncome.toLocaleString('ru-RU')`
- **loading** = `expectedLoading || incomeLoading`

Подключить `useMonthlyExpected`.

### 8. Payments (`app/(dashboard)/payments/page.tsx`)

Сегмент «Ожидается» обновить:
- **value** = `"₸ " + monthlyExpected.toLocaleString('ru-RU')`
- **meta** = `"этот месяц"`
- **loading** = `expectedLoading`

Подключить `useMonthlyExpected`.

---

## Граничные случаи

| Ситуация | Поведение |
|---|---|
| У тьютора нет уроков в этом месяце | Возвращает `0` |
| Курс без уроков в этом месяце | Не входит в SUM (COALESCE → 0) |
| Все уроки отменены | Возвращает `0` |

---

## Изменения БД

Миграций не требуется — запрос использует существующие таблицы `courses` и `lessons`.
