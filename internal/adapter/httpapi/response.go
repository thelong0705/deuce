package httpapi

import (
	"encoding/json"
	"log/slog"
	"net/http"
)

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	if v == nil {
		return
	}

	if err := json.NewEncoder(w).Encode(v); err != nil {
		// Status and headers are already on the wire, so there is no way left
		// to tell the client. Log it and move on.
		slog.Error("write json response", "error", err)
	}
}

type errorResponse struct {
	Error string `json:"error"`
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, errorResponse{Error: msg})
}
