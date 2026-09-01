package models

import "time"

// CalendarItem — одна строка единой ленты календаря. Общие поля наверху, чтобы
// фронт рисовал сетку, не разбираясь в типе; специфика лежит во вложенном
// объекте по имени типа и нужна только для бейджей и поповеров.
//
// Вложенные модели переиспользуются целиком, а не режутся на «часть для
// календаря»: урезанная копия неизбежно теряет поле, которое понадобится
// поповеру, и разъезжается с оригиналом при первой же правке.
type CalendarItem struct {
	Type            string    `json:"type"` // lesson | event | task
	ID              string    `json:"id"`
	Title           string    `json:"title"`
	StartsAt        time.Time `json:"starts_at"`
	DurationMinutes int       `json:"duration_minutes"`

	Lesson *CalendarLesson `json:"lesson,omitempty"`
	Event  *Event          `json:"event,omitempty"`
	Task   *Task           `json:"task,omitempty"`
}

// LessonTitle собирает заголовок урока: у группового курса это предмет,
// у индивидуального — «Предмет — Ученик».
func LessonTitle(l CalendarLesson) string {
	if l.IsGroup || l.StudentName == nil || *l.StudentName == "" {
		return l.Subject
	}
	return l.Subject + " — " + *l.StudentName
}
