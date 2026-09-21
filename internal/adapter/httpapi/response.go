package httpapi

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
)

// writeJSON encodes into a buffer before touching the ResponseWriter. Encoding
// straight to w would send the status line first, leaving no way to report a
// failure -- and Encode streams, so a partial body may already be on the wire.
func writeJSON(w http.ResponseWriter, status int, v any) {
	var buf bytes.Buffer

	if v != nil {
		if err := json.NewEncoder(&buf).Encode(v); err != nil {
			slog.Error("encode json response", "error", err)
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Length", strconv.Itoa(buf.Len()))
	w.WriteHeader(status)

	if _, err := w.Write(buf.Bytes()); err != nil {
		// Usually the client disconnected. Nothing left to tell them.
		slog.Error("write json response", "error", err)
	}
}

type errorResponse struct {
	Error string `json:"error"`
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, errorResponse{Error: msg})
}
