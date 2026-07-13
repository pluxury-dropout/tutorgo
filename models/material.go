package models

import "time"

type Material struct {
	ID        string    `json:"id"`
	TutorID   string    `json:"tutor_id"`
	ParentID  *string   `json:"parent_id"`
	Kind      string    `json:"kind"` // "folder" | "file"
	Name      string    `json:"name"`
	FilePath  string    `json:"-"` // ключ в S3, наружу не отдаём
	MimeType  string    `json:"mime_type"`
	SizeBytes int       `json:"size_bytes"`
	CreatedAt time.Time `json:"created_at"`
}

type CreateFolderRequest struct {
	Name     string  `json:"name"      validate:"required,min=1,max=100"`
	ParentID *string `json:"parent_id" validate:"omitempty,uuid"`
}

type MaterialResponse struct {
	ID        string    `json:"id"`
	Kind      string    `json:"kind"`
	Name      string    `json:"name"`
	MimeType  string    `json:"mime_type"`
	SizeBytes int       `json:"size_bytes"`
	CreatedAt time.Time `json:"created_at"`
}

func NewMaterialResponse(m Material) MaterialResponse {
	return MaterialResponse{
		ID:        m.ID,
		Kind:      m.Kind,
		Name:      m.Name,
		MimeType:  m.MimeType,
		SizeBytes: m.SizeBytes,
		CreatedAt: m.CreatedAt,
	}
}
