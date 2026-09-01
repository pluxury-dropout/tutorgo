package service

import (
	"context"
	"sort"
	"strings"
	"time"
	"tutorgo/models"

	"golang.org/x/sync/errgroup"
)

// Задача без указанной длительности рисуется получасовым блоком —
// в модели поле nullable, а сетке нужна высота.
const defaultTaskMinutes = 30

type CalendarService interface {
	// GetFeed отдаёт уроки, события и задачи одного репетитора в диапазоне,
	// отсортированные по времени начала. kinds — csv-фильтр («lesson,event»),
	// пустая строка означает все три источника.
	GetFeed(ctx context.Context, tutorID, from, to, kinds string) ([]models.CalendarItem, error)
	// GetConflicts возвращает всё, что занимает время в предполагаемом слоте.
	GetConflicts(ctx context.Context, tutorID string, startsAt time.Time, durationMinutes int, excludeType string, excludeID *string) ([]models.CalendarItem, error)
}

// Ленте нужны только диапазонные запросы, поэтому зависимости объявлены узкими
// интерфейсами, а не целыми LessonService/EventRepository: и связность меньше,
// и мок в тесте — на один метод вместо двадцати. Конкретные типы им
// удовлетворяют структурно, в router ничего оборачивать не надо.
type lessonFeedSource interface {
	GetCalendar(ctx context.Context, tutorID, from, to string) ([]models.CalendarLesson, error)
}

type eventFeedSource interface {
	GetByRange(ctx context.Context, tutorID, from, to string) ([]models.Event, error)
	GetOccupiedInRange(ctx context.Context, tutorID, from, to, excludeType string, excludeID *string) ([]models.CalendarItem, error)
}

type taskFeedSource interface {
	GetByRange(ctx context.Context, tutorID, from, to string) ([]models.Task, error)
}

type calendarService struct {
	lessons lessonFeedSource
	events  eventFeedSource
	tasks   taskFeedSource
}

func NewCalendarService(lessons lessonFeedSource, events eventFeedSource, tasks taskFeedSource) CalendarService {
	return &calendarService{lessons: lessons, events: events, tasks: tasks}
}

// wantKinds разбирает csv-фильтр; пустая строка — все типы.
func wantKinds(kinds string) map[string]bool {
	if kinds == "" {
		return map[string]bool{"lesson": true, "event": true, "task": true}
	}
	want := make(map[string]bool, 3)
	for k := range strings.SplitSeq(kinds, ",") {
		want[strings.TrimSpace(k)] = true
	}
	return want
}

func (s *calendarService) GetFeed(ctx context.Context, tutorID, from, to, kinds string) ([]models.CalendarItem, error) {
	want := wantKinds(kinds)

	var (
		lessons []models.CalendarLesson
		events  []models.Event
		tasks   []models.Task
	)
	// Три независимых источника — параллельно: это самый частый запрос в приложении.
	g, gctx := errgroup.WithContext(ctx)
	if want["lesson"] {
		g.Go(func() error {
			var err error
			lessons, err = s.lessons.GetCalendar(gctx, tutorID, from, to)
			return err
		})
	}
	if want["event"] {
		g.Go(func() error {
			var err error
			events, err = s.events.GetByRange(gctx, tutorID, from, to)
			return err
		})
	}
	if want["task"] {
		g.Go(func() error {
			var err error
			tasks, err = s.tasks.GetByRange(gctx, tutorID, from, to)
			return err
		})
	}
	if err := g.Wait(); err != nil {
		return nil, err
	}

	items := make([]models.CalendarItem, 0, len(lessons)+len(events)+len(tasks))
	for i := range lessons {
		l := lessons[i]
		items = append(items, models.CalendarItem{
			Type:            "lesson",
			ID:              l.ID,
			Title:           models.LessonTitle(l),
			StartsAt:        l.ScheduledAt,
			DurationMinutes: l.DurationMinutes,
			Lesson:          &l,
		})
	}
	for i := range events {
		e := events[i]
		items = append(items, models.CalendarItem{
			Type:            "event",
			ID:              e.ID,
			Title:           e.Title,
			StartsAt:        e.StartsAt,
			DurationMinutes: e.DurationMinutes,
			Event:           &e,
		})
	}
	for i := range tasks {
		t := tasks[i]
		if t.ScheduledAt == nil { // задача без слота живёт только на канбане
			continue
		}
		duration := defaultTaskMinutes
		if t.DurationMinutes != nil {
			duration = *t.DurationMinutes
		}
		items = append(items, models.CalendarItem{
			Type:            "task",
			ID:              t.ID,
			Title:           t.Title,
			StartsAt:        *t.ScheduledAt,
			DurationMinutes: duration,
			Task:            &t,
		})
	}

	sort.Slice(items, func(i, j int) bool { return items[i].StartsAt.Before(items[j].StartsAt) })
	return items, nil
}

func (s *calendarService) GetConflicts(ctx context.Context, tutorID string, startsAt time.Time, durationMinutes int, excludeType string, excludeID *string) ([]models.CalendarItem, error) {
	if durationMinutes <= 0 {
		return nil, ErrBadRequest
	}
	end := startsAt.Add(time.Duration(durationMinutes) * time.Minute)
	return s.events.GetOccupiedInRange(ctx, tutorID,
		startsAt.Format(time.RFC3339), end.Format(time.RFC3339), excludeType, excludeID)
}
