package models_test

import (
	"strings"
	"testing"

	"tutorgo/models"
	"tutorgo/validator"
)

func TestCreateTaskRequest_AllowsLongHTMLTitle(t *testing.T) {
	html := "<ul>" + strings.Repeat("<li>пункт списка</li>", 100) + "</ul>"
	req := models.CreateTaskRequest{Title: html, Status: "not_urgent"}

	if errs := validator.Validate(req); errs != nil {
		t.Fatalf("ожидали проход валидации, получили ошибки: %v", errs)
	}
}
