# Payment CRUD — Delete & Update

**Date:** 2026-06-01

## Goal

Add delete and edit (update) actions to payment rows in both `/payments` and `/courses/[id]` pages, backed by a new `PUT /payments/:id` API endpoint.

## Backend

### New endpoint: `PUT /payments/:id`

**Request model** (`models.UpdatePaymentRequest`):
```go
type UpdatePaymentRequest struct {
  Amount       *float64 `json:"amount"       validate:"omitempty,gt=0"`
  LessonsCount *int     `json:"lessons_count" validate:"omitempty,gt=0"`
  PaidAt       *string  `json:"paid_at"      validate:"omitempty"`
}
```
All fields optional (pointer types). At least one must be present.

**Repository** — `Update(ctx, id, tutorID, req)` runs a single `UPDATE payments SET ... WHERE id=$1 AND course_id IN (SELECT id FROM courses WHERE tutor_id=$2)`. Returns `ErrNotFound` (via `errors.New`) if `RowsAffected == 0`.

**Service** — `Update(ctx, id, tutorID, req)` delegates to repo, wraps not-found as `ErrNotFound`.

**Handler** — `PUT /payments/:id`, same tutorID guard pattern. Returns updated `Payment`.

**Router** — registered alongside existing payment routes.

## Frontend

### API client (`lib/api/payments.ts`)
Add:
- `update(id, data: Partial<PaymentInput>) → Promise<Payment>`
- `delete(id) → Promise<void>`

### Hooks (`lib/hooks/usePayments.ts`)
Add:
- `useUpdatePayment(courseId?)` — invalidates `byCourse`, `list`, `recent`, `monthlyIncome`
- `useDeletePayment(courseId?)` — same invalidations

### PaymentForm (`components/payments/PaymentForm.tsx`)
- Add optional `initialValues?: PaymentFormValues` prop
- Add optional `paymentId?: string` prop
- Title becomes "Новая оплата" or "Редактировать оплату" based on presence of `paymentId`
- On submit: calls `onSubmit` (parent decides create vs update)

### Table rows (both pages)

Replace `ChevronRight` column with an actions column:
- On row hover: show `Pencil` and `Trash2` icons (opacity-0 → opacity-100)
- Clicking `Pencil`: opens `PaymentForm` with `initialValues` filled from the row
- Clicking `Trash2`: opens `AlertDialog` confirmation, on confirm calls `deletePayment.mutateAsync(id)`
- Row `onClick` (navigate to course) fires only when clicking outside the icons

### AlertDialog
Reuse shadcn `AlertDialog`. Message: "Удалить платёж на {amount} ₸? Это действие нельзя отменить."

## Affected files

| File | Change |
|------|--------|
| `models/payment.go` | Add `UpdatePaymentRequest` |
| `repository/payment.go` | Add `Update` to interface + impl |
| `service/payment.go` | Add `Update` to interface + impl |
| `handlers/payment.go` | Add `Update` handler |
| `handlers/mocks_test.go` | Add `Update` to `mockPaymentService` |
| `service/payment_test.go` | Add `Update` to `mockPaymentRepo` |
| `router/router.go` | Register `PUT /payments/:id` |
| `lib/api/payments.ts` | Add `update`, `delete` |
| `lib/hooks/usePayments.ts` | Add `useUpdatePayment`, `useDeletePayment` |
| `components/payments/PaymentForm.tsx` | Support edit mode |
| `app/(dashboard)/payments/page.tsx` | Actions column |
| `app/(dashboard)/courses/[id]/page.tsx` | Actions column |
