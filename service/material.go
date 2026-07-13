package service

import (
	"context"
	"errors"

	"tutorgo/models"
	"tutorgo/repository"

	"github.com/jackc/pgx/v5"
)

type MaterialService interface {
	List(ctx context.Context, tutorID string, parentID *string) ([]models.Material, error)
	CreateFolder(ctx context.Context, tutorID string, req models.CreateFolderRequest) (models.Material, error)
	CreateFile(ctx context.Context, tutorID, name, filePath, mimeType string, sizeBytes int, parentID *string) (models.Material, error)
	// Delete возвращает удалённый материал: хендлеру нужен FilePath, чтобы снести
	// объект в S3 после того, как строка исчезла из БД.
	Delete(ctx context.Context, id, tutorID string) (models.Material, error)
	GetFile(ctx context.Context, id, tutorID string) (models.Material, error)
}

type materialService struct {
	repo repository.MaterialRepository
}

func NewMaterialService(repo repository.MaterialRepository) MaterialService {
	return &materialService{repo: repo}
}

// requireOwnFolder проверяет, что parentID (если задан) — существующая папка
// этого препода. ErrNotFound для чужого/несуществующего (чужое дерево не
// подсвечиваем), ErrBadRequest — если это файл, а не папка.
func (s *materialService) requireOwnFolder(ctx context.Context, tutorID string, parentID *string) error {
	if parentID == nil {
		return nil
	}
	parent, err := s.repo.GetByID(ctx, *parentID)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	if parent.TutorID != tutorID {
		return ErrNotFound
	}
	if parent.Kind != "folder" {
		return ErrBadRequest
	}
	return nil
}

func (s *materialService) List(ctx context.Context, tutorID string, parentID *string) ([]models.Material, error) {
	if err := s.requireOwnFolder(ctx, tutorID, parentID); err != nil {
		return nil, err
	}
	return s.repo.ListByParent(ctx, tutorID, parentID)
}

func (s *materialService) CreateFolder(ctx context.Context, tutorID string, req models.CreateFolderRequest) (models.Material, error) {
	if err := s.requireOwnFolder(ctx, tutorID, req.ParentID); err != nil {
		return models.Material{}, err
	}
	return s.repo.CreateFolder(ctx, tutorID, req.Name, req.ParentID)
}

func (s *materialService) CreateFile(ctx context.Context, tutorID, name, filePath, mimeType string, sizeBytes int, parentID *string) (models.Material, error) {
	if err := s.requireOwnFolder(ctx, tutorID, parentID); err != nil {
		return models.Material{}, err
	}
	return s.repo.CreateFile(ctx, tutorID, name, filePath, mimeType, sizeBytes, parentID)
}

func (s *materialService) Delete(ctx context.Context, id, tutorID string) (models.Material, error) {
	m, err := s.repo.GetByID(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return models.Material{}, ErrNotFound
	}
	if err != nil {
		return models.Material{}, err
	}
	if m.TutorID != tutorID {
		return models.Material{}, ErrNotFound
	}
	if m.Kind == "folder" {
		// ponytail: рекурсивного удаления нет — оно требует обхода дерева и пакетной
		// чистки S3. Непустую папку просто не даём удалить, как курс с уроками.
		// Если начнёт мешать — рекурсия по parent_id + батч Remove.
		has, err := s.repo.HasChildren(ctx, id)
		if err != nil {
			return models.Material{}, err
		}
		if has {
			return models.Material{}, ErrConflict
		}
	}
	if err := s.repo.Delete(ctx, id, tutorID); err != nil {
		return models.Material{}, err
	}
	return m, nil
}

func (s *materialService) GetFile(ctx context.Context, id, tutorID string) (models.Material, error) {
	m, err := s.repo.GetByID(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return models.Material{}, ErrNotFound
	}
	if err != nil {
		return models.Material{}, err
	}
	if m.TutorID != tutorID || m.Kind != "file" {
		return models.Material{}, ErrNotFound
	}
	return m, nil
}
