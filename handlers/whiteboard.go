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

type WhiteboardHandler struct {
	svc   service.WhiteboardService
	log   *slog.Logger
	wsHub *WbHubManager
	store *storage.Client
}

func NewWhiteboardHandler(svc service.WhiteboardService, log *slog.Logger, wsHub *WbHubManager, store *storage.Client) *WhiteboardHandler {
	return &WhiteboardHandler{svc: svc, log: log, wsHub: wsHub, store: store}
}

func (h *WhiteboardHandler) GetBoardByCourse(c *gin.Context) {
	tutorID := c.GetString("tutorID")
	if tutorID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	result, err := h.svc.GetOrCreateBoard(c.Request.Context(), c.Param("courseId"), tutorID)
	if err != nil {
		handleServiceError(c, err)
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
	var req models.UpdateBoardPageRequest
	if !bindAndValidate(c, &req) {
		return
	}
	// Ownership is verified in the service via the page's board; any client-supplied
	// ?boardId= query param is ignored.
	page, err := h.svc.UpdatePage(c.Request.Context(), c.Param("pageId"), tutorID, req)
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
	if err := h.svc.DeletePage(c.Request.Context(), c.Param("pageId"), tutorID); err != nil {
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
	inv, err := h.svc.CreateInvite(c.Request.Context(), c.Param("boardId"), tutorID)
	if err != nil {
		handleServiceError(c, err)
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
	if err := h.svc.DeleteInvite(c.Request.Context(), c.Param("boardId"), tutorID); err != nil {
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

	ext := filepath.Ext(header.Filename)
	key := fmt.Sprintf("board-assets/%s%s", uuid.New().String(), ext)

	mimeType := header.Header.Get("Content-Type")
	if err := h.store.Put(c.Request.Context(), key, file, header.Size, mimeType); err != nil {
		h.log.Error("upload asset to storage", "err", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "storage error"})
		return
	}

	asset, err := h.svc.SaveAsset(c.Request.Context(), c.Param("boardId"), tutorID, key, mimeType, int(header.Size))
	if err != nil {
		// Remove the just-uploaded object if persisting the asset failed
		// (e.g. board not owned by this tutor).
		_ = h.store.Remove(c.Request.Context(), key)
		handleServiceError(c, err)
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
	presignedlink, err := h.store.PresignGet(c.Request.Context(), asset.FilePath, 20*time.Minute)
	if err != nil {
		h.log.Error("failed to get presigned link", "err", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "presigned link error"})
		return
	}
	c.Redirect(http.StatusFound, presignedlink)
}
