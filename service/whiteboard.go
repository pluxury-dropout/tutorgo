package service

import (
	"context"
	"encoding/json"
	"tutorgo/models"
	"tutorgo/repository"
)

type WhiteboardService interface {
	GetOrCreateBoard(ctx context.Context, courseID, tutorID string) (models.BoardWithPages, error)
	GetOrCreateTrialBoard(ctx context.Context, tutorID string) (models.BoardWithPages, error)
	ValidateInvite(ctx context.Context, inviteID string) (models.BoardWithPages, error)
	CreatePage(ctx context.Context, boardID, tutorID, title string) (models.BoardPage, error)
	UpdatePage(ctx context.Context, pageID, tutorID string, req models.UpdateBoardPageRequest) (models.BoardPage, error)
	DeletePage(ctx context.Context, pageID, tutorID string) error
	SaveSnapshot(ctx context.Context, pageID string, snapshot json.RawMessage) error
	CreateInvite(ctx context.Context, boardID, tutorID string) (models.BoardInvite, error)
	DeleteInvite(ctx context.Context, boardID, tutorID string) error
	SaveAsset(ctx context.Context, boardID, tutorID, filePath, mimeType string, sizeBytes int) (models.BoardAsset, error)
	GetAsset(ctx context.Context, assetID string) (models.BoardAsset, error)
	GetPageSnapshot(ctx context.Context, pageID string) (json.RawMessage, error)
	// PageBelongsToTutor reports whether pageID's board is owned by tutorID.
	PageBelongsToTutor(ctx context.Context, pageID, tutorID string) (bool, error)
}

type whiteboardService struct {
	repo repository.WhiteboardRepository
}

func NewWhiteboardService(repo repository.WhiteboardRepository) WhiteboardService {
	return &whiteboardService{repo: repo}
}

func (s *whiteboardService) GetOrCreateBoard(ctx context.Context, courseID, tutorID string) (models.BoardWithPages, error) {
	ok, err := s.repo.CourseBelongsToTutor(ctx, courseID, tutorID)
	if err != nil {
		return models.BoardWithPages{}, err
	}
	if !ok {
		return models.BoardWithPages{}, ErrNotFound
	}
	board, err := s.repo.GetOrCreateBoard(ctx, courseID, tutorID)
	if err != nil {
		return models.BoardWithPages{}, err
	}
	// The upsert ignores tutor_id on conflict, so a pre-existing board could
	// belong to another tutor — verify ownership of the returned board.
	if board.TutorID != tutorID {
		return models.BoardWithPages{}, ErrNotFound
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

// GetOrCreateTrialBoard — общая доска пробных уроков препода (одна на препода,
// вне курсов). Проверки курса нет: курса у пробной доски не существует,
// владение проверяем по tutor_id вернувшейся доски.
func (s *whiteboardService) GetOrCreateTrialBoard(ctx context.Context, tutorID string) (models.BoardWithPages, error) {
	board, err := s.repo.GetOrCreateTrialBoard(ctx, tutorID)
	if err != nil {
		return models.BoardWithPages{}, err
	}
	// Upsert при конфликте не трогает tutor_id — на всякий случай убеждаемся,
	// что вернулась доска именно этого препода (та же проверка, что в GetOrCreateBoard).
	if board.TutorID != tutorID {
		return models.BoardWithPages{}, ErrNotFound
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

// verifyBoardOwnership loads the board and ensures it belongs to tutorID.
// Returns ErrNotFound on any mismatch or missing board (no cross-tenant leak).
func (s *whiteboardService) verifyBoardOwnership(ctx context.Context, boardID, tutorID string) error {
	board, err := s.repo.GetBoardByID(ctx, boardID)
	if err != nil || board.TutorID != tutorID {
		return ErrNotFound
	}
	return nil
}

func (s *whiteboardService) PageBelongsToTutor(ctx context.Context, pageID, tutorID string) (bool, error) {
	page, err := s.repo.GetPageByID(ctx, pageID)
	if err != nil {
		return false, nil
	}
	board, err := s.repo.GetBoardByID(ctx, page.BoardID)
	if err != nil {
		return false, nil
	}
	return board.TutorID == tutorID, nil
}

func (s *whiteboardService) CreatePage(ctx context.Context, boardID, tutorID, title string) (models.BoardPage, error) {
	if err := s.verifyBoardOwnership(ctx, boardID, tutorID); err != nil {
		return models.BoardPage{}, err
	}
	pages, err := s.repo.GetPagesByBoard(ctx, boardID)
	if err != nil {
		return models.BoardPage{}, err
	}
	return s.repo.CreatePage(ctx, boardID, title, len(pages))
}

func (s *whiteboardService) UpdatePage(ctx context.Context, pageID, tutorID string, req models.UpdateBoardPageRequest) (models.BoardPage, error) {
	page, err := s.repo.GetPageByID(ctx, pageID)
	if err != nil {
		return models.BoardPage{}, ErrNotFound
	}
	if err := s.verifyBoardOwnership(ctx, page.BoardID, tutorID); err != nil {
		return models.BoardPage{}, err
	}
	return s.repo.UpdatePage(ctx, pageID, req)
}

func (s *whiteboardService) DeletePage(ctx context.Context, pageID, tutorID string) error {
	page, err := s.repo.GetPageByID(ctx, pageID)
	if err != nil {
		return ErrNotFound
	}
	if err := s.verifyBoardOwnership(ctx, page.BoardID, tutorID); err != nil {
		return err
	}
	return s.repo.DeletePage(ctx, pageID, page.BoardID)
}

func (s *whiteboardService) SaveSnapshot(ctx context.Context, pageID string, snapshot json.RawMessage) error {
	return s.repo.SaveSnapshot(ctx, pageID, snapshot)
}

func (s *whiteboardService) CreateInvite(ctx context.Context, boardID, tutorID string) (models.BoardInvite, error) {
	if err := s.verifyBoardOwnership(ctx, boardID, tutorID); err != nil {
		return models.BoardInvite{}, err
	}
	return s.repo.CreateInvite(ctx, boardID)
}

func (s *whiteboardService) DeleteInvite(ctx context.Context, boardID, tutorID string) error {
	if err := s.verifyBoardOwnership(ctx, boardID, tutorID); err != nil {
		return err
	}
	return s.repo.DeleteInvite(ctx, boardID)
}

func (s *whiteboardService) SaveAsset(ctx context.Context, boardID, tutorID, filePath, mimeType string, sizeBytes int) (models.BoardAsset, error) {
	if err := s.verifyBoardOwnership(ctx, boardID, tutorID); err != nil {
		return models.BoardAsset{}, err
	}
	return s.repo.CreateAsset(ctx, boardID, filePath, mimeType, sizeBytes)
}

func (s *whiteboardService) GetAsset(ctx context.Context, assetID string) (models.BoardAsset, error) {
	return s.repo.GetAsset(ctx, assetID)
}

func (s *whiteboardService) GetPageSnapshot(ctx context.Context, pageID string) (json.RawMessage, error) {
	return s.repo.GetPageSnapshot(ctx, pageID)
}
