package handlers

import (
	"net/http"

	"tutorgo/validator"

	"github.com/gin-gonic/gin"
)

func bindAndValidate(c *gin.Context, req interface{}) bool {
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
