package models

import "fmt"

// PdfImportPage — одна страница импорта внутри jsonb-поля pages.
// До start AssetID пуст; воркер выставляет Done по мере рендера.
type PdfImportPage struct {
	N       int     `json:"n"`
	AssetID string  `json:"asset_id"`
	W       float64 `json:"w"` // пункты PDF (1/72"), масштаб сцены применяет клиент
	H       float64 `json:"h"`
	Done    bool    `json:"done"`
}

type PdfImport struct {
	ID      string
	BoardID string
	PageID  string
	TutorID string
	S3Key   string
	Status  string
	Pages   []PdfImportPage
	Error   *string
}

// PdfPageAssetKey — единственное место, где рождается S3-ключ страницы:
// его пишут в board_assets при start и по нему же льёт воркер.
func PdfPageAssetKey(importID string, n int) string {
	return fmt.Sprintf("board-assets/pdf/%s/%d.jpg", importID, n)
}

// PageSizePt — габариты страницы из pdfinfo, в пунктах.
type PageSizePt struct {
	W float64 `json:"w"`
	H float64 `json:"h"`
}

type PdfPreflightResponse struct {
	ImportID  string       `json:"import_id"`
	NumPages  int          `json:"num_pages"`
	PageSizes []PageSizePt `json:"page_sizes"`
}

type PdfImportPageOut struct {
	FileID string  `json:"file_id"`
	URL    string  `json:"url"` // относительный /public/board-assets/{id}
	W      float64 `json:"w"`   // пункты
	H      float64 `json:"h"`
}

type PdfStartResponse struct {
	Pages []PdfImportPageOut `json:"pages"`
}

type StartPdfImportRequest struct {
	From int `json:"from" validate:"required,min=1"`
	To   int `json:"to" validate:"required,min=1"`
}
