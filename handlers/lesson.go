package handlers

import (
	"log/slog"
	"net/http"

	"tutorgo/models"
	"tutorgo/service"
)

type LessonHandler struct {
	service service.LessonService
	log     *slog.Logger
}

func NewLessonHandler(svc service.LessonService, log *slog.Logger) *LessonHandler {
	return &LessonHandler{service: svc, log: log}
}

func (h *LessonHandler) Handle(w http.ResponseWriter, r *http.Request) {
	tutorID := r.Context().Value("tutorID").(string)

	switch r.Method {
	case http.MethodGet:
		h.getLessons(w, r, tutorID)
	case http.MethodPost:
		h.createLesson(w, r, tutorID)
	default:
		respondError(w, http.StatusMethodNotAllowed, "Method not allowed")
	}
}

func (h *LessonHandler) HandleOne(w http.ResponseWriter, r *http.Request) {
	tutorID := r.Context().Value("tutorID").(string)

	switch r.Method {
	case http.MethodGet:
		h.getLessonByID(w, r, tutorID)
	case http.MethodPut:
		h.updateLesson(w, r, tutorID)
	case http.MethodDelete:
		h.deleteLesson(w, r, tutorID)
	default:
		respondError(w, http.StatusMethodNotAllowed, "Method not allowed")
	}
}

func (h *LessonHandler) getLessons(w http.ResponseWriter, r *http.Request, tutorID string) {
	courseID := r.URL.Query().Get("course_id")
	if courseID == "" {
		respondError(w, http.StatusBadRequest, "course_id is required")
		return
	}
	lessons, err := h.service.GetByCourse(courseID, tutorID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to retrieve lessons")
		h.log.Error("Failed to get lessons", slog.String("error", err.Error()))
		return
	}
	h.log.Info("Lessons retrieved", slog.Int("count", len(lessons)))
	respondJSON(w, http.StatusOK, lessons)
}

func (h *LessonHandler) createLesson(w http.ResponseWriter, r *http.Request, tutorID string) {
	var req models.CreateLessonRequest
	if !decodeAndValidate(w, r, &req) {
		return
	}
	lesson, err := h.service.Create(req, tutorID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to create lesson")
		h.log.Error("Failed to create lesson", slog.String("error", err.Error()))
		return
	}
	h.log.Info("Lesson created", slog.String("id", lesson.ID))
	respondJSON(w, http.StatusCreated, lesson)
}

func (h *LessonHandler) getLessonByID(w http.ResponseWriter, r *http.Request, tutorID string) {
	id := r.PathValue("id")
	lesson, err := h.service.GetByID(id, tutorID)
	if err != nil {
		respondError(w, http.StatusNotFound, "Lesson not found")
		h.log.Error("Failed to get lesson", slog.String("id", id), slog.String("error", err.Error()))
		return
	}
	respondJSON(w, http.StatusOK, lesson)
}

func (h *LessonHandler) updateLesson(w http.ResponseWriter, r *http.Request, tutorID string) {
	id := r.PathValue("id")
	var req models.UpdateLessonRequest
	if !decodeAndValidate(w, r, &req) {
		return
	}
	lesson, err := h.service.Update(id, req, tutorID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to update lesson")
		h.log.Error("Failed to update lesson", slog.String("id", id), slog.String("error", err.Error()))
		return
	}
	h.log.Info("Lesson updated", slog.String("id", id))
	respondJSON(w, http.StatusOK, lesson)
}

func (h *LessonHandler) deleteLesson(w http.ResponseWriter, r *http.Request, tutorID string) {
	id := r.PathValue("id")
	err := h.service.Delete(id, tutorID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to delete lesson")
		h.log.Error("Failed to delete lesson", slog.String("id", id), slog.String("error", err.Error()))
		return
	}
	h.log.Info("Lesson deleted", slog.String("id", id))
	w.WriteHeader(http.StatusNoContent)
}
