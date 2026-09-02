package service_test

import (
	"context"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"tutorgo/models"
	"tutorgo/service"

	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type mockICSFeed struct{ mock.Mock }

func (m *mockICSFeed) GetFeed(ctx context.Context, tutorID, from, to, kinds string) ([]models.CalendarItem, error) {
	args := m.Called(ctx, tutorID, from, to, kinds)
	return args.Get(0).([]models.CalendarItem), args.Error(1)
}

type mockICSTokens struct{ mock.Mock }

func (m *mockICSTokens) GetIDByICSToken(ctx context.Context, token string) (string, error) {
	args := m.Called(ctx, token)
	return args.String(0), args.Error(1)
}
func (m *mockICSTokens) GetICSToken(ctx context.Context, tutorID string) (string, error) {
	args := m.Called(ctx, tutorID)
	return args.String(0), args.Error(1)
}
func (m *mockICSTokens) SetICSToken(ctx context.Context, tutorID, token string) error {
	return m.Called(ctx, tutorID, token).Error(0)
}

func icsSvc(feed *mockICSFeed, tokens *mockICSTokens) service.ICSService {
	return service.NewICSService(feed, tokens)
}

func lessonItem(title string, at time.Time, minutes int, status string) models.CalendarItem {
	return models.CalendarItem{
		Type: "lesson", ID: "l-1", Title: title, StartsAt: at, DurationMinutes: minutes,
		Lesson: &models.CalendarLesson{ID: "l-1", Status: status},
	}
}

// Календарь должен открываться в Google и Apple: обязательные заголовки на
// месте, время в UTC, каждое занятие — своим VEVENT со стабильным UID.
func TestICS_RendersCalendar(t *testing.T) {
	feed, tokens := new(mockICSFeed), new(mockICSTokens)

	at := time.Date(2026, time.September, 2, 12, 0, 0, 0, time.UTC)
	tokens.On("GetIDByICSToken", mock.Anything, "tok-1").Return(tutorID, nil)
	feed.On("GetFeed", mock.Anything, tutorID, mock.Anything, mock.Anything, "lesson,event").
		Return([]models.CalendarItem{lessonItem("Математика", at, 60, "scheduled")}, nil)

	out, err := icsSvc(feed, tokens).Render(context.Background(), "tok-1")

	require.NoError(t, err)
	require.True(t, strings.HasPrefix(out, "BEGIN:VCALENDAR\r\n"), "начало календаря")
	require.True(t, strings.HasSuffix(out, "END:VCALENDAR\r\n"), "конец календаря")
	require.Contains(t, out, "VERSION:2.0\r\n")
	require.Contains(t, out, "UID:lesson-l-1@amida.kz\r\n")
	require.Contains(t, out, "DTSTART:20260902T120000Z\r\n")
	require.Contains(t, out, "DTEND:20260902T130000Z\r\n")
	require.Contains(t, out, "SUMMARY:Математика\r\n")
}

// Запятая и точка с запятой в ICS — разделители полей: незаэкранированные,
// они рвут файл, и календарь молча отказывается его импортировать.
func TestICS_EscapesSpecialCharacters(t *testing.T) {
	feed, tokens := new(mockICSFeed), new(mockICSTokens)

	at := time.Date(2026, time.September, 2, 12, 0, 0, 0, time.UTC)
	tokens.On("GetIDByICSToken", mock.Anything, "tok-1").Return(tutorID, nil)
	feed.On("GetFeed", mock.Anything, tutorID, mock.Anything, mock.Anything, "lesson,event").
		Return([]models.CalendarItem{lessonItem("Алгебра, 9 класс; повтор", at, 60, "scheduled")}, nil)

	out, err := icsSvc(feed, tokens).Render(context.Background(), "tok-1")

	require.NoError(t, err)
	require.Contains(t, out, `SUMMARY:Алгебра\, 9 класс\; повтор`)
}

// Длинная строка складывается по RFC 5545: не длиннее 75 октетов, перенос
// начинается с пробела. Кириллица весит два байта на букву, поэтому предел
// достигается вдвое быстрее, а резать посреди руны нельзя — клиент покажет
// мусор вместо заголовка.
func TestICS_FoldsLongLines(t *testing.T) {
	feed, tokens := new(mockICSFeed), new(mockICSTokens)

	at := time.Date(2026, time.September, 2, 12, 0, 0, 0, time.UTC)
	long := "Подготовка к олимпиаде по математике для группы девятиклассников, второй тур"
	tokens.On("GetIDByICSToken", mock.Anything, "tok-1").Return(tutorID, nil)
	feed.On("GetFeed", mock.Anything, tutorID, mock.Anything, mock.Anything, "lesson,event").
		Return([]models.CalendarItem{lessonItem(long, at, 60, "scheduled")}, nil)

	out, err := icsSvc(feed, tokens).Render(context.Background(), "tok-1")
	require.NoError(t, err)

	for _, l := range strings.Split(out, "\r\n") {
		require.LessOrEqual(t, len(l), 75, "строка длиннее 75 октетов: %q", l)
	}
	require.True(t, utf8.ValidString(out), "перенос разрезал руну")
	// Склеиваем обратно, как это делает клиент: переносы вида CRLF+пробел.
	// Запятая в заголовке к этому моменту уже экранирована.
	unfolded := strings.ReplaceAll(out, "\r\n ", "")
	require.Contains(t, unfolded, "SUMMARY:"+strings.ReplaceAll(long, ",", `\,`))
}

// Отменённое занятие в подписке не нужно: календарь показывает, когда
// репетитор занят, а отменённый урок никого не занимает.
func TestICS_SkipsCancelledLessons(t *testing.T) {
	feed, tokens := new(mockICSFeed), new(mockICSTokens)

	at := time.Date(2026, time.September, 2, 12, 0, 0, 0, time.UTC)
	tokens.On("GetIDByICSToken", mock.Anything, "tok-1").Return(tutorID, nil)
	feed.On("GetFeed", mock.Anything, tutorID, mock.Anything, mock.Anything, "lesson,event").
		Return([]models.CalendarItem{
			lessonItem("Отменённая", at, 60, "cancelled"),
			lessonItem("Живая", at.Add(2*time.Hour), 60, "scheduled"),
		}, nil)

	out, err := icsSvc(feed, tokens).Render(context.Background(), "tok-1")

	require.NoError(t, err)
	require.NotContains(t, out, "SUMMARY:Отменённая")
	require.Contains(t, out, "SUMMARY:Живая")
}

// Чужая или отозванная ссылка не должна отдавать ничей календарь.
func TestICS_UnknownTokenIsNotFound(t *testing.T) {
	feed, tokens := new(mockICSFeed), new(mockICSTokens)

	tokens.On("GetIDByICSToken", mock.Anything, "нет-такого").Return("", service.ErrNotFound)

	_, err := icsSvc(feed, tokens).Render(context.Background(), "нет-такого")

	require.ErrorIs(t, err, service.ErrNotFound)
	feed.AssertNotCalled(t, "GetFeed")
}

// Из репозитория приходит pgx.ErrNoRows, а не доменная ошибка: без перевода
// публичная ручка отвечала бы на чужую ссылку пятисоткой вместо 404.
func TestICS_MissingRowBecomesNotFound(t *testing.T) {
	feed, tokens := new(mockICSFeed), new(mockICSTokens)

	tokens.On("GetIDByICSToken", mock.Anything, "нет-такого").Return("", pgx.ErrNoRows)

	_, err := icsSvc(feed, tokens).Render(context.Background(), "нет-такого")

	require.ErrorIs(t, err, service.ErrNotFound)
}

// Повторный запрос ссылки отдаёт ту же: иначе подписка, уже добавленная в
// телефон, тихо умирала бы при каждом заходе в профиль.
func TestICS_EnsureTokenKeepsExisting(t *testing.T) {
	feed, tokens := new(mockICSFeed), new(mockICSTokens)

	tokens.On("GetICSToken", mock.Anything, tutorID).Return("уже-есть", nil)

	tok, err := icsSvc(feed, tokens).EnsureToken(context.Background(), tutorID)

	require.NoError(t, err)
	require.Equal(t, "уже-есть", tok)
	tokens.AssertNotCalled(t, "SetICSToken")
}

func TestICS_EnsureTokenIssuesWhenMissing(t *testing.T) {
	feed, tokens := new(mockICSFeed), new(mockICSTokens)

	tokens.On("GetICSToken", mock.Anything, tutorID).Return("", nil)
	tokens.On("SetICSToken", mock.Anything, tutorID, mock.MatchedBy(func(tok string) bool {
		return len(tok) >= 32
	})).Return(nil)

	tok, err := icsSvc(feed, tokens).EnsureToken(context.Background(), tutorID)

	require.NoError(t, err)
	require.NotEmpty(t, tok)
	tokens.AssertExpectations(t)
}

// Отзыв — это обнуление: старая ссылка перестаёт открываться, следующий
// запрос выдаст новую.
func TestICS_RevokeClearsToken(t *testing.T) {
	feed, tokens := new(mockICSFeed), new(mockICSTokens)

	tokens.On("SetICSToken", mock.Anything, tutorID, "").Return(nil)

	require.NoError(t, icsSvc(feed, tokens).Revoke(context.Background(), tutorID))
	tokens.AssertExpectations(t)
}
