package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

func Auth(jwtSecret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
			return
		}

		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || parts[0] != "Bearer" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Invalid token format"})
			return
		}

		token, err := jwt.ParseWithClaims(
			parts[1],
			jwt.MapClaims{},
			func(token *jwt.Token) (interface{}, error) {
				return []byte(jwtSecret), nil
			},
			jwt.WithValidMethods([]string{"HS256"}),
		)
		if err != nil || !token.Valid {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Invalid token"})
			return
		}

		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Invalid token"})
			return
		}

		tutorID, ok := claims["id"].(string)
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Invalid token"})
			return
		}

		// устанавливаем в gin контекст (для новых gin-хендлеров)
		c.Set("tutorID", tutorID)
		// устанавливаем в request context (для старых stdlib-хендлеров)
		ctx := context.WithValue(c.Request.Context(), "tutorID", tutorID)
		c.Request = c.Request.WithContext(ctx)

		c.Next()
	}
}
