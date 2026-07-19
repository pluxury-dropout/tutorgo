package models

import (
	"time"
)

type Board struct {
	ID        string    `json:"id"`
	CourseID  string    `json:"course_id"`
	TutorID   string    `json:"tutor_id"`
	CreatedAt time.Time `json:"created_at"`
}

type BoardPage struct {
	ID      string `json:"id"`
	BoardID string `json:"board_id"`
	Title   string `json:"title"`
	// Снапшот сюда не кладётся: клиент получает его по WS при подключении к
	// странице, а в списке страниц он раздувал ответ на сотни килобайт.
	Position  int       `json:"position"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type BoardAsset struct {
	ID        string    `json:"id"`
	BoardID   string    `json:"board_id"`
	FilePath  string    `json:"file_path"`
	MimeType  string    `json:"mime_type"`
	SizeBytes int       `json:"size_bytes"`
	CreatedAt time.Time `json:"created_at"`
}

type BoardInvite struct {
	ID        string    `json:"id"`
	BoardID   string    `json:"board_id"`
	CreatedAt time.Time `json:"created_at"`
}

type CreateBoardPageRequest struct {
	Title string `json:"title" validate:"required,min=1,max=100"`
}

type UpdateBoardPageRequest struct {
	Title    *string `json:"title"    validate:"omitempty,min=1,max=100"`
	Position *int    `json:"position" validate:"omitempty,min=0"`
}

type BoardWithPages struct {
	Board
	Pages []BoardPage `json:"pages"`
}

type BoardAssetResponse struct {
	ID  string `json:"id"`
	URL string `json:"url"`
}
