# Payment Delete & Update Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add `PUT /payments/:id` backend endpoint and delete/edit UI actions (Pencil + Trash2 on hover) to both `/payments` and `/courses/[id]` pages.

**Architecture:** Standard layered Go addition (model → repo → service → handler → router) for update, then three frontend layers (API client → hooks → UI). PaymentForm gains optional `initialValues`/`paymentId` props for edit mode. Delete uses `window.confirm()` matching the existing student delete pattern.

**Tech Stack:** Go/Gin, pgx/v5, React/Next.js, TanStack Query, react-hook-form + zod, lucide-react

---

## File Map

| File | Change |
|------|--------|
| `models/payment.go` | Add `UpdatePaymentRequest` |
| `repository/payment.go` | Add `Update` to interface + impl |
| `service/payment.go` | Add `Update` to interface + impl |
| `handlers/payment.go` | Add `Update` handler |
| `handlers/mocks_test.go` | Add `Update` to `mockPaymentService` |
| `service/payment_test.go` | Add `Update` to `mockPaymentRepo` |
| `router/router.go` | Register `PUT /payments/:id` |
| `frontend/src/lib/api/payments.ts` | Add `update`, `delete` functions |
| `frontend/src/lib/hooks/usePayments.ts` | Add `useUpdatePayment`, `useDeletePayment` |
| `frontend/src/components/payments/PaymentForm.tsx` | Support edit mode via `initialValues`/`paymentId` |
| `frontend/src/app/(dashboard)/payments/page.tsx` | Add Pencil + Trash2 actions column |
| `frontend/src/app/(dashboard)/courses/[id]/page.tsx` | Add Pencil + Trash2 to payment rows |

---

## Task 1: Add UpdatePaymentRequest model

**Files:**
- Modify: `models/payment.go`

- [ ] **Step 1: Add the model**

Open `models/payment.go`. After the existing `CreatePaymentRequest` struct, add:

```go
type UpdatePaymentRequest struct {
	Amount       float64   `json:"amount"        validate:"required,gt=0"`
	LessonsCount int       `json:"lessons_count" validate:"required,gt=0"`
	PaidAt       time.Time `json:"paid_at"       validate:"required"`
}
```

- [ ] **Step 2: Verify build**

```bash
go build ./...
```
Expected: no output (success).

- [ ] **Step 3: Commit**

```bash
git add models/payment.go
git commit -m "feat: add UpdatePaymentRequest model"
```

---

## Task 2: Add Update to payment repository

**Files:**
- Modify: `repository/payment.go`

- [ ] **Step 1: Add `Update` to the interface**

In `repository/payment.go`, add `Update` to the `PaymentRepository` interface:

```go
type PaymentRepository interface {
	Create(ctx context.Context, req models.CreatePaymentRequest) (models.Payment, error)
	GetByCourse(ctx context.Context, courseID string, p models.Pagination) ([]models.Payment, int, error)
	GetAllByTutor(ctx context.Context, tutorID string, limit int) ([]models.Payment, error)
	GetAllByTutorPaged(ctx context.Context, tutorID string, p models.Pagination) ([]models.Payment, int, error)
	GetBalance(ctx context.Context, courseID string) (models.CourseBalance, error)
	GetMonthlyIncome(ctx context.Context, tutorID string) (float64, error)
	GetMonthlyExpected(ctx context.Context, tutorID string) (float64, error)
	Delete(ctx context.Context, id string, tutorID string) error
	Update(ctx context.Context, id string, tutorID string, req models.UpdatePaymentRequest) (models.Payment, error)
}
```

- [ ] **Step 2: Add pgx import**

The existing imports in `repository/payment.go` are:
```go
import (
	"context"
	"errors"
	"tutorgo/models"

	"github.com/jackc/pgx/v5/pgxpool"
)
```

Replace with:
```go
import (
	"context"
	"errors"
	"tutorgo/models"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)
```

- [ ] **Step 3: Implement `Update` method**

Add before `GetMonthlyExpected`:

```go
func (r *paymentRepository) Update(ctx context.Context, id string, tutorID string, req models.UpdatePaymentRequest) (models.Payment, error) {
	var payment models.Payment
	err := r.conn.QueryRow(ctx,
		`UPDATE payments SET amount=$1, lessons_count=$2, paid_at=$3
		 WHERE id=$4 AND course_id IN (SELECT id FROM courses WHERE tutor_id=$5)
		 RETURNING id, course_id, amount, lessons_count, paid_at`,
		req.Amount, req.LessonsCount, req.PaidAt, id, tutorID,
	).Scan(&payment.ID, &payment.CourseID, &payment.Amount, &payment.LessonsCount, &payment.PaidAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return models.Payment{}, errors.New("payment not found")
	}
	return payment, err
}
```

- [ ] **Step 4: Verify build**

```bash
go build ./...
```
Expected: no output.

- [ ] **Step 5: Commit**

```bash
git add repository/payment.go
git commit -m "feat: add Update to payment repository"
```

---

## Task 3: Add Update to payment service + mock

**Files:**
- Modify: `service/payment.go`
- Modify: `service/payment_test.go`

- [ ] **Step 1: Add `Update` to PaymentService interface**

In `service/payment.go`, add to the interface:

```go
type PaymentService interface {
	Create(ctx context.Context, req models.CreatePaymentRequest, tutorID string) (models.Payment, error)
	GetByCourse(ctx context.Context, courseID string, tutorID string, p models.Pagination) ([]models.Payment, int, error)
	GetAllByTutor(ctx context.Context, tutorID string, limit int) ([]models.Payment, error)
	GetAllByTutorPaged(ctx context.Context, tutorID string, p models.Pagination) ([]models.Payment, int, error)
	GetBalance(ctx context.Context, courseID string, tutorID string) (models.CourseBalance, error)
	GetMonthlyIncome(ctx context.Context, tutorID string) (float64, error)
	GetMonthlyExpected(ctx context.Context, tutorID string) (float64, error)
	Delete(ctx context.Context, id string, tutorID string) error
	Update(ctx context.Context, id string, tutorID string, req models.UpdatePaymentRequest) (models.Payment, error)
}
```

- [ ] **Step 2: Implement `Update` on paymentService**

Add after the existing `Delete` method in `service/payment.go`:

```go
func (s *paymentService) Update(ctx context.Context, id string, tutorID string, req models.UpdatePaymentRequest) (models.Payment, error) {
	payment, err := s.repo.Update(ctx, id, tutorID, req)
	if err != nil {
		return models.Payment{}, fmt.Errorf("payment: %w", ErrNotFound)
	}
	return payment, nil
}
```

- [ ] **Step 3: Add `Update` to mockPaymentRepo in service test**

In `service/payment_test.go`, after the existing `Delete` mock method, add:

```go
func (m *mockPaymentRepo) Update(ctx context.Context, id string, tutorID string, req models.UpdatePaymentRequest) (models.Payment, error) {
	args := m.Called(ctx, id, tutorID, req)
	return args.Get(0).(models.Payment), args.Error(1)
}
```

- [ ] **Step 4: Verify build and tests**

```bash
go build ./... && go test ./service/...
```
Expected: `ok tutorgo/service`.

- [ ] **Step 5: Commit**

```bash
git add service/payment.go service/payment_test.go
git commit -m "feat: add Update to payment service"
```

---

## Task 4: Add Update handler + mock + route

**Files:**
- Modify: `handlers/payment.go`
- Modify: `handlers/mocks_test.go`
- Modify: `router/router.go`

- [ ] **Step 1: Add `Update` handler**

In `handlers/payment.go`, add before `GetBalance`:

```go
func (h *PaymentHandler) Update(c *gin.Context) {
	tutorID := c.GetString("tutorID")
	if tutorID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	id := c.Param("id")
	var req models.UpdatePaymentRequest
	if !bindAndValidate(c, &req) {
		return
	}
	payment, err := h.service.Update(c.Request.Context(), id, tutorID, req)
	if err != nil {
		h.log.Error("Failed to update payment", slog.String("id", id), slog.String("error", err.Error()))
		handleServiceError(c, err)
		return
	}
	h.log.Info("Payment updated", slog.String("id", id))
	c.JSON(http.StatusOK, payment)
}
```

- [ ] **Step 2: Add `Update` to mockPaymentService in handler test**

In `handlers/mocks_test.go`, after the existing `Delete` mock method, add:

```go
func (m *mockPaymentService) Update(ctx context.Context, id string, tutorID string, req models.UpdatePaymentRequest) (models.Payment, error) {
	args := m.Called(ctx, id, tutorID, req)
	return args.Get(0).(models.Payment), args.Error(1)
}
```

- [ ] **Step 3: Register route**

In `router/router.go`, in the payments block add `PUT`:

```go
auth.GET("/payments", paymentHandler.GetAll)
auth.POST("/payments", paymentHandler.Create)
auth.DELETE("/payments/:id", paymentHandler.Delete)
auth.PUT("/payments/:id", paymentHandler.Update)
auth.GET("/payments/recent", paymentHandler.GetRecent)
auth.GET("/payments/balance", paymentHandler.GetBalance)
auth.GET("/payments/monthly-income", paymentHandler.GetMonthlyIncome)
auth.GET("/payments/monthly-expected", paymentHandler.GetMonthlyExpected)
```

- [ ] **Step 4: Build and test**

```bash
go build ./... && go test ./...
```
Expected: all packages pass.

- [ ] **Step 5: Commit**

```bash
git add handlers/payment.go handlers/mocks_test.go router/router.go
git commit -m "feat: add PUT /payments/:id endpoint"
```

---

## Task 5: Frontend — API client + hooks

**Files:**
- Modify: `frontend/src/lib/api/payments.ts`
- Modify: `frontend/src/lib/hooks/usePayments.ts`

- [ ] **Step 1: Add `update` and `delete` to API client**

Replace the content of `frontend/src/lib/api/payments.ts`:

```ts
import { api } from './client'
import { Payment, PaymentBalance, PagedResponse } from '@/types/api'

export interface PaymentInput {
  course_id: string
  amount: number
  lessons_count: number
  paid_at?: string
}

export type PaymentUpdateInput = Omit<PaymentInput, 'course_id'>

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
  create: (data: PaymentInput) =>
    api.post<Payment>('/payments', data).then((r) => r.data),
  update: (id: string, data: PaymentUpdateInput) =>
    api.put<Payment>(`/payments/${id}`, data).then((r) => r.data),
  delete: (id: string) =>
    api.delete(`/payments/${id}`).then(() => id),
  getBalance: (courseId: string) =>
    api.get<PaymentBalance>('/payments/balance', { params: { course_id: courseId } })
      .then((r) => r.data),
  monthlyIncome: () =>
    api.get<{ total: number }>('/payments/monthly-income').then((r) => r.data.total),
  monthlyExpected: () =>
    api.get<{ total: number }>('/payments/monthly-expected').then((r) => r.data.total),
}
```

- [ ] **Step 2: Add `useUpdatePayment` and `useDeletePayment` hooks**

Replace the content of `frontend/src/lib/hooks/usePayments.ts`:

```ts
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { paymentsApi, PaymentListParams, PaymentUpdateInput } from '@/lib/api/payments'
import { courseKeys } from '@/lib/hooks/useCourses'

export const paymentKeys = {
  byCourse:        (courseId: string) => ['payments', 'course', courseId] as const,
  paged:           (p: PaymentListParams) => ['payments', 'list', p] as const,
  recent:          ['payments', 'recent'] as const,
  monthlyIncome:   ['payments', 'monthly-income'] as const,
  monthlyExpected: ['payments', 'monthly-expected'] as const,
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
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: paymentKeys.byCourse(courseId) })
      qc.invalidateQueries({ queryKey: courseKeys.balance(courseId) })
      qc.invalidateQueries({ queryKey: ['payments', 'list'] })
      qc.invalidateQueries({ queryKey: paymentKeys.recent })
      qc.invalidateQueries({ queryKey: paymentKeys.monthlyIncome })
    },
  })
}

export function useUpdatePayment(courseId?: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ id, data }: { id: string; data: PaymentUpdateInput }) =>
      paymentsApi.update(id, data),
    onSuccess: () => {
      if (courseId) {
        qc.invalidateQueries({ queryKey: paymentKeys.byCourse(courseId) })
        qc.invalidateQueries({ queryKey: courseKeys.balance(courseId) })
      }
      qc.invalidateQueries({ queryKey: ['payments', 'list'] })
      qc.invalidateQueries({ queryKey: paymentKeys.recent })
      qc.invalidateQueries({ queryKey: paymentKeys.monthlyIncome })
    },
  })
}

export function useDeletePayment(courseId?: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: paymentsApi.delete,
    onSuccess: () => {
      if (courseId) {
        qc.invalidateQueries({ queryKey: paymentKeys.byCourse(courseId) })
        qc.invalidateQueries({ queryKey: courseKeys.balance(courseId) })
      }
      qc.invalidateQueries({ queryKey: ['payments', 'list'] })
      qc.invalidateQueries({ queryKey: paymentKeys.recent })
      qc.invalidateQueries({ queryKey: paymentKeys.monthlyIncome })
    },
  })
}

export function usePaymentsPaged(params: PaymentListParams) {
  return useQuery({
    queryKey: paymentKeys.paged(params),
    queryFn:  () => paymentsApi.listPaged(params),
  })
}
```

- [ ] **Step 3: Verify TypeScript compiles**

```bash
cd frontend && npx tsc --noEmit 2>&1 | head -20
```
Expected: no errors for these files.

- [ ] **Step 4: Commit**

```bash
git add frontend/src/lib/api/payments.ts frontend/src/lib/hooks/usePayments.ts
git commit -m "feat: add update/delete payment API and hooks"
```

---

## Task 6: Extend PaymentForm for edit mode

**Files:**
- Modify: `frontend/src/components/payments/PaymentForm.tsx`

- [ ] **Step 1: Replace PaymentForm with edit-mode support**

Replace the entire content of `frontend/src/components/payments/PaymentForm.tsx`:

```tsx
'use client'

import { useEffect } from 'react'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { toast } from 'sonner'

import { paymentSchema, PaymentFormValues } from '@/schemas/payment'
import { ApiError } from '@/types/api'

import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Dialog, DialogContent, DialogHeader, DialogTitle } from '@/components/ui/dialog'

interface PaymentFormProps {
  open:           boolean
  onClose:        () => void
  onSubmit:       (data: PaymentFormValues) => Promise<void>
  pricePerLesson: number
  initialValues?: PaymentFormValues
  paymentId?:     string
}

export function PaymentForm({
  open,
  onClose,
  onSubmit,
  pricePerLesson,
  initialValues,
  paymentId,
}: PaymentFormProps) {
  const isEdit = !!paymentId

  const {
    register,
    handleSubmit,
    reset,
    watch,
    setValue,
    formState: { errors, isSubmitting },
  } = useForm<PaymentFormValues>({
    resolver: zodResolver(paymentSchema),
    defaultValues: { amount: 0, lessons_count: 0, paid_at: '' },
  })

  const amount = watch('amount')

  useEffect(() => {
    if (open) {
      if (initialValues) {
        reset(initialValues)
      } else {
        reset({ amount: 0, lessons_count: 0, paid_at: new Date().toISOString().slice(0, 10) })
      }
    }
  }, [open, reset, initialValues])

  useEffect(() => {
    if (!isEdit && pricePerLesson > 0 && amount > 0) {
      setValue('lessons_count', Math.floor(amount / pricePerLesson))
    }
  }, [amount, pricePerLesson, setValue, isEdit])

  async function submit(values: PaymentFormValues) {
    try {
      await onSubmit(values)
      onClose()
    } catch (err) {
      const e = err as ApiError
      toast.error(e.message ?? 'Ошибка сохранения')
    }
  }

  return (
    <Dialog open={open} onOpenChange={onClose}>
      <DialogContent className="max-w-sm">
        <DialogHeader>
          <DialogTitle>{isEdit ? 'Редактировать оплату' : 'Новая оплата'}</DialogTitle>
        </DialogHeader>

        <form onSubmit={handleSubmit(submit)} className="space-y-4 pt-2">
          <div className="space-y-1.5">
            <Label htmlFor="amount">Сумма (₸)</Label>
            <Input
              id="amount"
              type="number"
              min={1}
              step="any"
              {...register('amount', { valueAsNumber: true })}
            />
            {errors.amount && (
              <p className="text-xs text-destructive">{errors.amount.message}</p>
            )}
          </div>

          <div className="space-y-1.5">
            <Label htmlFor="lessons_count">
              Уроков оплачено{' '}
              <span className="text-muted-foreground font-normal">
                (цена за урок: {pricePerLesson.toLocaleString()} ₸)
              </span>
            </Label>
            <Input
              id="lessons_count"
              type="number"
              min={1}
              {...register('lessons_count', { valueAsNumber: true })}
            />
            {errors.lessons_count && (
              <p className="text-xs text-destructive">{errors.lessons_count.message}</p>
            )}
          </div>

          <div className="space-y-1.5">
            <Label htmlFor="paid_at">Дата оплаты</Label>
            <Input id="paid_at" type="date" {...register('paid_at')} />
            {errors.paid_at && (
              <p className="text-xs text-destructive">{errors.paid_at.message}</p>
            )}
          </div>

          <div className="flex justify-end gap-2 pt-2">
            <Button type="button" variant="outline" onClick={onClose}>Отмена</Button>
            <Button type="submit" disabled={isSubmitting}>
              {isSubmitting ? 'Сохранение...' : 'Сохранить'}
            </Button>
          </div>
        </form>
      </DialogContent>
    </Dialog>
  )
}
```

- [ ] **Step 2: Verify TypeScript**

```bash
cd frontend && npx tsc --noEmit 2>&1 | grep PaymentForm
```
Expected: no errors.

- [ ] **Step 3: Commit**

```bash
git add frontend/src/components/payments/PaymentForm.tsx
git commit -m "feat: extend PaymentForm to support edit mode"
```

---

## Task 7: Global payments page — actions column

**Files:**
- Modify: `frontend/src/app/(dashboard)/payments/page.tsx`

- [ ] **Step 1: Replace PaymentsPageInner with version including edit/delete**

Replace the entire content of `frontend/src/app/(dashboard)/payments/page.tsx`:

```tsx
'use client'

import { useState, Suspense, useEffect } from 'react'
import { useRouter, useSearchParams } from 'next/navigation'
import { Pencil, Trash2 } from 'lucide-react'
import { toast } from 'sonner'

import { useCourses } from '@/lib/hooks/useCourses'
import {
  usePaymentsPaged,
  useMonthlyIncome,
  useMonthlyExpected,
  useUpdatePayment,
  useDeletePayment,
} from '@/lib/hooks/usePayments'
import { HeaderPanel } from '@/components/HeaderPanel'
import type { KpiSegment } from '@/components/HeaderPanel'
import { Pagination } from '@/components/common/Pagination'
import { PaymentForm } from '@/components/payments/PaymentForm'
import { Button } from '@/components/ui/button'
import type { Payment } from '@/types/api'
import type { PaymentFormValues } from '@/schemas/payment'

const LIMIT = 20

function PaymentsPageInner() {
  const router       = useRouter()
  const searchParams = useSearchParams()
  const [activeSegment, setActiveSegment] = useState('received')
  const [editingPayment, setEditingPayment] = useState<Payment | null>(null)
  const [formOpen, setFormOpen] = useState(false)

  const page = Math.max(1, Number(searchParams.get('page') ?? '1'))

  function handlePageChange(newPage: number) {
    const p = new URLSearchParams(searchParams.toString())
    p.set('page', String(newPage))
    router.push(`/payments?${p}`)
  }

  const { data: courses = [] }                                    = useCourses()
  const { data: pagedPayments, isLoading }                        = usePaymentsPaged({ page, limit: LIMIT })
  const { data: monthlyIncome = 0, isLoading: incomeLoading }     = useMonthlyIncome()
  const { data: monthlyExpected = 0, isLoading: expectedLoading } = useMonthlyExpected()

  const updatePayment = useUpdatePayment()
  const deletePayment = useDeletePayment()

  const payments   = pagedPayments?.data ?? []
  const total      = pagedPayments?.total ?? 0
  const totalPages = Math.ceil(total / LIMIT)

  useEffect(() => {
    if (!isLoading && total > 0 && page > totalPages) handlePageChange(totalPages)
  }, [isLoading, total, page, totalPages]) // eslint-disable-line react-hooks/exhaustive-deps

  const courseMap      = Object.fromEntries(courses.map((c) => [c.id, c.subject]))
  const coursePriceMap = Object.fromEntries(courses.map((c) => [c.id, c.price_per_lesson]))

  function openEdit(p: Payment) {
    setEditingPayment(p)
    setFormOpen(true)
  }

  async function handleEdit(values: PaymentFormValues) {
    if (!editingPayment) return
    await updatePayment.mutateAsync({
      id:   editingPayment.id,
      data: {
        amount:        values.amount,
        lessons_count: values.lessons_count,
        paid_at:       values.paid_at,
      },
    })
    toast.success('Платёж обновлён')
  }

  async function handleDelete(p: Payment) {
    if (!confirm(`Удалить платёж на ${p.amount.toLocaleString()} ₸?`)) return
    await deletePayment.mutateAsync(p.id)
    toast.success('Платёж удалён')
  }

  const segments: KpiSegment[] = [
    {
      id:       'received',
      label:    'Получено',
      value:    '₸ ' + monthlyIncome.toLocaleString('ru-RU'),
      dotColor: 'var(--success)',
      meta:     'этот месяц',
      loading:  incomeLoading,
    },
    {
      id:       'count',
      label:    'Операций',
      value:    String(total),
      dotColor: 'var(--primary)',
      meta:     'всего записей',
      loading:  isLoading,
    },
    {
      id:       'avg',
      label:    'Средний чек',
      value:    '—',
      dotColor: 'var(--warning)',
      meta:     'нет данных',
    },
    {
      id:       'pending',
      label:    'Ожидается',
      value:    '₸ ' + monthlyExpected.toLocaleString('ru-RU'),
      dotColor: 'var(--purple)',
      meta:     'этот месяц',
      loading:  expectedLoading,
    },
  ]

  return (
    <>
      <HeaderPanel
        title="Платежи"
        subtitle={`${total} записей`}
        segments={segments}
        activeSegment={activeSegment}
        onSegmentChange={setActiveSegment}
      />

      <div className="border rounded-lg mt-4 overflow-hidden">
        <table className="w-full text-sm">
          <thead>
            <tr className="border-b bg-muted/40">
              <th className="text-left px-4 py-3 font-medium text-muted-foreground">Дата</th>
              <th className="text-left px-4 py-3 font-medium text-muted-foreground">Курс</th>
              <th className="text-right px-4 py-3 font-medium text-muted-foreground">Сумма</th>
              <th className="text-right px-4 py-3 font-medium text-muted-foreground">Уроков</th>
              <th className="w-20" />
            </tr>
          </thead>
          <tbody>
            {isLoading ? (
              [...Array(4)].map((_, i) => (
                <tr key={i}>
                  <td colSpan={5} className="px-4 py-3">
                    <div className="h-4 rounded bg-muted animate-pulse" />
                  </td>
                </tr>
              ))
            ) : payments.length === 0 ? (
              <tr>
                <td colSpan={5} className="px-4 py-6 text-center text-muted-foreground">
                  Нет оплат
                </td>
              </tr>
            ) : (
              payments.map((p) => (
                <tr
                  key={p.id}
                  className="border-b last:border-0 hover:bg-muted/30 cursor-pointer group"
                  onClick={() => router.push(`/courses/${p.course_id}`)}
                >
                  <td className="px-4 py-3 text-muted-foreground">
                    {new Date(p.paid_at).toLocaleDateString('ru-RU')}
                  </td>
                  <td className="px-4 py-3 font-medium">{courseMap[p.course_id] ?? '—'}</td>
                  <td className="px-4 py-3 text-right font-medium">{p.amount.toLocaleString()} ₸</td>
                  <td className="px-4 py-3 text-right text-muted-foreground">{p.lessons_count} ур.</td>
                  <td className="pr-2 py-3" onClick={(e) => e.stopPropagation()}>
                    <div className="flex items-center justify-end gap-1 opacity-0 group-hover:opacity-100 transition-opacity duration-150">
                      <Button
                        size="icon"
                        variant="ghost"
                        className="h-7 w-7"
                        onClick={() => openEdit(p)}
                      >
                        <Pencil className="h-3.5 w-3.5" />
                      </Button>
                      <Button
                        size="icon"
                        variant="ghost"
                        className="h-7 w-7 text-destructive hover:text-destructive"
                        onClick={() => handleDelete(p)}
                      >
                        <Trash2 className="h-3.5 w-3.5" />
                      </Button>
                    </div>
                  </td>
                </tr>
              ))
            )}
          </tbody>
        </table>
      </div>

      {totalPages > 1 && (
        <div className="flex items-center justify-between mt-3 px-1">
          <span className="text-xs text-muted-foreground">
            Страница {page} из {totalPages}
          </span>
          <Pagination page={page} totalPages={totalPages} onPageChange={handlePageChange} />
        </div>
      )}

      <PaymentForm
        open={formOpen}
        onClose={() => { setFormOpen(false); setEditingPayment(null) }}
        onSubmit={handleEdit}
        pricePerLesson={editingPayment ? (coursePriceMap[editingPayment.course_id] ?? 0) : 0}
        initialValues={
          editingPayment
            ? {
                amount:        editingPayment.amount,
                lessons_count: editingPayment.lessons_count,
                paid_at:       new Date(editingPayment.paid_at).toISOString().slice(0, 10),
              }
            : undefined
        }
        paymentId={editingPayment?.id}
      />
    </>
  )
}

export default function PaymentsPage() {
  return (
    <Suspense>
      <PaymentsPageInner />
    </Suspense>
  )
}
```

- [ ] **Step 2: Verify TypeScript**

```bash
cd frontend && npx tsc --noEmit 2>&1 | grep payments/page
```
Expected: no errors.

- [ ] **Step 3: Commit**

```bash
git add "frontend/src/app/(dashboard)/payments/page.tsx"
git commit -m "feat: add edit/delete actions to global payments list"
```

---

## Task 8: Course detail page — payment row actions

**Files:**
- Modify: `frontend/src/app/(dashboard)/courses/[id]/page.tsx`

- [ ] **Step 1: Add imports**

At the top of `frontend/src/app/(dashboard)/courses/[id]/page.tsx`:

**Add `Payment` to the `@/types/api` import.** Find:
```ts
import { Lesson } from '@/types/api'
```
Replace with:
```ts
import { Lesson, Payment } from '@/types/api'
```

The existing imports include `usePayments, useCreatePayment`. Replace that line with:

```ts
import { usePayments, useCreatePayment, useUpdatePayment, useDeletePayment } from '@/lib/hooks/usePayments'
```

Also add `Pencil, Trash2` to the lucide-react import. Find the line that imports from lucide-react and add them:

```ts
import { ..., Pencil, Trash2 } from 'lucide-react'
```

Add `PaymentFormValues` to the schema import if not already present (it already is).

- [ ] **Step 2: Add state and mutations near existing payment state**

Find the block in the component that has:
```ts
const [paymentFormOpen, setPaymentFormOpen] = useState(false)
```

After it, add:
```ts
const [editingPayment, setEditingPayment] = useState<Payment | null>(null)
```

Find where `createPayment` is defined:
```ts
const createPayment = useCreatePayment(id)
```

After it, add:
```ts
const updatePayment = useUpdatePayment(id)
const deletePayment = useDeletePayment(id)
```

- [ ] **Step 3: Add helper functions**

Find the existing `handlePaymentSubmit` function and replace it with:

```ts
async function handlePaymentSubmit(values: PaymentFormValues) {
  if (editingPayment) {
    await updatePayment.mutateAsync({
      id:   editingPayment.id,
      data: {
        amount:        values.amount,
        lessons_count: values.lessons_count,
        paid_at:       values.paid_at,
      },
    })
    toast.success('Платёж обновлён')
  } else {
    await createPayment.mutateAsync({
      course_id:     id,
      amount:        values.amount,
      lessons_count: values.lessons_count,
      paid_at:       values.paid_at,
    })
  }
}

async function handlePaymentDelete(p: Payment) {
  if (!confirm(`Удалить платёж на ${p.amount.toLocaleString()} ₸?`)) return
  await deletePayment.mutateAsync(p.id)
  toast.success('Платёж удалён')
}
```

- [ ] **Step 4: Update the payment row render and the PaymentForm call**

Find the payments section with `payments.map((p) => (` and replace each row with:

```tsx
<div
  key={p.id}
  className="flex items-center justify-between py-2 border-b last:border-0 text-sm group"
>
  <span className="text-muted-foreground">
    {new Date(p.paid_at).toLocaleDateString('ru-RU')}
  </span>
  <span className="font-medium">{p.amount.toLocaleString()} ₸</span>
  <span className="text-muted-foreground">{p.lessons_count} ур.</span>
  <div className="flex items-center gap-1 opacity-0 group-hover:opacity-100 transition-opacity duration-150">
    <Button
      size="icon"
      variant="ghost"
      className="h-7 w-7"
      onClick={() => { setEditingPayment(p); setPaymentFormOpen(true) }}
    >
      <Pencil className="h-3.5 w-3.5" />
    </Button>
    <Button
      size="icon"
      variant="ghost"
      className="h-7 w-7 text-destructive hover:text-destructive"
      onClick={() => handlePaymentDelete(p)}
    >
      <Trash2 className="h-3.5 w-3.5" />
    </Button>
  </div>
</div>
```

Find the `<PaymentForm` usage (near line 494) and replace it with:

```tsx
<PaymentForm
  open={paymentFormOpen}
  onClose={() => { setPaymentFormOpen(false); setEditingPayment(null) }}
  onSubmit={handlePaymentSubmit}
  pricePerLesson={course?.price_per_lesson ?? 0}
  initialValues={
    editingPayment
      ? {
          amount:        editingPayment.amount,
          lessons_count: editingPayment.lessons_count,
          paid_at:       new Date(editingPayment.paid_at).toISOString().slice(0, 10),
        }
      : undefined
  }
  paymentId={editingPayment?.id}
/>
```

- [ ] **Step 5: Verify TypeScript**

```bash
cd frontend && npx tsc --noEmit 2>&1 | grep courses
```
Expected: no errors.

- [ ] **Step 6: Commit**

```bash
git add "frontend/src/app/(dashboard)/courses/[id]/page.tsx"
git commit -m "feat: add edit/delete actions to course detail payment rows"
```

---

## Task 9: End-to-end smoke test

- [ ] **Step 1: Start the backend**

```bash
go build -o ./tmp/main.exe . && ./tmp/main.exe
```

- [ ] **Step 2: Start the frontend**

```bash
cd frontend && npm run dev
```

- [ ] **Step 3: Test global payments page**
  1. Open `/payments`
  2. Hover over a payment row — Pencil and Trash2 icons should appear
  3. Click Pencil → form opens with pre-filled values and title "Редактировать оплату"
  4. Change the amount and save → row updates without page reload
  5. Click Trash2 → `confirm()` dialog appears → confirm → row disappears

- [ ] **Step 4: Test course detail page**
  1. Open `/courses/{id}`
  2. Scroll to "Оплаты" section
  3. Hover over a payment row — icons appear
  4. Click Pencil → form opens with pre-filled values
  5. Save → balance recalculates (LessonsRemaining changes)
  6. Click Trash2 → confirm → row disappears, balance updates

- [ ] **Step 5: Final commit if any fixes were made**

```bash
git add -p
git commit -m "fix: payment edit/delete smoke test corrections"
```
