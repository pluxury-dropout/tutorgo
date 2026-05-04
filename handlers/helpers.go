package handlers

import (
	"errors"
	"net/http"

	"tutorgo/service"
	"tutorgo/validator"

	"github.com/gin-gonic/gin"
)

func bindAndValidate(c *gin.Context, req any) bool {
	if err := c.ShouldBindJSON(req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid data format"})
		return false
	}
	if validationErrors := validator.Validate(req); validationErrors != nil {
		c.JSON(http.StatusBadRequest, validationErrors)
		return false
	}
	return true
}

func handleServiceError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
	case errors.Is(err, service.ErrForbidden):
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
	case errors.Is(err, service.ErrConflict):
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
	case errors.Is(err, service.ErrBadRequest):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
	}
}
