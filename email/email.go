// Package email отправляет транзакционные письма через Resend.
// ponytail: одна функция поверх официального SDK; смена провайдера = правка NewSender,
// а не архитектуры — сервисы видят только тип Sender.
package email

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/resend/resend-go/v3"
)

// Sender шлёт письмо. Возвращается из NewSender с зашитыми ключом/адресом, чтобы
// сервисы не тащили конфиг и легко подменяли отправку фейком в тестах.
type Sender func(ctx context.Context, to, subject, htmlBody string) error

// NewSender: если apiKey пуст — dev-режим, письмо логируется вместо отправки.
// ponytail: пустой ключ = dev-режим, тело в лог; прод обязан задать RESEND_API_KEY.
func NewSender(apiKey, from string, log *slog.Logger) Sender {
	if apiKey == "" {
		return func(_ context.Context, to, subject, htmlBody string) error {
			log.Warn("RESEND_API_KEY not set — email not sent (dev mode)",
				slog.String("to", to), slog.String("subject", subject), slog.String("body", htmlBody))
			return nil
		}
	}
	// NewCustomClient, а не NewClient: дефолтный http.Client у SDK ждёт минуту,
	// а отправка висит внутри ручки регистрации, ответа которой ждёт человек.
	client := resend.NewCustomClient(&http.Client{Timeout: 10 * time.Second}, apiKey)
	return func(ctx context.Context, to, subject, htmlBody string) error {
		_, err := client.Emails.SendWithContext(ctx, &resend.SendEmailRequest{
			From:    from,
			To:      []string{to},
			Subject: subject,
			Html:    htmlBody,
		})
		return err
	}
}
