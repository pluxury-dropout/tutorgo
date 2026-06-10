package handlers

import (
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"

	"tutorgo/models"
	"tutorgo/service"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type WhiteboardHandler struct {
	svc   service.WhiteboardService
	log   *slog.Logger
	wsHub *WbHubManager
}

func NewWhiteboardHandler(svc service.WhiteboardService, log *slog.Logger, wsHub *WbHubManager) *WhiteboardHandler {
	return &WhiteboardHandler{svc: svc, log: log, wsHub: wsHub}
}

func (h *WhiteboardHandler) GetBoardByCourse(c *gin.Context) {
	tutorID := c.GetString("tutorID")
	if tutorID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	result, err := h.svc.GetOrCreateBoard(c.Request.Context(), c.Param("courseId"), tutorID)
	if err != nil {
		h.log.Error("GetOrCreateBoard", slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	c.JSON(http.StatusOK, result)
}

func (h *WhiteboardHandler) CreatePage(c *gin.Context) {
	tutorID := c.GetString("tutorID")
	if tutorID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	var req models.CreateBoardPageRequest
	if !bindAndValidate(c, &req) {
		return
	}
	page, err := h.svc.CreatePage(c.Request.Context(), c.Param("boardId"), tutorID, req.Title)
	if err != nil {
		handleServiceError(c, err)
		return
	}
	c.JSON(http.StatusCreated, page)
}

func (h *WhiteboardHandler) UpdatePage(c *gin.Context) {
	tutorID := c.GetString("tutorID")
	if tutorID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	_ = tutorID
	var req models.UpdateBoardPageRequest
	if !bindAndValidate(c, &req) {
		return
	}
	// boardId comes as query param; service's GetPageByID verifies board ownership
	page, err := h.svc.UpdatePage(c.Request.Context(), c.Param("pageId"), c.Query("boardId"), req)
	if err != nil {
		handleServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, page)
}

func (h *WhiteboardHandler) DeletePage(c *gin.Context) {
	tutorID := c.GetString("tutorID")
	if tutorID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	_ = tutorID
	if err := h.svc.DeletePage(c.Request.Context(), c.Param("pageId"), c.Query("boardId")); err != nil {
		handleServiceError(c, err)
		return
	}
	c.JSON(http.StatusNoContent, nil)
}

func (h *WhiteboardHandler) CreateInvite(c *gin.Context) {
	tutorID := c.GetString("tutorID")
	if tutorID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	inv, err := h.svc.CreateInvite(c.Request.Context(), c.Param("boardId"))
	if err != nil {
		h.log.Error("CreateInvite", slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	c.JSON(http.StatusCreated, inv)
}

func (h *WhiteboardHandler) DeleteInvite(c *gin.Context) {
	tutorID := c.GetString("tutorID")
	if tutorID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	if err := h.svc.DeleteInvite(c.Request.Context(), c.Param("boardId")); err != nil {
		handleServiceError(c, err)
		return
	}
	c.JSON(http.StatusNoContent, nil)
}

func (h *WhiteboardHandler) JoinByInvite(c *gin.Context) {
	result, err := h.svc.ValidateInvite(c.Request.Context(), c.Param("token"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "invite not found"})
		return
	}
	c.JSON(http.StatusOK, result)
}

func (h *WhiteboardHandler) UploadAsset(c *gin.Context) {
	tutorID := c.GetString("tutorID")
	if tutorID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	// Apply 20MB limit for asset uploads
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 20<<20)

	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "file required"})
		return
	}
	defer file.Close()

	const maxSize = 20 << 20 // 20MB
	if header.Size > maxSize {
		c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": "file too large (max 20MB)"})
		return
	}

	dir := "uploads/board-assets"
	if err := os.MkdirAll(dir, 0755); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "storage error"})
		return
	}

	ext := filepath.Ext(header.Filename)
	filename := fmt.Sprintf("%s%s", uuid.New().String(), ext)
	dst := filepath.Join(dir, filename)

	out, err := os.Create(dst)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "storage error"})
		return
	}
	defer out.Close()

	if _, err := io.Copy(out, file); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "storage error"})
		return
	}

	mimeType := header.Header.Get("Content-Type")
	asset, err := h.svc.SaveAsset(c.Request.Context(), c.Param("boardId"), dst, mimeType, int(header.Size))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	c.JSON(http.StatusCreated, models.BoardAssetResponse{
		ID:  asset.ID,
		URL: fmt.Sprintf("/public/board-assets/%s", asset.ID),
	})
}

func (h *WhiteboardHandler) ServeAsset(c *gin.Context) {
	asset, err := h.svc.GetAsset(c.Request.Context(), c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}
	c.File(asset.FilePath)
}
