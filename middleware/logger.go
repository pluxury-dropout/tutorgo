package middleware

import (
	"log/slog"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// slowRequest — порог, выше которого запрос перестаёт быть штатным и уезжает в
// Warn. Все наши запросы к БД укладываются в сотню миллисекунд; секунда — это
// уже либо внешний вызов (S3, LiveKit, Resend), либо голодание пула, либо
// что-то, чего мы не ждали. В общем потоке Info такие тонут.
const slowRequest = time.Second

func Logger(log *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		if c.Request.URL.RawQuery != "" {
			path += "?" + c.Request.URL.RawQuery
		}

		c.Next()

		took := time.Since(start)

		// WebSocket отдельным событием, и вот почему это не косметика:
		// ServeWS заканчивается блокирующим readPump, то есть gin-хендлер не
		// возвращается, пока соединение живо. В общем потоке "http" такая
		// запись выглядит как HTTP-запрос длиной в целый урок — и ровно так
		// она попадает в перцентили латентности, которые считает платформа.
		// Один урок на 40 минут задирает p99 сильнее, чем все реальные
		// тормоза вместе взятые.
		if strings.HasPrefix(c.Request.URL.Path, "/ws/") {
			log.Info("ws session closed",
				slog.String("path", path),
				slog.Duration("lived", took),
			)
			return
		}

		attrs := []any{
			slog.String("method", c.Request.Method),
			slog.String("path", path),
			slog.Int("status", c.Writer.Status()),
			slog.Duration("duration", took),
		}
		if took >= slowRequest {
			log.Warn("http slow", attrs...)
			return
		}
		log.Info("http", attrs...)
	}
}
