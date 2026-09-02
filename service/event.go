package service

import (
	"context"
	"errors"
	"fmt"
	"time"
	"tutorgo/models"
	"tutorgo/repository"

	"github.com/jackc/pgx/v5"
)

type EventService interface {
	Create(ctx context.Context, tutorID string, req models.CreateEventRequest) (models.Event, error)
	GetByID(ctx context.Context, id, tutorID string) (models.Event, error)
	GetByRange(ctx context.Context, tutorID, from, to string) ([]models.Event, error)
	Update(ctx context.Context, id, tutorID string, req models.UpdateEventRequest) (models.Event, error)
	Delete(ctx context.Context, id, tutorID string) error
}

type eventService struct {
	repo       repository.EventRepository
	recurrence RecurrenceService
}

func NewEventService(repo repository.EventRepository, recurrence RecurrenceService) EventService {
	return &eventService{repo: repo, recurrence: recurrence}
}

func (s *eventService) Create(ctx context.Context, tutorID string, req models.CreateEventRequest) (models.Event, error) {
	if req.Kind == "" {
		req.Kind = "personal"
	}
	if req.Recurrence == nil {
		return s.repo.Create(ctx, tutorID, req)
	}

	// Первое событие — оно же шаблон серии: остальные вхождения материализация
	// копирует с него (см. repository.InsertOccurrences).
	rule, err := s.recurrence.CreateRule(ctx, *req.Recurrence, req.StartsAt, req.DurationMinutes, tutorID)
	if err != nil {
		return models.Event{}, err
	}
	req.RuleID = rule.ID
	req.OccurrenceDate = &rule.StartsOn

	event, err := s.repo.Create(ctx, tutorID, req)
	if err != nil {
		if delErr := s.recurrence.DeleteRule(ctx, rule.ID); delErr != nil {
			return models.Event{}, fmt.Errorf("create event: %w (orphan rule %s)", err, rule.ID)
		}
		return models.Event{}, err
	}

	// Материализация не критична: не вышло сейчас — ночная джоба догонит.
	_, _ = s.recurrence.Materialize(ctx, rule.ID, time.Now().Add(RecurrenceHorizon))
	return event, nil
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
	if req.Kind == "" {
		req.Kind = "personal"
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
