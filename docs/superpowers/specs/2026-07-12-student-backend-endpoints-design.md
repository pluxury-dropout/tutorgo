# Student backend endpoints (v1) — design

**Дата:** 2026-07-12
**Статус:** утверждён к реализации
**Контекст:** аккаунты учеников, backend v1 (invite→accept, login, refresh/logout,
room-token, board-token) уже влит в `main`. Эта спека закрывает недостающие
data-эндпоинты, нужные кабинету ученика. Frontend — **отдельная спека** после этой.

## Цель

Дать залогиненному ученику (JWT `role:"student"`) минимальный API, чтобы кабинет
мог: показать список уроков (ближайшие + история), показать профиль, сменить пароль.

Всё под `middleware.AuthStudent` в группе `/student`, **без** subscription-гейта
(подписка — забота репетитора). Реализация по паттерну фичи: repo → service →
handler → router + тесты.

## Вне скоупа (осознанно)

- **Board WS live-auth (follow-up #1)** — переносится во frontend-спеку. Live-
  перепроверку `EnrolledInLesson` на WS-коннекте доски нельзя проверить end-to-end
  без WS-клиента, а выбор типа токена (короткоживущий подписанный board-token vs
  invite-UUID vs student-JWT в `?token=`) — security-решение, которое делается рядом
  с клиентом. Дыры нет: WS-клиента ученика пока не существует.
- Аудит-лог смены пароля, `updated_at`.

## Эндпоинты

### 1. `GET /student/lessons?filter=upcoming|past`

Список уроков ученика, отфильтрованных по его enrollment.

- **Query-параметр** `filter`: `upcoming` (дефолт при пустом/неизвестном значении)
  или `past`.
  - `upcoming`: `scheduled_at >= now()`, сортировка ASC.
  - `past`: `scheduled_at < now()`, сортировка DESC (для истории).
- **Ответ** `200`: `[]models.CalendarLesson` (существующая модель — несёт `subject`,
  `scheduled_at`, `duration_minutes`, `status`, `is_group`, `student_name`).
  Пустой список → `[]`, не `null`.
- **Скоуп:** `studentID` из контекста. Enrollment-джойн — тот же, что в
  `EnrolledInLesson`:
  ```sql
  FROM lessons l JOIN courses c ON c.id = l.course_id
  WHERE (c.student_id = $1
         OR EXISTS (SELECT 1 FROM course_enrollments ce
                    WHERE ce.course_id = c.id AND ce.student_id = $1))
    AND l.scheduled_at >= now()   -- или < now() для past
  ORDER BY l.scheduled_at ASC     -- или DESC для past
  ```

**Слои:**
- `repository/student.go`: `ListLessons(ctx, studentID string, past bool) ([]models.CalendarLesson, error)`.
- `service/student.go`: `ListLessons(ctx, studentID string, past bool) (...)` — проброс.
- `handlers/student.go` (или новый `student_lessons.go`): парсит `?filter`, мапит в
  `past bool`, зовёт сервис. Один эндпоинт с флагом вместо двух — меньше кода.

**Решение — одна repo-функция с булевым флагом**, а не две почти одинаковые. `past`
меняет только знак сравнения и направление сортировки; тело запроса ветвится по
флагу.

### 2. `GET /student/me`

Профиль текущего ученика.

- **Ответ** `200`: `models.StudentProfile` — `{ "first_name", "last_name", "phone",
  "username" }`. `username` — колонка из миграции 020. Отдельная модель, чтобы не
  тащить в ответ tutor-scoped поля из `Student` (`tutor_id`, `notes`, `active`).
- **Слои:** `repository/student.go`: `GetProfile(ctx, studentID) (models.StudentProfile, error)`;
  service-проброс; handler.

### 3. `POST /student/password`

Смена пароля залогиненным учеником.

- **Тело:** `{ "old_password", "new_password" }`, валидация обоих `required,min=6`
  (модель `models.ChangePasswordRequest`).
- **Логика:**
  1. Fetch `password_hash` по `studentID`.
  2. `bcrypt.CompareHashAndPassword(hash, old_password)` — при несовпадении `401`
     `{"error":"invalid credentials"}`.
  3. `bcrypt.GenerateFromPassword(new_password)` → update `password_hash`.
  4. **Revoke всех refresh-токенов ученика** (`RevokeAllForStudent`) — смена пароля
     выкидывает остальные устройства.
  5. Выдать **свежую сессию текущему устройству** (переиспользовать `issueSession`
     из `StudentAuthHandler`: новый access-JWT + новый refresh-cookie), чтобы
     инициатор не разлогинился шагом 4.
- **Ответ** `200`: `models.LoginResponse` (новый `access_token`) + `Set-Cookie`
  свежего refresh-токена. По симметрии с accept-invite/login.

**Слои:**
- `repository/student.go`: `GetPasswordHash(ctx, studentID) (string, error)`,
  `UpdatePassword(ctx, studentID, hash string) error`.
- `repository/student_refresh_token.go`: `RevokeAllForStudent(ctx, studentID) error`
  (`UPDATE ... SET revoked_at = now() WHERE student_id = $1 AND revoked_at IS NULL`).
- Handler — в `StudentAuthHandler` (у него уже есть `refreshSvc`, `issueSession`,
  jwt-secret). Метод `ChangePassword(c)`.

## Роутинг

```go
// router.go, в группе stu := r.Group("/student"); stu.Use(middleware.AuthStudent(...))
stu.GET("/lessons", studentHandler.ListLessons)
stu.GET("/me", studentHandler.Me)
stu.POST("/password", studentAuthHandler.ChangePassword)
// (существующие room-token / board-token остаются)
```

`GET /student/me` и `GET /student/lessons` — на `studentHandler`; `ChangePassword`
— на `studentAuthHandler` (нужен доступ к refresh-сервису и выдаче сессии).

## Обработка ошибок

- Нет `studentID` в контексте → `401` (nil-guard, как в остальных student-хендлерах).
- Неизвестный `?filter` → трактуем как `upcoming` (не 400 — толерантно).
- Смена пароля, неверный старый → `401`.
- Ошибка БД → `500` с логом (`h.log.Error`).

## Тестирование

Service-слой (`service/student_test.go`, testify/mock):
- `ListLessons` upcoming/past — прокидывает `past` в repo, возвращает список.
- `GetProfile` — успех.
- change-password путь тестируется на handler-слое (там bcrypt + revoke + issue).

Handler-слой (`handlers/student_test.go` / `student_auth_test.go`):
- `ListLessons`: 200 + JSON-массив; неизвестный filter → upcoming.
- `Me`: 200 + поля профиля.
- `ChangePassword`: неверный старый пароль → 401 (revoke/issue не вызваны);
  успех → 200 + новый access-token + Set-Cookie + `RevokeAllForStudent` вызван.

## Порядок реализации

1. Модели (`StudentProfile`, `ChangePasswordRequest`, `Username` в `Student` при нужде).
2. Repo-методы (`ListLessons`, `GetProfile`, `GetPasswordHash`, `UpdatePassword`,
   `RevokeAllForStudent`).
3. Service-методы (`ListLessons`, `GetProfile`) + расширение интерфейсов.
4. Handlers (`ListLessons`, `Me` на `StudentHandler`; `ChangePassword` на
   `StudentAuthHandler`).
5. Роуты в `router.go`.
6. Тесты (service + handler).
