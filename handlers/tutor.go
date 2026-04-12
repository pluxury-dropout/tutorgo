package handlers

import (
	"log/slog"
	"net/http"

	"tutorgo/models"
	"tutorgo/service"
)

type TutorHandler struct {
	service service.TutorService
	log     *slog.Logger
}

func NewTutorHandler(svc service.TutorService, log *slog.Logger) *TutorHandler {
	return &TutorHandler{service: svc, log: log}
}

func (h *TutorHandler) Handle(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.getTutors(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *TutorHandler) HandleOne(w http.ResponseWriter, r *http.Request) {
	tutorID := r.Context().Value("tutorID").(string)
	id := r.PathValue("id")

	if id != tutorID {
		respondError(w, http.StatusForbidden, "access denied")
		return
	}

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
