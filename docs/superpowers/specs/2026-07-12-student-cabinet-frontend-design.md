# Кабинет ученика — frontend (v1)

**Дата:** 2026-07-12
**Статус:** утверждён к планированию
**Контекст:** backend аккаунтов учеников полностью в `main`: `/student/auth/*`
(accept-invite, login, refresh, logout), `/student/me`, `/student/lessons?filter=`,
`/student/password`, `/student/lessons/:id/room-token`,
`/student/lessons/:id/board-token`. Эта спека — весь frontend ученика плюс
недостающие хвосты в tutor-UI.

## Цель

Ученик по ссылке-приглашению создаёт аккаунт, логинится (телефон или username),
видит свои уроки с позицией в оплаченном цикле, входит в звонок с доской,
меняет пароль. Репетитор получает кнопку генерации ссылки-приглашения.

## Вне scope (осознанно)

- **Канбан-задачи ученика** — сущности «задание ученику» в backend нет
  (`tasks` — личный канбан репетитора). Отдельная спека; место на главной
  кабинета резервируется.
- Расписание/календарь, оплаты учеником, история прогресса, редактирование
  профиля учеником.
- E2E-тесты.

## 1. Изоляция auth-стека (ядро дизайна)

Новый `lib/api/studentClient.ts` — отдельный axios-инстанс, независимый от
tutor-стека (`client.ts` не трогаем):

- localStorage: `tg_student_token`, `tg_student_user` (не пересекаются с
  `tg_token`/`tg_user`); tutor и ученик могут быть залогинены в одном браузере
  одновременно.
- Request-interceptor: proactive refresh при < 7 дней до exp (как в tutor-клиенте),
  refresh через `POST /student/auth/refresh` с `withCredentials` (refresh-cookie
  ученика на backend уже отдельный).
- Response-interceptor: 401 → refresh → retry; hard-fail refresh →
  `forceStudentLogout()` (чистка storage + redirect `/student/login`).
  Защита от 429-шторма (флаг `refreshFailed`) — копируем из tutor-клиента.
  **Без 402-ветки**: подписка — забота репетитора.
- Нормализация ошибок в тот же `ApiError` (message / fieldErrors / status).

`stores/studentAuth.ts` — zustand-store по образу `stores/auth.ts`, тип
пользователя — `StudentProfile` (`first_name`, `last_name`, `phone`, `username`).

`lib/api/student.ts` — методы API: `acceptInvite(token, {username, password})`,
`login({identifier, password})`, `logout()`, `me()`, `lessons(filter)`,
`changePassword({old_password, new_password})`, `roomToken(lessonId)`,
`boardToken(lessonId)`.

Решение: отдельный клиент-копия (~100 строк), а не фабрика поверх tutor-клиента —
не трогаем обкатанный tutor-стек, student-версия проще (нет 402).

## 2. Маршруты — route group `(student)`

```
/student/login              — вход: телефон ИЛИ username + пароль
/student/invite/[token]     — принять приглашение: username + пароль ×2
/student/lessons            — главная: уроки upcoming/past + цикл   [гейт]
/student/lessons/[id]/call  — звонок + доска (CallRoom)             [гейт]
/student/profile            — профиль + смена пароля                [гейт]
```

- Layout группы: гейт по `tg_student_token` — нет токена → redirect
  `/student/login?next=<путь>`; login после успеха уважает `next`.
- Шапка: логотип, имя ученика, ссылка «Профиль», кнопка «Выйти»
  (`POST /student/auth/logout` + чистка store). Одна колонка `max-w-3xl` —
  основные устройства: ноутбук/планшет. Без tutor-shell (Sidebar/BottomNav).
- UI-кит переиспользуем: SectionCard, токены, Button/Input/Dialog.
- `/student/invite/[token]`: форма username + пароль ×2; истёкший/неизвестный
  токен → человекочитаемая ошибка «Ссылка недействительна, попросите репетитора
  прислать новую». Успех → сессия выдана backend'ом → redirect `/student/lessons`.
- `/student/login`: одно поле «Телефон или логин» (`identifier`) + пароль.

## 3. Главная: уроки + оплаченный цикл

- Вкладки «Ближайшие» / «Прошедшие» → `GET /student/lessons?filter=upcoming|past`
  (state в URL `?tab=`, как на странице курсов).
- Карточка урока: предмет, дата/время (локаль ru), длительность, статус,
  для группового — бейдж «Группа», бейдж «Урок N из M» при наличии
  `cycle_position`/`cycle_size`.
- Кнопка «Войти в урок» → `/student/lessons/[id]/call` — на уроках со статусом
  scheduled (ближайшие).
- Пустые состояния: «Ближайших уроков нет» / «Прошедших уроков нет».

### Backend-расширение (единственное в спеке)

`repository/student.go ListLessons`: добавить расчёт `cycle_position` /
`cycle_size` тем же ROW_NUMBER-приёмом, что в tutor-календаре
(`rank` по курсу → позиция в цикле по `courses.lessons_per_cycle`).
Поля в `models.CalendarLesson` уже есть (`omitempty`) — контракт не ломается.
Обновить service/handler не нужно (проброс той же модели); тест на repo-уровне
не требуется (нет инфраструктуры), проверка — существующие service-тесты +
ручной smoke.

## 4. Звонок + доска

Страница `/student/lessons/[id]/call` (client component):

1. Поллинг `GET /public/lessons/:id/room-status` каждые 5 с (эндпоинт публичный,
   живой — тот же паттерн, что был в гостевом входе). До `active` — экран
   «Урок ещё не начался, ожидаем…»; `ended` — «Урок завершён».
2. `active` → `POST /student/lessons/:id/room-token` → рендер `CallRoom`
   (`serverUrl`, `token`, роль guest-типа, `enableMedia`).
3. Доска: перед рендером `CallRoom` получить
   `GET /student/lessons/:id/board-token` → `{invite_token, page_id}` →
   передать в существующий guest-путь `CallRoom` (`joinByInvite`).
   При открытии доски токен запрашивается свежий (invite ротируется на backend
   при каждой выдаче — протухший из-за чужого запроса invite перезапрашиваем).
4. Disconnect → повторная проверка room-status: `ended` → экран завершения,
   иначе возврат к ожиданию.

**Board WS live-auth (follow-up #1) закрывается без изменений WS-хаба:**
invite-UUID выдаётся только после enrollment-проверки и ротируется при каждой
выдаче — этого достаточно для v1. `authorizeWS` не трогаем.

## 5. Хвосты в tutor-UI (входят в scope)

- **Кнопка «Пригласить в кабинет»** в списке учеников (`StudentsList`):
  `POST /students/:id/invite` → диалог со ссылкой
  `${origin}/student/invite/{invite_token}` и кнопкой «Скопировать»
  (+ срок действия из `expires_at`). Сейчас эндпоинт с фронта не вызывается —
  ученикам неоткуда взяться.
- **Мёртвый гостевой вход** `/join/[lessonId]` (вызывает удалённый
  `guest-token`): страницу заменить на redirect →
  `/student/login?next=/student/lessons`. Метод `getGuestToken` из
  `lib/api/calls.ts` удалить, если не используется quick-room-путём.
- **`inviteUrl` в call-странице репетитора** (`(call)/lessons/[id]/call`):
  `${origin}/join/${id}` → `${origin}/student/lessons/${id}/call` —
  залогиненный ученик попадает прямо в урок, незалогиненный — через login с
  `next`.
- Quick-rooms (`/join/room/[id]`, `/public/quick/*`) не трогаем.

## 6. Профиль и смена пароля

`/student/profile`:

- Карточка данных из `GET /student/me` — read-only (имя, телефон, username).
- Форма смены пароля: старый, новый ×2 (клиентская проверка совпадения),
  min 6 символов. `POST /student/password`:
  - 401 → «Неверный текущий пароль» под полем.
  - 200 → в ответе свежий `access_token` (+ Set-Cookie refresh) — **сохранить
    токен в store/storage**, иначе следующий запрос уйдёт с отозванной сессией;
    тост «Пароль изменён. Другие устройства разлогинены», форма очищается.

## 7. Обработка ошибок

- Гейт-layout: отсутствие токена → login с `next`; протухший токен чинит
  interceptor (refresh → retry), мёртвая сессия → forceStudentLogout.
- `room-token`/`board-token` 403 (выписали из курса между рендером и кликом) →
  экран «Нет доступа к уроку» со ссылкой на `/student/lessons`.
- Сетевые ошибки списков — стандартный error-state SectionCard с retry.

## 8. Тестирование

- `tsc --noEmit` + `next build` зелёные — обязательная планка каждой задачи.
- Чистая логика при наличии (напр., нормализация ошибок studentClient) —
  `node:test` + TS-strip по существующему паттерну репо.
- Ручной smoke-чеклист в конце: invite → аккаунт → login → уроки → звонок+доска
  → смена пароля → повторный login.
- Backend-расширение ListLessons: `go build ./... && go vet ./... && make test`.

## Порядок реализации (волны)

1. **Ядро (последовательно):** studentClient + studentAuth store + student API
   + route group layout с гейтом. Плюс backend-расширение ListLessons.
2. **Страницы (параллельно, файлы не пересекаются):**
   login + invite; главная (уроки+цикл); профиль; call-страница.
3. **Tutor-хвосты (параллельно с волной 2):** кнопка «Пригласить», redirect
   `/join/[lessonId]`, замена inviteUrl.
