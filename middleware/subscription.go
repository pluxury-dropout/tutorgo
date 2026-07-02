package middleware

import (
	"context"
	"net/http"

	"tutorgo/service"

	"github.com/gin-gonic/gin"
)

// SubscriptionState — узкий интерфейс, чтобы middleware не зависел от всего сервиса.
type SubscriptionState interface {
	State(ctx context.Context, tutorID string) (string, error)
}

// RequireActiveSubscription блокирует бизнес-ручки при истёкшей подписке (402).
// Ставится ПОСЛЕ middleware.Auth (нужен tutorID в контексте).
func RequireActiveSubscription(svc SubscriptionState) gin.HandlerFunc {
	return func(c *gin.Context) {
		tutorID := c.GetString("tutorID")
		if tutorID == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}
		state, err := svc.State(c.Request.Context(), tutorID)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
			return
		}
		if state == service.StateBlocked {
			c.AbortWithStatusJSON(http.StatusPaymentRequired, gin.H{"error": "subscription_required"})
			return
		}
		c.Next()
	}
}
