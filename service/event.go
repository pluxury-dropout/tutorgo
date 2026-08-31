package service

import (
	"context"
	"errors"
	"fmt"
	"tutorgo/models"
	"tutorgo/repository"

	"github.com/jackc/pgx/v5"
)

// Событие на весь день длительности не имеет; сутки нужны только чтобы
// FullCalendar и лента считали его одним блоком. В занятость all_day не
// попадает — иначе «День рождения мамы» ругался бы на каждый урок этого дня.
const allDayMinutes = 24 * 60

type EventService interface {
	Create(ctx context.Context, tutorID string, req models.CreateEventRequest) (models.Event, error)
	GetByID(ctx context.Context, id, tutorID string) (models.Event, error)
	GetByRange(ctx context.Context, tutorID, from, to string) ([]models.Event, error)
	Update(ctx context.Context, id, tutorID string, req models.UpdateEventRequest) (models.Event, error)
	Delete(ctx context.Context, id, tutorID string) error
}

type eventService struct {
	repo repository.EventRepository
}

func NewEventService(repo repository.EventRepository) EventService {
	return &eventService{repo: repo}
}

// applyDefaults приводит запрос к виду, который принимает БД: kind по умолчанию
// личный, длительность у all_day — сутки, у обычного события обязательна.
func applyDefaults(req *models.CreateEventRequest) error {
	if req.Kind == "" {
		req.Kind = "personal"
	}
	if req.AllDay {
		req.DurationMinutes = allDayMinutes
		return nil
	}
	if req.DurationMinutes <= 0 {
		return fmt.Errorf("duration_minutes is required: %w", ErrBadRequest)
	}
	return nil
}

func (s *eventService) Create(ctx context.Context, tutorID string, req models.CreateEventRequest) (models.Event, error) {
	if err := applyDefaults(&req); err != nil {
		return models.Event{}, err
	}
	return s.repo.Create(ctx, tutorID, req)
}

func (s *eventService) GetByID(ctx context.Context, id, tutorID string) (models.Event, error) {
	e, err := s.repo.GetByID(ctx, id, tutorID)
	if errors.Is(err, pgx.ErrNoRows) {
		return models.Event{}, fmt.Errorf("event: %w", ErrNotFound)
	}
	return e, err
}

func (s *eventService) GetByRange(ctx context.Context, tutorID, from, to string) ([]models.Event, error) {
	return s.repo.GetByRange(ctx, tutorID, from, to)
}

func (s *eventService) Update(ctx context.Context, id, tutorID string, req models.UpdateEventRequest) (models.Event, error) {
	if err := applyDefaults(&req); err != nil {
		return models.Event{}, err
	}
	e, err := s.repo.Update(ctx, id, tutorID, req)
	if errors.Is(err, pgx.ErrNoRows) {
		return models.Event{}, fmt.Errorf("event: %w", ErrNotFound)
	}
	return e, err
}

func (s *eventService) Delete(ctx context.Context, id, tutorID string) error {
	if err := s.repo.Delete(ctx, id, tutorID); err != nil {
		if errors.Is(err, repository.ErrEventNotFound) {
			return fmt.Errorf("event: %w", ErrNotFound)
		}
		return err
	}
	return nil
}
