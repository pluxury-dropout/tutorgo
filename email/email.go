// Package email отправляет транзакционные письма через Resend одним HTTP-запросом.
// ponytail: одна функция, без SDK; смена провайдера = правка Send, не архитектуры.
package email

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"
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
	client := &http.Client{Timeout: 10 * time.Second}
	return func(ctx context.Context, to, subject, htmlBody string) error {
		body, _ := json.Marshal(map[string]string{
			"from":    from,
			"to":      to,
			"subject": subject,
			"html":    htmlBody,
		})
		req, err := http.NewRequestWithContext(ctx, http.MethodPost,
			"https://api.resend.com/emails", bytes.NewReader(body))
		if err != nil {
			return err
		}
		req.Header.Set("Authorization", "Bearer "+apiKey)
		req.Header.Set("Content-Type", "application/json")

		resp, err := client.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		if resp.StatusCode >= 300 {
			b, _ := io.ReadAll(resp.Body)
			return fmt.Errorf("resend: status %d: %s", resp.StatusCode, b)
		}
		return nil
	}
}
