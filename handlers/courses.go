package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"tutorgo/db"
	"tutorgo/models"

	"github.com/jackc/pgx/v5"
)

type CourseHandler struct {
	conn *pgx.Conn
	log  *slog.Logger
}

func NewCourseHandler(conn *pgx.Conn, log *slog.Logger) *CourseHandler {
	return &CourseHandler{conn: conn, log: log}
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
	courses, err := db.GetCourses(h.conn, tutorID)
	if err != nil {
		http.Error(w, "Failed to retrieve courses", http.StatusInternalServerError)
		h.log.Error("Failed to get courses", slog.String("error", err.Error()))
		return
	}
	h.log.Info("Courses retrieved", slog.Int("count", len(courses)))
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(courses)
}

func (h *CourseHandler) createCourse(w http.ResponseWriter, r *http.Request, tutorID string) {
	var req models.CreateCourseRequest
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		http.Error(w, "Invalid data format", http.StatusBadRequest)
		return
	}
	course, err := db.CreateCourse(h.conn, req, tutorID)
	if err != nil {
		http.Error(w, "Failed to create course", http.StatusInternalServerError)
		h.log.Error("Failed to create course", slog.String("error", err.Error()))
		return
	}
	h.log.Info("Course created", slog.String("id", course.ID))
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(course)
}

func (h *CourseHandler) getCourseByID(w http.ResponseWriter, r *http.Request, tutorID string) {
	id := r.PathValue("id")
	course, err := db.GetCourseByID(h.conn, id, tutorID)
	if err != nil {
		http.Error(w, "Course not found", http.StatusNotFound)
		h.log.Error("Failed to get course", slog.String("id", id), slog.String("error", err.Error()))
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(course)
}

func (h *CourseHandler) updateCourse(w http.ResponseWriter, r *http.Request, tutorID string) {
	id := r.PathValue("id")
	var req models.UpdateCourseRequest
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		http.Error(w, "Invalid data format", http.StatusBadRequest)
		return
	}
	course, err := db.UpdateCourse(h.conn, id, tutorID, req)
	if err != nil {
		http.Error(w, "Failed to update course", http.StatusInternalServerError)
		h.log.Error("Failed to update course", slog.String("id", id), slog.String("error", err.Error()))
		return
	}
	h.log.Info("Course updated", slog.String("id", id))
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(course)
}

func (h *CourseHandler) deleteCourse(w http.ResponseWriter, r *http.Request, tutorID string) {
	id := r.PathValue("id")
	err := db.DeleteCourse(h.conn, id, tutorID)
	if err != nil {
		http.Error(w, "Failed to delete course", http.StatusInternalServerError)
		h.log.Error("Failed to delete course", slog.String("id", id), slog.String("error", err.Error()))
		return
	}
	h.log.Info("Course deleted", slog.String("id", id))
	w.WriteHeader(http.StatusNoContent)
}
