package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"tutorgo/models"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// Односторонний ICS-фид: репетитор подписывается на ссылку в Google Календаре
// и видит расписание в телефоне без всякого OAuth. Двусторонняя синхронизация —
// отдельная работа, здесь только чтение.

// ICSHorizon — сколько отдавать в обе стороны. Прошлое нужно: подписка
// показывает историю занятий, а не только будущее.
const ICSHorizon = 6 * 30 * 24 * time.Hour

type icsFeedSource interface {
	GetFeed(ctx context.Context, tutorID, from, to, kinds string) ([]models.CalendarItem, error)
}

type icsTokenStore interface {
	GetIDByICSToken(ctx context.Context, token string) (string, error)
	GetICSToken(ctx context.Context, tutorID string) (string, error)
	SetICSToken(ctx context.Context, tutorID, token string) error
}

type ICSService interface {
	// Render отдаёт готовый text/calendar по публичному токену.
	Render(ctx context.Context, token string) (string, error)
	// EnsureToken возвращает ссылку репетитора, выпуская её при первом запросе.
	EnsureToken(ctx context.Context, tutorID string) (string, error)
	// Revoke обнуляет токен: подписка по старой ссылке перестаёт работать.
	Revoke(ctx context.Context, tutorID string) error
}

type icsService struct {
	feed   icsFeedSource
	tokens icsTokenStore
}

func NewICSService(feed icsFeedSource, tokens icsTokenStore) ICSService {
	return &icsService{feed: feed, tokens: tokens}
}

func (s *icsService) EnsureToken(ctx context.Context, tutorID string) (string, error) {
	token, err := s.tokens.GetICSToken(ctx, tutorID)
	if err != nil {
		return "", err
	}
	if token != "" {
		return token, nil
	}
	token = uuid.NewString()
	if err := s.tokens.SetICSToken(ctx, tutorID, token); err != nil {
		return "", err
	}
	return token, nil
}

func (s *icsService) Revoke(ctx context.Context, tutorID string) error {
	return s.tokens.SetICSToken(ctx, tutorID, "")
}

func (s *icsService) Render(ctx context.Context, token string) (string, error) {
	tutorID, err := s.tokens.GetIDByICSToken(ctx, token)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", fmt.Errorf("ics: %w", ErrNotFound)
	}
	if err != nil {
		return "", err
	}
	now := time.Now()
	items, err := s.feed.GetFeed(ctx,
		tutorID,
		now.Add(-ICSHorizon).Format(time.RFC3339),
		now.Add(ICSHorizon).Format(time.RFC3339),
		"lesson,event", // задачи занятостью не считаются — их в подписке нет
	)
	if err != nil {
		return "", err
	}
	return renderICS(items, now), nil
}

func renderICS(items []models.CalendarItem, stamp time.Time) string {
	var b strings.Builder
	line := func(s string) { b.WriteString(s); b.WriteString("\r\n") }

	line("BEGIN:VCALENDAR")
	line("VERSION:2.0")
	line("PRODID:-//Amida//TutorGo//RU")
	line("CALSCALE:GREGORIAN")
	line("METHOD:PUBLISH")
	line("X-WR-CALNAME:Amida")

	for _, item := range items {
		// Отменённый урок никого не занимает — в подписке ему делать нечего.
		if item.Lesson != nil && item.Lesson.Status == "cancelled" {
			continue
		}
		end := item.StartsAt.Add(time.Duration(item.DurationMinutes) * time.Minute)

		line("BEGIN:VEVENT")
		// UID стабилен между обновлениями фида: иначе календарь на каждой
		// синхронизации считал бы занятие новым и плодил дубликаты.
		line(fmt.Sprintf("UID:%s-%s@amida.kz", item.Type, item.ID))
		line("DTSTAMP:" + icsTime(stamp))
		line("DTSTART:" + icsTime(item.StartsAt))
		line("DTEND:" + icsTime(end))
		line(fold("SUMMARY:" + icsEscape(item.Title)))
		if item.Event != nil && item.Event.Location != "" {
			line(fold("LOCATION:" + icsEscape(item.Event.Location)))
		}
		if item.Event != nil && item.Event.Notes != "" {
			line(fold("DESCRIPTION:" + icsEscape(item.Event.Notes)))
		}
		if item.Lesson != nil && item.Lesson.Notes != "" {
			line(fold("DESCRIPTION:" + icsEscape(item.Lesson.Notes)))
		}
		line("END:VEVENT")
	}

	line("END:VCALENDAR")
	return b.String()
}

func icsTime(t time.Time) string {
	return t.UTC().Format("20060102T150405Z")
}

// icsEscape экранирует разделители полей: незаэкранированные запятая и точка с
// запятой рвут строку на части, и календарь молча отказывается импортировать
// файл целиком. Порядок важен — обратный слэш первым.
func icsEscape(s string) string {
	return strings.NewReplacer(
		`\`, `\\`,
		`;`, `\;`,
		`,`, `\,`,
		"\n", `\n`,
		"\r", "",
	).Replace(s)
}

// fold складывает длинную строку по RFC 5545: не длиннее 75 октетов, перенос
// начинается с пробела. Режем по границам рун — заголовок с кириллицей иначе
// развалился бы на середине символа.
func fold(s string) string {
	const limit = 75
	if len(s) <= limit {
		return s
	}
	var b strings.Builder
	width := 0
	for _, r := range s {
		n := len(string(r))
		if width+n > limit {
			b.WriteString("\r\n ")
			width = 1 // ведущий пробой уже занял октет
		}
		b.WriteRune(r)
		width += n
	}
	return b.String()
}
