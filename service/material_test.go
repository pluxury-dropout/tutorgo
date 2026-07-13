package service_test

import (
	"context"
	"testing"

	"tutorgo/models"
	"tutorgo/service"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type mockMaterialRepo struct{ mock.Mock }

func (m *mockMaterialRepo) ListByParent(ctx context.Context, tutorID string, parentID *string) ([]models.Material, error) {
	args := m.Called(ctx, tutorID, parentID)
	return args.Get(0).([]models.Material), args.Error(1)
}
func (m *mockMaterialRepo) GetByID(ctx context.Context, id string) (models.Material, error) {
	args := m.Called(ctx, id)
	return args.Get(0).(models.Material), args.Error(1)
}
func (m *mockMaterialRepo) CreateFolder(ctx context.Context, tutorID, name string, parentID *string) (models.Material, error) {
	args := m.Called(ctx, tutorID, name, parentID)
	return args.Get(0).(models.Material), args.Error(1)
}
func (m *mockMaterialRepo) CreateFile(ctx context.Context, tutorID, name, filePath, mimeType string, sizeBytes int, parentID *string) (models.Material, error) {
	args := m.Called(ctx, tutorID, name, filePath, mimeType, sizeBytes, parentID)
	return args.Get(0).(models.Material), args.Error(1)
}
func (m *mockMaterialRepo) HasChildren(ctx context.Context, id string) (bool, error) {
	args := m.Called(ctx, id)
	return args.Bool(0), args.Error(1)
}
func (m *mockMaterialRepo) Delete(ctx context.Context, id, tutorID string) error {
	return m.Called(ctx, id, tutorID).Error(0)
}

// Папка другого препода как parent — отказ, чужое дерево недоступно.
func TestCreateFolder_ForeignParent(t *testing.T) {
	repo := new(mockMaterialRepo)
	repo.On("GetByID", mock.Anything, "p1").
		Return(models.Material{ID: "p1", TutorID: "other", Kind: "folder"}, nil)

	svc := service.NewMaterialService(repo)
	_, err := svc.CreateFolder(context.Background(), "me",
		models.CreateFolderRequest{Name: "Аудирование", ParentID: ptr("p1")})

	assert.ErrorIs(t, err, service.ErrNotFound)
	repo.AssertNotCalled(t, "CreateFolder", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

// Родителем может быть только папка, но не файл.
func TestCreateFolder_ParentIsFile(t *testing.T) {
	repo := new(mockMaterialRepo)
	repo.On("GetByID", mock.Anything, "f1").
		Return(models.Material{ID: "f1", TutorID: "me", Kind: "file"}, nil)

	svc := service.NewMaterialService(repo)
	_, err := svc.CreateFolder(context.Background(), "me",
		models.CreateFolderRequest{Name: "Аудирование", ParentID: ptr("f1")})

	assert.ErrorIs(t, err, service.ErrBadRequest)
	repo.AssertNotCalled(t, "CreateFolder", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

// Непустую папку удалять нельзя — тот же контракт, что у курса с уроками.
func TestDelete_NonEmptyFolder(t *testing.T) {
	repo := new(mockMaterialRepo)
	repo.On("GetByID", mock.Anything, "d1").
		Return(models.Material{ID: "d1", TutorID: "me", Kind: "folder"}, nil)
	repo.On("HasChildren", mock.Anything, "d1").Return(true, nil)

	svc := service.NewMaterialService(repo)
	_, err := svc.Delete(context.Background(), "d1", "me")

	assert.ErrorIs(t, err, service.ErrConflict)
	repo.AssertNotCalled(t, "Delete", mock.Anything, mock.Anything, mock.Anything)
}

// Удаление файла возвращает материал: хендлеру нужен FilePath, чтобы снести объект в S3.
func TestDelete_FileReturnsMaterial(t *testing.T) {
	repo := new(mockMaterialRepo)
	repo.On("GetByID", mock.Anything, "f1").Return(
		models.Material{ID: "f1", TutorID: "me", Kind: "file", FilePath: "materials/f1.mp3"}, nil)
	repo.On("Delete", mock.Anything, "f1", "me").Return(nil)

	svc := service.NewMaterialService(repo)
	got, err := svc.Delete(context.Background(), "f1", "me")

	assert.NoError(t, err)
	assert.Equal(t, "materials/f1.mp3", got.FilePath)
	repo.AssertExpectations(t)
}
