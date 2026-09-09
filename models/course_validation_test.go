package models_test

import (
	"testing"
	"time"

	"tutorgo/models"
	"tutorgo/validator"
)

// Создание и правка курса обязаны вести себя одинаково: нулевая цена законна
// (бесплатный, пробный или ещё не оценённый курс — неявный курс из календаря
// создаётся именно с нулём), отрицательная — нет. До спеки 2026-09-06 правка
// стояла на `required,gt=0` и отдавала 400 на нуле.
func TestCourseRequests_PriceZeroAllowedNegativeRejected(t *testing.T) {
	create := func(price float64) any {
		return models.CreateCourseRequest{Subject: "Математика", PricePerCycle: price, LessonsPerCycle: 8}
	}
	update := func(price float64) any {
		return models.UpdateCourseRequest{
			Subject: "Математика", PricePerCycle: price, LessonsPerCycle: 8, StartedAt: time.Now(),
		}
	}

	for _, tc := range []struct {
		name    string
		req     any
		wantErr bool
	}{
		{"создание, цена 0", create(0), false},
		{"создание, цена -1", create(-1), true},
		{"правка, цена 0", update(0), false},
		{"правка, цена -1", update(-1), true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			errs := validator.Validate(tc.req)
			if tc.wantErr && errs == nil {
				t.Fatal("ожидали ошибку валидации, её нет")
			}
			if !tc.wantErr && errs != nil {
				t.Fatalf("ожидали проход валидации, получили: %v", errs)
			}
		})
	}
}
