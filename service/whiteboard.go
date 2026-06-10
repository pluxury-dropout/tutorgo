package service

import (
	"context"
	"encoding/json"
	"tutorgo/models"
	"tutorgo/repository"
)

type WhiteboardService interface {
	GetOrCreateBoard(ctx context.Context, courseID, tutorID string) (models.BoardWithPages, error)
	ValidateInvite(ctx context.Context, inviteID string) (models.BoardWithPages, error)
	CreatePage(ctx context.Context, boardID, tutorID, title string) (models.BoardPage, error)
	UpdatePage(ctx context.Context, pageID, boardID string, req models.UpdateBoardPageRequest) (models.BoardPage, error)
	DeletePage(ctx context.Context, pageID, boardID string) error
	SaveSnapshot(ctx context.Context, pageID string, snapshot json.RawMessage) error
	CreateInvite(ctx context.Context, boardID string) (models.BoardInvite, error)
	DeleteInvite(ctx context.Context, boardID string) error
	SaveAsset(ctx context.Context, boardID, filePath, mimeType string, sizeBytes int) (models.BoardAsset, error)
	GetAsset(ctx context.Context, assetID string) (models.BoardAsset, error)
	GetPageSnapshot(ctx context.Context, pageID string) (json.RawMessage, error)
}

type whiteboardService struct {
	repo repository.WhiteboardRepository
}

func NewWhiteboardService(repo repository.WhiteboardRepository) WhiteboardService {
	return &whiteboardService{repo: repo}
}

func (s *whiteboardService) GetOrCreateBoard(ctx context.Context, courseID, tutorID string) (models.BoardWithPages, error) {
	board, err := s.repo.GetOrCreateBoard(ctx, courseID, tutorID)
	if err != nil {
		return models.BoardWithPages{}, err
	}
	pages, err := s.repo.GetPagesByBoard(ctx, board.ID)
	if err != nil {
		return models.BoardWithPages{}, err
	}
	if len(pages) == 0 {
		first, err := s.repo.CreatePage(ctx, board.ID, "Страница 1", 0)
		if err != nil {
			return models.BoardWithPages{}, err
		}
		pages = []models.BoardPage{first}
	}
	return models.BoardWithPages{Board: board, Pages: pages}, nil
}

func (s *whiteboardService) ValidateInvite(ctx context.Context, inviteID string) (models.BoardWithPages, error) {
	board, err := s.repo.GetBoardByInvite(ctx, inviteID)
	if err != nil {
		return models.BoardWithPages{}, ErrNotFound
	}
	pages, err := s.repo.GetPagesByBoard(ctx, board.ID)
	if err != nil {
		return models.BoardWithPages{}, err
	}
	return models.BoardWithPages{Board: board, Pages: pages}, nil
}

func (s *whiteboardService) CreatePage(ctx context.Context, boardID, tutorID, title string) (models.BoardPage, error) {
	pages, err := s.repo.GetPagesByBoard(ctx, boardID)
	if err != nil {
		return models.BoardPage{}, err
	}
	return s.repo.CreatePage(ctx, boardID, title, len(pages))
}

func (s *whiteboardService) UpdatePage(ctx context.Context, pageID, boardID string, req models.UpdateBoardPageRequest) (models.BoardPage, error) {
	page, err := s.repo.GetPageByID(ctx, pageID)
	if err != nil || page.BoardID != boardID {
		return models.BoardPage{}, ErrNotFound
	}
	return s.repo.UpdatePage(ctx, pageID, req)
}

func (s *whiteboardService) DeletePage(ctx context.Context, pageID, boardID string) error {
	return s.repo.DeletePage(ctx, pageID, boardID)
}

func (s *whiteboardService) SaveSnapshot(ctx context.Context, pageID string, snapshot json.RawMessage) error {
	return s.repo.SaveSnapshot(ctx, pageID, snapshot)
}

func (s *whiteboardService) CreateInvite(ctx context.Context, boardID string) (models.BoardInvite, error) {
	return s.repo.CreateInvite(ctx, boardID)
}

func (s *whiteboardService) DeleteInvite(ctx context.Context, boardID string) error {
	return s.repo.DeleteInvite(ctx, boardID)
}

func (s *whiteboardService) SaveAsset(ctx context.Context, boardID, filePath, mimeType string, sizeBytes int) (models.BoardAsset, error) {
	return s.repo.CreateAsset(ctx, boardID, filePath, mimeType, sizeBytes)
}

func (s *whiteboardService) GetAsset(ctx context.Context, assetID string) (models.BoardAsset, error) {
	return s.repo.GetAsset(ctx, assetID)
}

func (s *whiteboardService) GetPageSnapshot(ctx context.Context, pageID string) (json.RawMessage, error) {
	return s.repo.GetPageSnapshot(ctx, pageID)
}
