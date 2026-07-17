package service_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"tutorgo/models"
	"tutorgo/repository"
	"tutorgo/service"
)

type mockPdfImportRepo struct{ mock.Mock }

func (m *mockPdfImportRepo) Create(ctx context.Context, boardID, pageID, tutorID, s3Key string, pages []models.PdfImportPage) (string, error) {
	args := m.Called(ctx, boardID, pageID, tutorID, s3Key, pages)
	return args.String(0), args.Error(1)
}

func (m *mockPdfImportRepo) GetByID(ctx context.Context, id string) (models.PdfImport, error) {
	args := m.Called(ctx, id)
	return args.Get(0).(models.PdfImport), args.Error(1)
}

func (m *mockPdfImportRepo) Start(ctx context.Context, id string, from, to int, enqueue func(pgx.Tx) error) ([]models.PdfImportPage, error) {
	args := m.Called(ctx, id, from, to, enqueue)
	if err := enqueue(nil); err != nil { // убеждаемся, что enqueue-цепочка живая
		return nil, err
	}
	return args.Get(0).([]models.PdfImportPage), args.Error(1)
}

func (m *mockPdfImportRepo) MarkPageDone(ctx context.Context, id string, n int) error {
	return m.Called(ctx, id, n).Error(0)
}

func (m *mockPdfImportRepo) SetStatus(ctx context.Context, id, status string, errMsg *string) error {
	return m.Called(ctx, id, status, errMsg).Error(0)
}

func (m *mockPdfImportRepo) DeleteStalePending(ctx context.Context) ([]string, error) {
	args := m.Called(ctx)
	return args.Get(0).([]string), args.Error(1)
}

func (m *mockPdfImportRepo) BoardBelongsToTutor(ctx context.Context, boardID, tutorID string) (bool, error) {
	args := m.Called(ctx, boardID, tutorID)
	return args.Bool(0), args.Error(1)
}

func (m *mockPdfImportRepo) PageBelongsToBoard(ctx context.Context, pageID, boardID string) (bool, error) {
	args := m.Called(ctx, pageID, boardID)
	return args.Bool(0), args.Error(1)
}

func noEnqueue(ctx context.Context, tx pgx.Tx, importID string) error { return nil }

func TestCreateImportChecksOwnership(t *testing.T) {
	repo := new(mockPdfImportRepo)
	svc := service.NewPdfImportService(repo, noEnqueue)
	repo.On("BoardBelongsToTutor", mock.Anything, "b1", "t1").Return(false, nil)

	_, err := svc.CreateImport(context.Background(), "b1", "p1", "t1", "key", []models.PageSizePt{{W: 612, H: 792}})
	assert.ErrorIs(t, err, service.ErrForbidden)
}

func TestCreateImportHappyPath(t *testing.T) {
	repo := new(mockPdfImportRepo)
	svc := service.NewPdfImportService(repo, noEnqueue)
	repo.On("BoardBelongsToTutor", mock.Anything, "b1", "t1").Return(true, nil)
	repo.On("PageBelongsToBoard", mock.Anything, "p1", "b1").Return(true, nil)
	repo.On("Create", mock.Anything, "b1", "p1", "t1", "key",
		[]models.PdfImportPage{{N: 1, W: 612, H: 792}, {N: 2, W: 595, H: 842}},
	).Return("imp1", nil)

	resp, err := svc.CreateImport(context.Background(), "b1", "p1", "t1", "key",
		[]models.PageSizePt{{W: 612, H: 792}, {W: 595, H: 842}})
	assert.NoError(t, err)
	assert.Equal(t, "imp1", resp.ImportID)
	assert.Equal(t, 2, resp.NumPages)
	assert.Len(t, resp.PageSizes, 2)
}

func TestStartChecksOwnership(t *testing.T) {
	repo := new(mockPdfImportRepo)
	svc := service.NewPdfImportService(repo, noEnqueue)
	repo.On("GetByID", mock.Anything, "imp1").Return(models.PdfImport{ID: "imp1", TutorID: "other"}, nil)

	_, err := svc.Start(context.Background(), "imp1", "t1", 1, 2)
	assert.ErrorIs(t, err, service.ErrForbidden)
}

func TestStartOnMissingImportReturnsNotFound(t *testing.T) {
	repo := new(mockPdfImportRepo)
	svc := service.NewPdfImportService(repo, noEnqueue)
	repo.On("GetByID", mock.Anything, "missing").Return(models.PdfImport{}, pgx.ErrNoRows)

	_, err := svc.Start(context.Background(), "missing", "t1", 1, 2)
	assert.ErrorIs(t, err, service.ErrNotFound)
}

func TestStartAlreadyStartedReturnsBadRequest(t *testing.T) {
	repo := new(mockPdfImportRepo)
	svc := service.NewPdfImportService(repo, noEnqueue)
	repo.On("GetByID", mock.Anything, "imp1").Return(models.PdfImport{ID: "imp1", TutorID: "t1"}, nil)
	repo.On("Start", mock.Anything, "imp1", 1, 2, mock.Anything).Return(
		[]models.PdfImportPage(nil), fmt.Errorf("import imp1 already started (status rendering): %w", repository.ErrImportAlreadyStarted))

	_, err := svc.Start(context.Background(), "imp1", "t1", 1, 2)
	assert.ErrorIs(t, err, service.ErrBadRequest)
}

func TestStartBuildsRelativeURLs(t *testing.T) {
	repo := new(mockPdfImportRepo)
	svc := service.NewPdfImportService(repo, noEnqueue)
	repo.On("GetByID", mock.Anything, "imp1").Return(models.PdfImport{ID: "imp1", TutorID: "t1"}, nil)
	repo.On("Start", mock.Anything, "imp1", 2, 3, mock.Anything).Return(
		[]models.PdfImportPage{
			{N: 2, AssetID: "a2", W: 612, H: 792},
			{N: 3, AssetID: "a3", W: 595, H: 842},
		}, nil)

	resp, err := svc.Start(context.Background(), "imp1", "t1", 2, 3)
	assert.NoError(t, err)
	assert.Len(t, resp.Pages, 2)
	assert.Equal(t, "a2", resp.Pages[0].FileID)
	assert.Equal(t, "/public/board-assets/a2", resp.Pages[0].URL)
	assert.Equal(t, 612.0, resp.Pages[0].W)
}
