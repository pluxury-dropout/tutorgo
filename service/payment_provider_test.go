package service_test

import (
	"context"
	"net/http/httptest"
	"testing"

	"tutorgo/service"

	"github.com/stretchr/testify/assert"
)

func TestStubProvider_SatisfiesInterfaceAndErrors(t *testing.T) {
	var p service.PaymentProvider = service.StubProvider{}

	_, err := p.InitPayment(context.Background(), "o1", "t1", "monthly", 10000)
	assert.Error(t, err)
	_, err = p.Charge(context.Background(), "o1", "tok", 10000)
	assert.Error(t, err)
	_, err = p.CheckStatus(context.Background(), "o1")
	assert.Error(t, err)
	_, err = p.ParseCallback(httptest.NewRequest("POST", "/subscription/webhook", nil))
	assert.Error(t, err)
}
