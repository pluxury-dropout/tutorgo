package service_test

import (
	"context"
	"testing"
	"time"

	"tutorgo/models"
	"tutorgo/service"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// mockLessonCalendar — источник уроков для ленты (узкий интерфейс, один метод).
type mockLessonCalendar struct{ mock.Mock }

func (m *mockLessonCalendar) GetCalendar(ctx context.Context, tutorID, from, to string) ([]models.CalendarLesson, error) {
	args := m.Called(ctx, tutorID, from, to)
	return args.Get(0).([]models.CalendarLesson), args.Error(1)
}

const (
	feedFrom = "2026-09-01T00:00:00Z"
	feedTo   = "2026-09-08T00:00:00Z"
)

func at(hhmm string) time.Time {
	t, _ := time.Parse(time.RFC3339, "2026-09-02T"+hhmm+":00Z")
	return t
}

func TestCalendarService_GetFeed_MergesAndSorts(t *testing.T) {
	lessons := new(mockLessonCalendar)
	events := new(mockEventRepo)
	tasks := new(mockTaskRepo)
	svc := service.NewCalendarService(lessons, events, tasks)

	lessons.On("GetCalendar", mock.Anything, "tutor-1", feedFrom, feedTo).Return([]models.CalendarLesson{
		{ID: "l1", CourseID: "c1", ScheduledAt: at("17:00"), DurationMinutes: 60,
			Subject: "Математика", StudentName: ptr("Айгерим")},
	}, nil)
	events.On("GetByRange", mock.Anything, "tutor-1", feedFrom, feedTo).Return([]models.Event{
		{ID: "e1", Title: "Спортзал", Kind: "personal", StartsAt: at("07:00"), DurationMinutes: 90},
	}, nil)
	tasks.On("GetByRange", mock.Anything, "tutor-1", feedFrom, feedTo).Return([]models.Task{
		{ID: "t1", Title: "Проверить ДЗ", ScheduledAt: ptr(at("12:00"))},
		{ID: "t2", Title: "Без слота"}, // канбан-карточка: в ленте ей не место
	}, nil)

	items, err := svc.GetFeed(context.Background(), "tutor-1", feedFrom, feedTo, "")

	assert.NoError(t, err)
	assert.Equal(t, []string{"e1", "t1", "l1"}, []string{items[0].ID, items[1].ID, items[2].ID})
	assert.Equal(t, []string{"event", "task", "lesson"}, []string{items[0].Type, items[1].Type, items[2].Type})
	assert.Len(t, items, 3)

	// Заголовок урока собирается на сервере, специфика — во вложенном объекте.
	assert.Equal(t, "Математика — Айгерим", items[2].Title)
	assert.Equal(t, "c1", items[2].Lesson.CourseID)
	assert.Equal(t, "personal", items[0].Event.Kind)
	// Задача без длительности всё равно должна занять высоту в сетке.
	assert.Equal(t, 30, items[1].DurationMinutes)
}

func TestCalendarService_GetFeed_KindsFilter(t *testing.T) {
	lessons := new(mockLessonCalendar)
	events := new(mockEventRepo)
	tasks := new(mockTaskRepo)
	svc := service.NewCalendarService(lessons, events, tasks)

	events.On("GetByRange", mock.Anything, "tutor-1", feedFrom, feedTo).Return([]models.Event{
		{ID: "e1", Title: "Спортзал", StartsAt: at("07:00"), DurationMinutes: 90},
	}, nil)

	items, err := svc.GetFeed(context.Background(), "tutor-1", feedFrom, feedTo, "event")

	assert.NoError(t, err)
	assert.Len(t, items, 1)
	lessons.AssertNotCalled(t, "GetCalendar", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	tasks.AssertNotCalled(t, "GetByRange", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func TestCalendarService_GetConflicts_UsesSlotEnd(t *testing.T) {
	lessons := new(mockLessonCalendar)
	events := new(mockEventRepo)
	tasks := new(mockTaskRepo)
	svc := service.NewCalendarService(lessons, events, tasks)

	events.On("GetOccupiedInRange", mock.Anything, "tutor-1",
		"2026-09-02T17:00:00Z", "2026-09-02T18:00:00Z", "lesson", (*string)(nil),
	).Return([]models.CalendarItem{{Type: "event", ID: "e1", Title: "Спортзал"}}, nil)

	conflicts, err := svc.GetConflicts(context.Background(), "tutor-1", at("17:00"), 60, "lesson", nil)

	assert.NoError(t, err)
	assert.Len(t, conflicts, 1)
	events.AssertExpectations(t)
}
