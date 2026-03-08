package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"tutorgo/models"
	"tutorgo/repository"
)

type StudentHandler struct {
	repo repository.StudentRepository
	log  *slog.Logger
}

func NewStudentHandler(repo repository.StudentRepository, log *slog.Logger) *StudentHandler {
	return &StudentHandler{repo: repo, log: log}
}

func (h *StudentHandler) Handle(w http.ResponseWriter, r *http.Request) {
	tutorID := r.Context().Value("tutorID").(string)

	switch r.Method {
	case http.MethodGet:
		h.getStudents(w, r, tutorID)
	case http.MethodPost:
		h.createStudent(w, r, tutorID)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *StudentHandler) HandleOne(w http.ResponseWriter, r *http.Request) {
	tutorID := r.Context().Value("tutorID").(string)

	switch r.Method {
	case http.MethodGet:
		h.getStudentByID(w, r, tutorID)
	case http.MethodPut:
		h.updateStudent(w, r, tutorID)
	case http.MethodDelete:
		h.deleteStudent(w, r, tutorID)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *StudentHandler) getStudents(w http.ResponseWriter, r *http.Request, tutorID string) {
	students, err := h.repo.GetAll(tutorID)
	if err != nil {
		http.Error(w, "Failed to retrieve students", http.StatusInternalServerError)
		h.log.Error("Failed to get students", slog.String("error", err.Error()))
		return
	}
	h.log.Info("Students retrieved", slog.Int("count", len(students)))
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(students)
}

func (h *StudentHandler) createStudent(w http.ResponseWriter, r *http.Request, tutorID string) {
	var req models.CreateStudentRequest
	if !decodeAndValidate(w, r, &req) {
		return
	}

	student, err := h.repo.Create(req, tutorID)
	if err != nil {
		http.Error(w, "Failed to create student", http.StatusInternalServerError)
		h.log.Error("Failed to create student", slog.String("error", err.Error()))
		return
	}
	h.log.Info("Student created", slog.String("id", student.ID))
	respondJSON(w, http.StatusCreated, student)
}

func (h *StudentHandler) getStudentByID(w http.ResponseWriter, r *http.Request, tutorID string) {
	id := r.PathValue("id")
	student, err := h.repo.GetByID(id, tutorID)
	if err != nil {
		http.Error(w, "Student not found", http.StatusNotFound)
		h.log.Error("Failed to get student", slog.String("id", id), slog.String("error", err.Error()))
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(student)
}

func (h *StudentHandler) updateStudent(w http.ResponseWriter, r *http.Request, tutorID string) {
	id := r.PathValue("id")
	var req models.UpdateStudentRequest
	if !decodeAndValidate(w, r, &req) {
		return
	}

	student, err := h.repo.Update(id, tutorID, req)
	if err != nil {
		http.Error(w, "Failed to update student", http.StatusInternalServerError)
		h.log.Error("Failed to update student", slog.String("id", id), slog.String("error", err.Error()))
		return
	}
	h.log.Info("Student updated", slog.String("id", id))
	respondJSON(w, http.StatusOK, student)
}

func (h *StudentHandler) deleteStudent(w http.ResponseWriter, r *http.Request, tutorID string) {
	id := r.PathValue("id")
	err := h.repo.Delete(id, tutorID)
	if err != nil {
		http.Error(w, "Failed to delete student", http.StatusInternalServerError)
		h.log.Error("Failed to delete student", slog.String("id", id), slog.String("error", err.Error()))
		return
	}
	h.log.Info("Student deleted", slog.String("id", id))
	w.WriteHeader(http.StatusNoContent)
}
