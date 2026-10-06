package api

import (
	"encoding/json"
	"net/http"
)

// errorResponse is the JSON body returned by writeError.
type errorResponse struct {
	Error string `json:"error"`
}

// writeJSON sets the JSON content type, writes status, and encodes payload.
func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if payload == nil {
		return
	}
	_ = json.NewEncoder(w).Encode(payload)
}

// writeError writes a JSON {"error": message} body with the given status.
func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, errorResponse{Error: message})
}
