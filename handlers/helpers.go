package handlers

import (
	"encoding/json"
	"net/http"
	"tutorgo/validator"
)

func respondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func respondError(w http.ResponseWriter, status int, message string) {
	respondJSON(w, status, map[string]string{"error": message})
}

func decodeAndValidate(w http.ResponseWriter, r *http.Request, req interface{}) bool {
	if err := json.NewDecoder(r.Body).Decode(req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid data format")
		return false
	}
	if validationErrors := validator.Validate(req); validationErrors != nil {
		respondJSON(w, http.StatusBadRequest, validationErrors)
		return false
	}
	return true
}
