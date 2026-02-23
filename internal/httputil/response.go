package httputil

import (
	"encoding/json"
	"log/slog"
	"net/http"
)

type ErrorBody struct {
	Error string `json:"error"`
}

func WriteJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Error("failed to encode JSON response", "error", err)
	}
}

func WriteError(w http.ResponseWriter, status int, message string) {
	WriteJSON(w, status, ErrorBody{Error: message})
}
