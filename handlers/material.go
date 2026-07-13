package handlers

import (
	"fmt"
	"log/slog"
	"net/http"
	"path/filepath"
	"time"

	"tutorgo/models"
	"tutorgo/service"
	"tutorgo/storage"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type MaterialHandler struct {
	svc   service.MaterialService
	store *storage.Client
	log   *slog.Logger
}

func NewMaterialHandler(svc service.MaterialService, store *storage.Client, log *slog.Logger) *MaterialHandler {
	return &MaterialHandler{svc: svc, store: store, log: log}
}

// optionalUUID возвращает nil для пустой строки — это корень дерева.
func optionalUUID(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// GET /materials?parent_id=
func (h *MaterialHandler) List(c *gin.Context) {
	tutorID := c.GetString("tutorID")
	if tutorID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	items, err := h.svc.List(c.Request.Context(), tutorID, optionalUUID(c.Query("parent_id")))
	if err != nil {
		handleServiceError(c, err)
		return
	}
	resp := make([]models.MaterialResponse, 0, len(items))
	for _, m := range items {
		resp = append(resp, models.NewMaterialResponse(m))
	}
	c.JSON(http.StatusOK, resp)
}

// POST /materials/folder
func (h *MaterialHandler) CreateFolder(c *gin.Context) {
	tutorID := c.GetString("tutorID")
	if tutorID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	var req models.CreateFolderRequest
	if !bindAndValidate(c, &req) {
		return
	}
	m, err := h.svc.CreateFolder(c.Request.Context(), tutorID, req)
	if err != nil {
		handleServiceError(c, err)
		return
	}
	c.JSON(http.StatusCreated, models.NewMaterialResponse(m))
}

// POST /materials — multipart: file, parent_id
func (h *MaterialHandler) Upload(c *gin.Context) {
	tutorID := c.GetString("tutorID")
	if tutorID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	// Лимит тела (50 МБ для multipart) уже стоит в глобальном middleware — как в UploadAsset.
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		h.log.Error("upload material: form file", "err", err, "content_type", c.ContentType())
		c.JSON(http.StatusBadRequest, gin.H{"error": "file required"})
		return
	}
	defer file.Close()

	const maxSize = 50 << 20 // 50MB
	if header.Size > maxSize {
		c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": "file too large (max 50MB)"})
		return
	}

	ext := filepath.Ext(header.Filename)
	key := fmt.Sprintf("materials/%s%s", uuid.New().String(), ext)
	mimeType := header.Header.Get("Content-Type")

	if err := h.store.Put(c.Request.Context(), key, file, header.Size, mimeType); err != nil {
		h.log.Error("upload material to storage", "err", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "storage error"})
		return
	}

	m, err := h.svc.CreateFile(c.Request.Context(), tutorID, header.Filename, key,
		mimeType, int(header.Size), optionalUUID(c.PostForm("parent_id")))
	if err != nil {
		// Строка не записалась — сносим только что залитый объект, иначе он осиротеет
		// в бакете (тот же откат, что в UploadAsset).
		_ = h.store.Remove(c.Request.Context(), key)
		handleServiceError(c, err)
		return
	}
	c.JSON(http.StatusCreated, models.NewMaterialResponse(m))
}

// DELETE /materials/:id
func (h *MaterialHandler) Delete(c *gin.Context) {
	tutorID := c.GetString("tutorID")
	if tutorID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	m, err := h.svc.Delete(c.Request.Context(), c.Param("id"), tutorID)
	if err != nil {
		handleServiceError(c, err)
		return
	}
	if m.Kind == "file" && m.FilePath != "" {
		// Объект чистим после БД: осиротевший объект дешевле, чем строка, указывающая
		// в пустоту.
		if err := h.store.Remove(c.Request.Context(), m.FilePath); err != nil {
			h.log.Error("remove material object", "err", err, "key", m.FilePath)
		}
	}
	c.Status(http.StatusNoContent)
}

// GET /materials/:id/url — 302 на presigned-ссылку.
func (h *MaterialHandler) GetURL(c *gin.Context) {
	tutorID := c.GetString("tutorID")
	if tutorID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	m, err := h.svc.GetFile(c.Request.Context(), c.Param("id"), tutorID)
	if err != nil {
		handleServiceError(c, err)
		return
	}
	// 4 часа: ссылку получает ученик по WS и переполучить её не может, поэтому она
	// должна пережить весь урок (board-assets хватает 20 минут — там картинка
	// перезапрашивается через редирект).
	url, err := h.store.PresignGet(c.Request.Context(), m.FilePath, 4*time.Hour)
	if err != nil {
		h.log.Error("presign material", "err", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "presigned link error"})
		return
	}
	// Ссылку отдаём телом, а не 302 (как board-assets): её потребляет fetch/XHR,
	// а после редиректа браузер пошёл бы к S3 кросс-доменным CORS-запросом и
	// упёрся бы в CORS бакета. У board-assets редирект работает лишь потому,
	// что его потребляет <img>, а картинки под CORS не попадают.
	c.JSON(http.StatusOK, gin.H{"url": url})
}
