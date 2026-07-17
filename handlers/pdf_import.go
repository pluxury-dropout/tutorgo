package handlers

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"tutorgo/models"
	"tutorgo/pdftool"
	"tutorgo/service"
	"tutorgo/storage"
)

type PdfImportHandler struct {
	svc   service.PdfImportService
	store *storage.Client
	log   *slog.Logger
	// infoFn — pdftool.Info за полем: тесты подставляют фейк, exec не нужен.
	infoFn func(ctx context.Context, path string) ([]models.PageSizePt, error)
}

func NewPdfImportHandler(svc service.PdfImportService, store *storage.Client, log *slog.Logger) *PdfImportHandler {
	return &PdfImportHandler{svc: svc, store: store, log: log, infoFn: pdftool.Info}
}

const maxPdfBytes = 50 << 20 // держать в синхроне с MAX_ASSET_BYTES фронта

// Upload — preflight: оригинал в S3, метаданные страниц клиенту.
// POST /boards/:boardId/pdf (multipart: file, page_id)
func (h *PdfImportHandler) Upload(c *gin.Context) {
	tutorID := c.GetString("tutorID")
	if tutorID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	pageID := c.PostForm("page_id")
	if pageID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "page_id required"})
		return
	}
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "file required"})
		return
	}
	defer file.Close()
	if header.Size > maxPdfBytes {
		c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": "file too large (max 50MB)"})
		return
	}

	// pdfinfo работает с путём — кладём во временный файл.
	tmp, err := os.CreateTemp("", "pdfupload-*.pdf")
	if err != nil {
		h.log.Error("pdf upload: temp", "err", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	defer os.Remove(tmp.Name())
	if err := c.SaveUploadedFile(header, tmp.Name()); err != nil {
		h.log.Error("pdf upload: save", "err", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	sizes, err := h.infoFn(c.Request.Context(), tmp.Name())
	if err != nil {
		h.log.Warn("pdf upload: pdfinfo", "err", err)
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "не удалось прочитать PDF"})
		return
	}

	src, err := os.Open(tmp.Name())
	if err != nil {
		h.log.Error("pdf upload: reopen", "err", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	defer src.Close()
	key := fmt.Sprintf("board-pdf/%s%s", uuid.New().String(), filepath.Ext(header.Filename))
	if err := h.store.Put(c.Request.Context(), key, src, header.Size, "application/pdf"); err != nil {
		h.log.Error("pdf upload: s3", "err", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "storage error"})
		return
	}

	resp, err := h.svc.CreateImport(c.Request.Context(), c.Param("boardId"), pageID, tutorID, key, sizes)
	if err != nil {
		if errors.Is(err, service.ErrForbidden) {
			c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
			return
		}
		h.log.Error("pdf upload: create import", "err", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	c.JSON(http.StatusOK, resp)
}

// Start фиксирует диапазон и ставит джобу.
// POST /pdf-imports/:id/start {from, to}
func (h *PdfImportHandler) Start(c *gin.Context) {
	tutorID := c.GetString("tutorID")
	if tutorID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	var req models.StartPdfImportRequest
	if !bindAndValidate(c, &req) {
		return
	}
	resp, err := h.svc.Start(c.Request.Context(), c.Param("id"), tutorID, req.From, req.To)
	if err != nil {
		if errors.Is(err, service.ErrForbidden) {
			c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
			return
		}
		h.log.Warn("pdf import start", "err", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, resp)
}
