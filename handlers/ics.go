package handlers

import (
	"log/slog"
	"net/http"

	"tutorgo/service"

	"github.com/gin-gonic/gin"
)

type ICSHandler struct {
	service service.ICSService
	log     *slog.Logger
}

func NewICSHandler(svc service.ICSService, log *slog.Logger) *ICSHandler {
	return &ICSHandler{service: svc, log: log}
}

// Feed — GET /ics/:token. Публичная: календарь-клиент ходит сюда без всякой
// авторизации, ссылка и есть секрет. Отзыв токена закрывает доступ.
func (h *ICSHandler) Feed(c *gin.Context) {
	body, err := h.service.Render(c.Request.Context(), c.Param("token"))
	if err != nil {
		handleServiceError(c, err)
		return
	}
	// Имя файла нужно тем клиентам, которые скачивают фид, а не подписываются.
	c.Header("Content-Disposition", `inline; filename="amida.ics"`)
	c.Data(http.StatusOK, "text/calendar; charset=utf-8", []byte(body))
}

// EnsureLink — POST /ics/link: выдать токен подписки, создав его при первом
// запросе. Повторный вызов отдаёт тот же — подписка в телефоне не должна
// умирать от захода в профиль.
//
// Отдаём токен, а не готовый URL: API живёт на своём хосте, и знает его
// только клиент (NEXT_PUBLIC_API_URL). Собирать ссылку на сервере значило бы
// завести ещё одну переменную окружения ради конкатенации.
func (h *ICSHandler) EnsureLink(c *gin.Context) {
	tutorID := c.GetString("tutorID")
	if tutorID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	token, err := h.service.EnsureToken(c.Request.Context(), tutorID)
	if err != nil {
		h.log.Error("Failed to issue ICS token", slog.String("error", err.Error()))
		handleServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"token": token})
}

// RevokeLink — DELETE /ics/link: обнулить токен. Старая ссылка сразу перестаёт
// открываться, следующий EnsureLink выдаст новую.
func (h *ICSHandler) RevokeLink(c *gin.Context) {
	tutorID := c.GetString("tutorID")
	if tutorID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	if err := h.service.Revoke(c.Request.Context(), tutorID); err != nil {
		h.log.Error("Failed to revoke ICS token", slog.String("error", err.Error()))
		handleServiceError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}
