# Постоянный доступ ученика к доске + отказ от страниц доски

**Дата:** 2026-07-13

## Проблема

Ученик попадает на доску только через урок: `GET /student/lessons/:id/board-token`
проверяет запись на урок и выдаёт invite-токен. Во фронте кнопка «Доска» видна
только для ближайших уроков в статусе `scheduled`. Делать ДЗ на доске между
уроками невозможно.

Дополнительно: `repository.CreateInvite` ротирует токен на каждом вызове
(`ON CONFLICT (board_id) DO UPDATE SET id = gen_random_uuid()`), поэтому
постоянной ссылки на доску сейчас не существует в принципе. В групповом курсе
второй ученик, запросивший токен, инвалидирует ссылку первого.

## Решение

Доска уже привязана к **курсу** (`boards.course_id`), а не к уроку. Меняем точку
входа с «урок → токен» на «курс → токен». Урок был лишь косвенной проверкой того
же факта — принадлежности ученика курсу.

Параллельно убираем UI страниц доски: доска одна, без страниц. Схема БД
(`board_pages`) и backend-эндпоинты страниц остаются нетронутыми на будущее.

## Часть A: постоянный доступ

### 1. Стабильный invite-токен

`repository/whiteboard.go` → `CreateInvite`: `ON CONFLICT (board_id)` больше не
меняет `id`. Возвращается существующий invite. Токен становится «ключом от
двери» вместо одноразового пропуска.

Размен: единственный способ отзыва ссылки — `DeleteInvite` (эндпоинт уже есть,
кнопки в UI репетитора нет). Осознанно принимаем: ротация при каждом «Пригласить
ученика» несовместима с постоянным доступом.

### 2. Список курсов ученика

`repository/student.go` → `ListCourses(ctx, studentID) ([]models.StudentCourse, error)`.
Тот же enrollment-джойн, что в `ListHomework`: `courses.student_id = $1` ИЛИ
`EXISTS (course_enrollments)`. Возвращает `id, subject, tutor_id`.
Пробрасывается через `service/student.go` без логики.

`models.StudentCourse`: `ID`, `Subject`, `TutorID` (последний — только для
внутреннего использования, в JSON не отдаём).

### 3. Роуты (группа `stu`, `middleware.AuthStudent`)

- `GET /student/courses` → `[]{id, subject}` для страницы выбора доски.
- `GET /student/courses/:id/board-token` → `{invite_token, page_id}`.
  Логика — копия `WhiteboardHandler.StudentBoardToken` с заменой проверки урока
  на проверку курса: курс должен быть в `ListCourses(studentID)`; оттуда же
  берём `tutorID`; далее `GetOrCreateBoard(courseID, tutorID)` + `CreateInvite`.
  `page_id` = первая страница доски.

Старый `GET /student/lessons/:id/board-token` остаётся — кнопка «Доска» у урока
продолжает работать как ярлык.

### 4. Frontend

- `student/(cabinet)/layout.tsx`: пункт «Доска» в навигации.
- `student/(cabinet)/board/page.tsx`: карточки курсов (`SectionCard`), кнопка
  «Открыть доску» → `studentApi.courseBoardToken(id)` → `router.push('/board/join/'+token)`.
- `lib/api/student.ts`: `courses()`, `courseBoardToken(courseId)`.

Гостевую страницу доски (`/board/join/[token]`) править не нужно.

## Часть B: убрать UI страниц

- Удалить `components/whiteboard/BoardPageMenu.tsx`.
- В `ExcalidrawCanvas.tsx` убрать из `renderTopRightUI` дропдаун «Урок ▾»,
  состояние `pagesOpen` и связанный `useEffect` закрытия по клику вне.
- Удалить хуки `useCreatePage` / `useDeletePage` из `lib/hooks/useWhiteboard.ts`
  (единственный потребитель — удаляемый `BoardPageMenu`).
- Кнопка вставки картинки в `renderTopRightUI` остаётся.
- Доска всегда работает с `pages[0]`.

Backend страниц (`POST /boards/:boardId/pages`, `PUT`/`DELETE /board-pages/:pageId`,
таблица `board_pages`) **не трогаем**. `GetOrCreateBoard` продолжает создавать
первую страницу автоматически. Возврат UI страниц = откат одного коммита.

## Тесты

- `service/student_test.go`: `ListCourses` — индивидуальный и групповой курс.
- `handlers/whiteboard_student_test.go`: `board-token` по курсу — 403 для чужого
  курса, 200 + токен для своего.
- `repository`-тестов на стабильность invite нет (нет инфраструктуры БД-тестов);
  проверяем ручным smoke: два вызова `board-token` подряд дают один токен.

## Вне scope

- Отдельные «личные страницы ДЗ» для учеников (доска общая на курс).
- Read-only режим доски.
- Кнопка отзыва invite-ссылки в UI репетитора.
- Миграции БД.
