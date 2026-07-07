package service

import (
	"context"
	"errors"
	"net/http"
)

// Callback — провайдер-нейтральный разбор webhook/ответа платёжного провайдера.
type Callback struct {
	OrderID           string
	ProviderPaymentID string
	Status            string // "success" | "failed"
	CardToken         string // непусто на success CIT (галка "сохранить карту")
}

// PaymentProvider — единственный I/O-seam к платёжному провайдеру.
// Реализации: StubProvider (пока нет freedompay), freedompay (отдельный план),
// fakeProvider (тесты).
type PaymentProvider interface {
	// CIT: hosted-страница первой оплаты, возвращает redirect URL.
	InitPayment(ctx context.Context, orderID, tutorID, plan string, amount int) (redirectURL string, err error)
	// MIT: списание по сохранённому токену.
	Charge(ctx context.Context, orderID, token string, amount int) (providerPaymentID string, err error)
	// Reconciliation при ошибке Charge (провайдерский status_v2).
	// Возвращает "success" | "failed" | "unknown".
	CheckStatus(ctx context.Context, orderID string) (status string, err error)
	// Разбор + проверка подписи входящего webhook.
	ParseCallback(r *http.Request) (Callback, error)
}

// StubProvider — временная заглушка для проводки в router, пока не готов
// freedompay-адаптер (отдельный план).
// ponytail: stub до freedompay-адаптера — НЕ деплоить в прод с ним.
type StubProvider struct{}

var errStubProvider = errors.New("payment provider not configured")

func (StubProvider) InitPayment(context.Context, string, string, string, int) (string, error) {
	return "", errStubProvider
}
func (StubProvider) Charge(context.Context, string, string, int) (string, error) {
	return "", errStubProvider
}
func (StubProvider) CheckStatus(context.Context, string) (string, error) {
	return "unknown", errStubProvider
}
func (StubProvider) ParseCallback(*http.Request) (Callback, error) {
	return Callback{}, errStubProvider
}
