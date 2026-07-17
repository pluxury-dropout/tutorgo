package worker

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"tutorgo/jobs"
	"tutorgo/models"
)

// мок репозитория — те же сигнатуры, что в service/pdf_import_test.go
type mockRepo struct{ mock.Mock }

func (m *mockRepo) GetByID(ctx context.Context, id string) (models.PdfImport, error) {
	args := m.Called(ctx, id)
	return args.Get(0).(models.PdfImport), args.Error(1)
}
func (m *mockRepo) MarkPageDone(ctx context.Context, id string, n int) error {
	return m.Called(ctx, id, n).Error(0)
}
func (m *mockRepo) SetStatus(ctx context.Context, id, status string, errMsg *string) error {
	return m.Called(ctx, id, status, errMsg).Error(0)
}

func testImport() models.PdfImport {
	return models.PdfImport{
		ID: "imp1", PageID: "page1", S3Key: "board-pdf/x.pdf", Status: "rendering",
		Pages: []models.PdfImportPage{
			{N: 1, AssetID: "a1", W: 612, H: 792, Done: true}, // уже готова: повтор джобы
			{N: 2, AssetID: "a2", W: 612, H: 792},
		},
	}
}

func newTestWorker(repo *mockRepo, events *[]models.BoardEvent, renderErr error) *PdfImportWorker {
	return &PdfImportWorker{
		Repo: repo,
		Fetch: func(ctx context.Context, key string) (string, error) { return "/tmp/fake.pdf", nil },
		Render: func(ctx context.Context, pdfPath string, n int) (string, error) {
			if renderErr != nil {
				return "", renderErr
			}
			return "/tmp/fake.jpg", nil
		},
		Upload: func(ctx context.Context, key, jpegPath string) error { return nil },
		Notify: func(ctx context.Context, ev models.BoardEvent) error {
			*events = append(*events, ev)
			return nil
		},
	}
}

func job(attempt, maxAttempts int) *river.Job[jobs.PdfImportArgs] {
	return &river.Job[jobs.PdfImportArgs]{
		JobRow: &rivertype.JobRow{Attempt: attempt, MaxAttempts: maxAttempts},
		Args:   jobs.PdfImportArgs{ImportID: "imp1"},
	}
}

func TestWorkSkipsDonePagesAndNotifies(t *testing.T) {
	repo := new(mockRepo)
	var events []models.BoardEvent
	w := newTestWorker(repo, &events, nil)
	repo.On("GetByID", mock.Anything, "imp1").Return(testImport(), nil)
	repo.On("MarkPageDone", mock.Anything, "imp1", 2).Return(nil)
	repo.On("SetStatus", mock.Anything, "imp1", "done", (*string)(nil)).Return(nil)

	err := w.Work(context.Background(), job(1, 5))
	assert.NoError(t, err)
	// страница 1 done — не трогали; страница 2 отрендерена и объявлена
	repo.AssertNotCalled(t, "MarkPageDone", mock.Anything, "imp1", 1)
	assert.Len(t, events, 1)
	assert.Equal(t, "page1", events[0].PageID)
	assert.True(t, strings.Contains(string(events[0].Msg), `"a2"`))
	assert.True(t, strings.Contains(string(events[0].Msg), `"file"`))
}

func TestWorkFinalAttemptMarksFailed(t *testing.T) {
	repo := new(mockRepo)
	var events []models.BoardEvent
	w := newTestWorker(repo, &events, errors.New("битый pdf"))
	repo.On("GetByID", mock.Anything, "imp1").Return(testImport(), nil)
	repo.On("SetStatus", mock.Anything, "imp1", "failed", mock.Anything).Return(nil)

	err := w.Work(context.Background(), job(5, 5)) // последняя попытка
	assert.Error(t, err)
	repo.AssertCalled(t, "SetStatus", mock.Anything, "imp1", "failed", mock.Anything)
	// import_failed с fileId нерендерённых страниц
	assert.Len(t, events, 1)
	var ev struct {
		Type    string `json:"type"`
		Payload struct {
			FileIDs []string `json:"fileIds"`
		} `json:"payload"`
	}
	assert.NoError(t, json.Unmarshal(events[0].Msg, &ev))
	assert.Equal(t, "import_failed", ev.Type)
	assert.Equal(t, []string{"a2"}, ev.Payload.FileIDs)
}

func TestWorkTransientErrorRetries(t *testing.T) {
	repo := new(mockRepo)
	var events []models.BoardEvent
	w := newTestWorker(repo, &events, errors.New("s3 моргнул"))
	repo.On("GetByID", mock.Anything, "imp1").Return(testImport(), nil)

	err := w.Work(context.Background(), job(2, 5)) // не последняя попытка
	assert.Error(t, err)
	repo.AssertNotCalled(t, "SetStatus", mock.Anything, "imp1", "failed", mock.Anything)
	assert.Empty(t, events)
}
