package models

import "time"

// LessonTask — задача (ДЗ), привязанная к уроку. Отдельная сущность от
// tutor-канбан Task (models/task.go): у той своя таблица и жизненный цикл.
type LessonTask struct {
	ID          string    `json:"id"`
	LessonID    string    `json:"lesson_id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Done        bool      `json:"done"`
	CreatedAt   time.Time `json:"created_at"`
}

type CreateLessonTaskRequest struct {
	Title       string `json:"title"       validate:"required,min=1,max=200"`
	Description string `json:"description" validate:"omitempty,max=1000"`
}

type UpdateLessonTaskRequest struct {
	Title       string `json:"title"       validate:"required,min=1,max=200"`
	Description string `json:"description" validate:"omitempty,max=1000"`
}

type SetLessonTaskDoneRequest struct {
	Done bool `json:"done"`
}
