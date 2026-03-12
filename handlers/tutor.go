package handlers

import (
	"errors"
	"log/slog"
	"net/http"

	"tutorgo/models"
	"tutorgo/service"

	"github.com/jackc/pgx/v5/pgconn"
)

type TutorHandler struct {
	service service.TutorService
	log     *slog.Logger
}

func NewTutorHandler(repo service.TutorService, log *slog.Logger) *TutorHandler {
	return &TutorHandler{service: repo, log: log}
}

func (h *TutorHandler) Handle(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.getTutors(w, r)
	case http.MethodPost:
		h.createTutor(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *TutorHandler) HandleOne(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.getTutorByID(w, r)
	case http.MethodPut:
		h.updateTutor(w, r)
	case http.MethodDelete:
		h.deleteTutor(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *TutorHandler) getTutors(w http.ResponseWriter, r *http.Request) {
	tutors, err := h.service.GetAll()
	if err != nil {
		http.Error(w, "Failed to retrieve tutors", http.StatusInternalServerError)
		h.log.Error("Failed to get tutors", slog.String("error", err.Error()))
		return
	}
	h.log.Info("Tutors retrieved", slog.Int("count", len(tutors)))
	respondJSON(w, http.StatusOK, tutors)
}

func (h *TutorHandler) createTutor(w http.ResponseWriter, r *http.Request) {
	var req models.CreateTutorRequest
	if !decodeAndValidate(w, r, &req) {
		return
	}
	tutor, err := h.service.Create(req, req.Password)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			http.Error(w, "Tutor with this email already exists", http.StatusConflict)
			h.log.Warn("Duplicate email", slog.String("email", req.Email))
			return
		}
		http.Error(w, "Failed to create tutor", http.StatusInternalServerError)
		h.log.Error("Failed to create tutor", slog.String("error", err.Error()))
		return
	}
	h.log.Info("Tutor created", slog.String("id", tutor.ID), slog.String("name", tutor.FirstName+" "+tutor.LastName), slog.String("email", tutor.Email))
	respondJSON(w, http.StatusCreated, tutor)
}

func (h *TutorHandler) getTutorByID(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	tutor, err := h.service.GetByID(id)
	if err != nil {
		http.Error(w, "Tutor not found", http.StatusNotFound)
		h.log.Error("Failed to get tutor", slog.String("id", id), slog.String("error", err.Error()))
		return
	}
	h.log.Info("Tutor retrieved", slog.String("id", id))
	respondJSON(w, http.StatusOK, tutor)

}

func (h *TutorHandler) updateTutor(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req models.UpdateTutorRequest
	if !decodeAndValidate(w, r, &req) {
		return
	}
	tutor, err := h.service.Update(id, req)
	if err != nil {
		http.Error(w, "Failed to update tutor", http.StatusInternalServerError)
		h.log.Error("Failed to update tutor", slog.String("id", id), slog.String("error", err.Error()))
		return
	}
	h.log.Info("Tutor updated", slog.String("id", id))
	respondJSON(w, http.StatusOK, tutor)
}

func (h *TutorHandler) deleteTutor(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	err := h.service.Delete(id)
	if err != nil {
		http.Error(w, "Failed to delete tutor", http.StatusInternalServerError)
		h.log.Error("Failed to delete tutor", slog.String("id", id), slog.String("error", err.Error()))
		return
	}
	h.log.Info("Tutor deleted", slog.String("id", id))
	w.WriteHeader(http.StatusNoContent)
}
