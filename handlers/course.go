package handlers

import (
	"log/slog"
	"net/http"

	"tutorgo/models"
	"tutorgo/service"
)

type CourseHandler struct {
	service service.CourseService
	log     *slog.Logger
}

func NewCourseHandler(svc service.CourseService, log *slog.Logger) *CourseHandler {
	return &CourseHandler{service: svc, log: log}
}

func (h *CourseHandler) Handle(w http.ResponseWriter, r *http.Request) {
	tutorID := r.Context().Value("tutorID").(string)

	switch r.Method {
	case http.MethodGet:
		h.getCourses(w, r, tutorID)
	case http.MethodPost:
		h.createCourse(w, r, tutorID)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *CourseHandler) HandleOne(w http.ResponseWriter, r *http.Request) {
	tutorID := r.Context().Value("tutorID").(string)

	switch r.Method {
	case http.MethodGet:
		h.getCourseByID(w, r, tutorID)
	case http.MethodPut:
		h.updateCourse(w, r, tutorID)
	case http.MethodDelete:
		h.deleteCourse(w, r, tutorID)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *CourseHandler) getCourses(w http.ResponseWriter, r *http.Request, tutorID string) {
	courses, err := h.service.GetAll(tutorID)
	if err != nil {
		http.Error(w, "Failed to retrieve courses", http.StatusInternalServerError)
		h.log.Error("Failed to get courses", slog.String("error", err.Error()))
		return
	}
	h.log.Info("Courses retrieved", slog.Int("count", len(courses)))
	respondJSON(w, http.StatusOK, courses)

}

func (h *CourseHandler) createCourse(w http.ResponseWriter, r *http.Request, tutorID string) {
	var req models.CreateCourseRequest
	if !decodeAndValidate(w, r, &req) {
		return
	}
	course, err := h.service.Create(req, tutorID)
	if err != nil {
		http.Error(w, "Failed to create course", http.StatusInternalServerError)
		h.log.Error("Failed to create course", slog.String("error", err.Error()))
		return
	}
	h.log.Info("Course created", slog.String("id", course.ID))
	respondJSON(w, http.StatusCreated, course)

}

func (h *CourseHandler) getCourseByID(w http.ResponseWriter, r *http.Request, tutorID string) {
	id := r.PathValue("id")
	course, err := h.service.GetByID(id, tutorID)
	if err != nil {
		http.Error(w, "Course not found", http.StatusNotFound)
		h.log.Error("Failed to get course", slog.String("id", id), slog.String("error", err.Error()))
		return
	}
	respondJSON(w, http.StatusOK, course)

}

func (h *CourseHandler) updateCourse(w http.ResponseWriter, r *http.Request, tutorID string) {
	id := r.PathValue("id")
	var req models.UpdateCourseRequest
	if !decodeAndValidate(w, r, &req) {
		return
	}
	course, err := h.service.Update(id, tutorID, req)
	if err != nil {
		http.Error(w, "Failed to update course", http.StatusInternalServerError)
		h.log.Error("Failed to update course", slog.String("id", id), slog.String("error", err.Error()))
		return
	}
	h.log.Info("Course updated", slog.String("id", id))
	respondJSON(w, http.StatusOK, course)
}

func (h *CourseHandler) deleteCourse(w http.ResponseWriter, r *http.Request, tutorID string) {
	id := r.PathValue("id")
	err := h.service.Delete(id, tutorID)
	if err != nil {
		http.Error(w, "Failed to delete course", http.StatusInternalServerError)
		h.log.Error("Failed to delete course", slog.String("id", id), slog.String("error", err.Error()))
		return
	}
	h.log.Info("Course deleted", slog.String("id", id))
	w.WriteHeader(http.StatusNoContent)
}
