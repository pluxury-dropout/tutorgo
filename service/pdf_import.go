package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"tutorgo/models"
	"tutorgo/repository"
)

type PdfImportService interface {
	// CreateImport — preflight: файл уже в S3, здесь ownership и запись pending.
	CreateImport(ctx context.Context, boardID, pageID, tutorID, s3Key string, sizes []models.PageSizePt) (models.PdfPreflightResponse, error)
	// Start фиксирует диапазон, создаёт ассеты и ставит джобу (транзакционно в repo).
	Start(ctx context.Context, importID, tutorID string, from, to int) (models.PdfStartResponse, error)
}

type pdfImportService struct {
	repo    repository.PdfImportRepository
	enqueue func(ctx context.Context, tx pgx.Tx, importID string) error
}

func NewPdfImportService(repo repository.PdfImportRepository, enqueue func(ctx context.Context, tx pgx.Tx, importID string) error) PdfImportService {
	return &pdfImportService{repo: repo, enqueue: enqueue}
}

func (s *pdfImportService) CreateImport(ctx context.Context, boardID, pageID, tutorID, s3Key string, sizes []models.PageSizePt) (models.PdfPreflightResponse, error) {
	ok, err := s.repo.BoardBelongsToTutor(ctx, boardID, tutorID)
	if err != nil {
		return models.PdfPreflightResponse{}, err
	}
	if !ok {
		return models.PdfPreflightResponse{}, ErrForbidden
	}
	ok, err = s.repo.PageBelongsToBoard(ctx, pageID, boardID)
	if err != nil {
		return models.PdfPreflightResponse{}, err
	}
	if !ok {
		return models.PdfPreflightResponse{}, ErrForbidden
	}

	pages := make([]models.PdfImportPage, len(sizes))
	for i, sz := range sizes {
		pages[i] = models.PdfImportPage{N: i + 1, W: sz.W, H: sz.H}
	}
	id, err := s.repo.Create(ctx, boardID, pageID, tutorID, s3Key, pages)
	if err != nil {
		return models.PdfPreflightResponse{}, err
	}
	return models.PdfPreflightResponse{ImportID: id, NumPages: len(sizes), PageSizes: sizes}, nil
}

func (s *pdfImportService) Start(ctx context.Context, importID, tutorID string, from, to int) (models.PdfStartResponse, error) {
	imp, err := s.repo.GetByID(ctx, importID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return models.PdfStartResponse{}, fmt.Errorf("import: %w", ErrNotFound)
		}
		return models.PdfStartResponse{}, err
	}
	if imp.TutorID != tutorID {
		return models.PdfStartResponse{}, ErrForbidden
	}
	selected, err := s.repo.Start(ctx, importID, from, to, func(tx pgx.Tx) error {
		return s.enqueue(ctx, tx, importID)
	})
	if err != nil {
		// ErrImportAlreadyStarted/ErrBadPageRange — наши безопасные сообщения,
		// клиент может показать err.Error() напрямую (см. handleServiceError).
		if errors.Is(err, repository.ErrImportAlreadyStarted) || errors.Is(err, repository.ErrBadPageRange) {
			return models.PdfStartResponse{}, fmt.Errorf("%v: %w", err, ErrBadRequest)
		}
		return models.PdfStartResponse{}, err
	}
	out := make([]models.PdfImportPageOut, len(selected))
	for i, p := range selected {
		out[i] = models.PdfImportPageOut{
			FileID: p.AssetID,
			URL:    "/public/board-assets/" + p.AssetID,
			W:      p.W,
			H:      p.H,
		}
	}
	return models.PdfStartResponse{Pages: out}, nil
}
