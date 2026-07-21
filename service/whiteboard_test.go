package service_test

import (
	"context"
	"encoding/json"
	"testing"
	"tutorgo/models"
	"tutorgo/service"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type mockWhiteboardRepo struct{ mock.Mock }

func (m *mockWhiteboardRepo) CourseBelongsToTutor(ctx context.Context, courseID, tutorID string) (bool, error) {
	args := m.Called(ctx, courseID, tutorID)
	return args.Bool(0), args.Error(1)
}
func (m *mockWhiteboardRepo) GetBoardByID(ctx context.Context, boardID string) (models.Board, error) {
	args := m.Called(ctx, boardID)
	return args.Get(0).(models.Board), args.Error(1)
}
func (m *mockWhiteboardRepo) GetOrCreateBoard(ctx context.Context, courseID, tutorID string) (models.Board, error) {
	args := m.Called(ctx, courseID, tutorID)
	return args.Get(0).(models.Board), args.Error(1)
}
func (m *mockWhiteboardRepo) GetOrCreateTrialBoard(ctx context.Context, tutorID string) (models.Board, error) {
	args := m.Called(ctx, tutorID)
	return args.Get(0).(models.Board), args.Error(1)
}
func (m *mockWhiteboardRepo) GetBoardByInvite(ctx context.Context, inviteID string) (models.Board, error) {
	args := m.Called(ctx, inviteID)
	return args.Get(0).(models.Board), args.Error(1)
}
func (m *mockWhiteboardRepo) GetPagesByBoard(ctx context.Context, boardID string) ([]models.BoardPage, error) {
	args := m.Called(ctx, boardID)
	return args.Get(0).([]models.BoardPage), args.Error(1)
}
func (m *mockWhiteboardRepo) GetPageByID(ctx context.Context, pageID string) (models.BoardPage, error) {
	args := m.Called(ctx, pageID)
	return args.Get(0).(models.BoardPage), args.Error(1)
}
func (m *mockWhiteboardRepo) CreatePage(ctx context.Context, boardID, title string, position int) (models.BoardPage, error) {
	args := m.Called(ctx, boardID, title, position)
	return args.Get(0).(models.BoardPage), args.Error(1)
}
func (m *mockWhiteboardRepo) UpdatePage(ctx context.Context, pageID string, req models.UpdateBoardPageRequest) (models.BoardPage, error) {
	args := m.Called(ctx, pageID, req)
	return args.Get(0).(models.BoardPage), args.Error(1)
}
func (m *mockWhiteboardRepo) SaveSnapshot(ctx context.Context, pageID string, snapshot json.RawMessage) error {
	return m.Called(ctx, pageID, snapshot).Error(0)
}
func (m *mockWhiteboardRepo) DeletePage(ctx context.Context, pageID, boardID string) error {
	return m.Called(ctx, pageID, boardID).Error(0)
}
func (m *mockWhiteboardRepo) CreateInvite(ctx context.Context, boardID string) (models.BoardInvite, error) {
	args := m.Called(ctx, boardID)
	return args.Get(0).(models.BoardInvite), args.Error(1)
}
func (m *mockWhiteboardRepo) DeleteInvite(ctx context.Context, boardID string) error {
	return m.Called(ctx, boardID).Error(0)
}
func (m *mockWhiteboardRepo) GetInviteByBoard(ctx context.Context, boardID string) (models.BoardInvite, error) {
	args := m.Called(ctx, boardID)
	return args.Get(0).(models.BoardInvite), args.Error(1)
}
func (m *mockWhiteboardRepo) CreateAsset(ctx context.Context, boardID, filePath, mimeType string, sizeBytes int) (models.BoardAsset, error) {
	args := m.Called(ctx, boardID, filePath, mimeType, sizeBytes)
	return args.Get(0).(models.BoardAsset), args.Error(1)
}
func (m *mockWhiteboardRepo) GetAsset(ctx context.Context, assetID string) (models.BoardAsset, error) {
	args := m.Called(ctx, assetID)
	return args.Get(0).(models.BoardAsset), args.Error(1)
}
func (m *mockWhiteboardRepo) MergeElements(ctx context.Context, pageID string, els []models.BoardElement) error {
	return m.Called(ctx, pageID, els).Error(0)
}
func (m *mockWhiteboardRepo) MergeFiles(ctx context.Context, pageID string, files json.RawMessage) error {
	return m.Called(ctx, pageID, files).Error(0)
}
func (m *mockWhiteboardRepo) GetPageState(ctx context.Context, pageID string) (json.RawMessage, bool, error) {
	args := m.Called(ctx, pageID)
	if args.Get(0) == nil {
		return nil, args.Bool(1), args.Error(2)
	}
	return args.Get(0).(json.RawMessage), args.Bool(1), args.Error(2)
}
func (m *mockWhiteboardRepo) ImportSnapshotToElements(ctx context.Context, pageID string) error {
	return m.Called(ctx, pageID).Error(0)
}
func (m *mockWhiteboardRepo) DeleteOldTombstones(ctx context.Context) (int64, error) {
	args := m.Called(ctx)
	return int64(args.Int(0)), args.Error(1)
}

func TestWhiteboardService_GetOrCreateBoard_CreatesFirstPage(t *testing.T) {
	repo := new(mockWhiteboardRepo)
	svc := service.NewWhiteboardService(repo)

	board := models.Board{ID: "board-1", CourseID: "course-1", TutorID: "tutor-1"}
	repo.On("CourseBelongsToTutor", mock.Anything, "course-1", "tutor-1").Return(true, nil)
	repo.On("GetOrCreateBoard", mock.Anything, "course-1", "tutor-1").Return(board, nil)
	// No pages → service creates first one
	repo.On("GetPagesByBoard", mock.Anything, "board-1").Return([]models.BoardPage{}, nil)
	firstPage := models.BoardPage{ID: "page-1", BoardID: "board-1", Title: "Страница 1", Position: 0}
	repo.On("CreatePage", mock.Anything, "board-1", "Страница 1", 0).Return(firstPage, nil)

	result, err := svc.GetOrCreateBoard(context.Background(), "course-1", "tutor-1")
	assert.NoError(t, err)
	assert.Equal(t, "board-1", result.ID)
	assert.Len(t, result.Pages, 1)
	assert.Equal(t, "Страница 1", result.Pages[0].Title)
	repo.AssertExpectations(t)
}

func TestWhiteboardService_GetOrCreateBoard_ExistingPages(t *testing.T) {
	repo := new(mockWhiteboardRepo)
	svc := service.NewWhiteboardService(repo)

	board := models.Board{ID: "board-1", CourseID: "course-1", TutorID: "tutor-1"}
	pages := []models.BoardPage{{ID: "page-1", BoardID: "board-1", Title: "Урок 1"}}
	repo.On("CourseBelongsToTutor", mock.Anything, "course-1", "tutor-1").Return(true, nil)
	repo.On("GetOrCreateBoard", mock.Anything, "course-1", "tutor-1").Return(board, nil)
	repo.On("GetPagesByBoard", mock.Anything, "board-1").Return(pages, nil)

	result, err := svc.GetOrCreateBoard(context.Background(), "course-1", "tutor-1")
	assert.NoError(t, err)
	assert.Len(t, result.Pages, 1)
	// CreatePage should not be called if pages already exist
	repo.AssertNotCalled(t, "CreatePage")
}

// (b) GetOrCreateBoard rejects a course not owned by the tutor.
func TestWhiteboardService_GetOrCreateBoard_ForeignCourse(t *testing.T) {
	repo := new(mockWhiteboardRepo)
	svc := service.NewWhiteboardService(repo)

	repo.On("CourseBelongsToTutor", mock.Anything, "course-x", "tutor-1").Return(false, nil)

	_, err := svc.GetOrCreateBoard(context.Background(), "course-x", "tutor-1")
	assert.ErrorIs(t, err, service.ErrNotFound)
	// Must not upsert a board for a course the tutor does not own.
	repo.AssertNotCalled(t, "GetOrCreateBoard")
}

// GetOrCreateBoard rejects a pre-existing board owned by another tutor
// (the upsert ignores tutor_id on conflict).
func TestWhiteboardService_GetOrCreateBoard_PreexistingForeignBoard(t *testing.T) {
	repo := new(mockWhiteboardRepo)
	svc := service.NewWhiteboardService(repo)

	// Course check passes, but the returned board belongs to another tutor.
	foreign := models.Board{ID: "board-1", CourseID: "course-1", TutorID: "tutor-2"}
	repo.On("CourseBelongsToTutor", mock.Anything, "course-1", "tutor-1").Return(true, nil)
	repo.On("GetOrCreateBoard", mock.Anything, "course-1", "tutor-1").Return(foreign, nil)

	_, err := svc.GetOrCreateBoard(context.Background(), "course-1", "tutor-1")
	assert.ErrorIs(t, err, service.ErrNotFound)
}

// Пробная доска препода отдаётся вместе с первой страницей.
func TestGetOrCreateTrialBoard_CreatesFirstPage(t *testing.T) {
	repo := new(mockWhiteboardRepo)
	board := models.Board{ID: "b1", CourseID: "", TutorID: "me"}
	repo.On("GetOrCreateTrialBoard", mock.Anything, "me").Return(board, nil)
	repo.On("GetPagesByBoard", mock.Anything, "b1").Return([]models.BoardPage{}, nil)
	repo.On("CreatePage", mock.Anything, "b1", "Страница 1", 0).
		Return(models.BoardPage{ID: "p1", BoardID: "b1", Title: "Страница 1"}, nil)

	svc := service.NewWhiteboardService(repo)
	got, err := svc.GetOrCreateTrialBoard(context.Background(), "me")

	assert.NoError(t, err)
	assert.Equal(t, "b1", got.Board.ID)
	assert.Len(t, got.Pages, 1)
}

// Доска, вернувшаяся с чужим tutor_id, наружу не уходит.
func TestGetOrCreateTrialBoard_ForeignBoard(t *testing.T) {
	repo := new(mockWhiteboardRepo)
	repo.On("GetOrCreateTrialBoard", mock.Anything, "me").
		Return(models.Board{ID: "b1", TutorID: "other"}, nil)

	svc := service.NewWhiteboardService(repo)
	_, err := svc.GetOrCreateTrialBoard(context.Background(), "me")

	assert.ErrorIs(t, err, service.ErrNotFound)
	repo.AssertNotCalled(t, "GetPagesByBoard", mock.Anything, mock.Anything)
}

// (a) UpdatePage rejects a page whose board belongs to another tutor.
func TestWhiteboardService_UpdatePage_ForeignBoard(t *testing.T) {
	repo := new(mockWhiteboardRepo)
	svc := service.NewWhiteboardService(repo)

	page := models.BoardPage{ID: "page-1", BoardID: "board-1"}
	foreignBoard := models.Board{ID: "board-1", TutorID: "tutor-2"}
	repo.On("GetPageByID", mock.Anything, "page-1").Return(page, nil)
	repo.On("GetBoardByID", mock.Anything, "board-1").Return(foreignBoard, nil)

	title := "Hacked"
	_, err := svc.UpdatePage(context.Background(), "page-1", "tutor-1", models.UpdateBoardPageRequest{Title: &title})
	assert.ErrorIs(t, err, service.ErrNotFound)
	repo.AssertNotCalled(t, "UpdatePage")
}

// UpdatePage succeeds when the page's board belongs to the tutor.
func TestWhiteboardService_UpdatePage_OwnBoard(t *testing.T) {
	repo := new(mockWhiteboardRepo)
	svc := service.NewWhiteboardService(repo)

	page := models.BoardPage{ID: "page-1", BoardID: "board-1"}
	board := models.Board{ID: "board-1", TutorID: "tutor-1"}
	title := "Renamed"
	updated := models.BoardPage{ID: "page-1", BoardID: "board-1", Title: title}
	repo.On("GetPageByID", mock.Anything, "page-1").Return(page, nil)
	repo.On("GetBoardByID", mock.Anything, "board-1").Return(board, nil)
	repo.On("UpdatePage", mock.Anything, "page-1", models.UpdateBoardPageRequest{Title: &title}).Return(updated, nil)

	result, err := svc.UpdatePage(context.Background(), "page-1", "tutor-1", models.UpdateBoardPageRequest{Title: &title})
	assert.NoError(t, err)
	assert.Equal(t, "Renamed", result.Title)
	repo.AssertExpectations(t)
}

// (c) CreatePage rejects a foreign board.
func TestWhiteboardService_CreatePage_ForeignBoard(t *testing.T) {
	repo := new(mockWhiteboardRepo)
	svc := service.NewWhiteboardService(repo)

	foreignBoard := models.Board{ID: "board-1", TutorID: "tutor-2"}
	repo.On("GetBoardByID", mock.Anything, "board-1").Return(foreignBoard, nil)

	_, err := svc.CreatePage(context.Background(), "board-1", "tutor-1", "New page")
	assert.ErrorIs(t, err, service.ErrNotFound)
	repo.AssertNotCalled(t, "CreatePage")
}

// CreateInvite rejects a foreign board.
func TestWhiteboardService_CreateInvite_ForeignBoard(t *testing.T) {
	repo := new(mockWhiteboardRepo)
	svc := service.NewWhiteboardService(repo)

	foreignBoard := models.Board{ID: "board-1", TutorID: "tutor-2"}
	repo.On("GetBoardByID", mock.Anything, "board-1").Return(foreignBoard, nil)

	_, err := svc.CreateInvite(context.Background(), "board-1", "tutor-1")
	assert.ErrorIs(t, err, service.ErrNotFound)
	repo.AssertNotCalled(t, "CreateInvite")
}
