package httpapi

import (
	"bytes"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/thelong0705/deuce/internal/pkg/apperr"
)

// internalErrorBody is encoded once at startup, so reporting an encoding
// failure cannot itself fail to encode.
var internalErrorBody = mustEncode(errorResponse{
	Code:  apperr.ErrInternal.Code,
	Error: apperr.ErrInternal.Message,
})

var invalidBodyResponse = mustEncode(errorResponse{
	Code:  "invalid_body",
	Error: "invalid JSON body",
})

func mustEncode(v any) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		panic("httpapi: encode response body: " + err.Error())
	}
	return append(b, '\n')
}

type errorResponse struct {
	Code  string `json:"code"`
	Error string `json:"error"`
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	var buf bytes.Buffer

	if v != nil {
		if err := json.NewEncoder(&buf).Encode(v); err != nil {
			slog.Error("encode json response", "error", err)
			writeInternalError(w)
			return
		}
	}

	writeBody(w, status, buf.Bytes())
}

func writeBody(w http.ResponseWriter, status int, body []byte) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Length", strconv.Itoa(len(body)))
	w.WriteHeader(status)

	if _, err := w.Write(body); err != nil {
		// Usually the client disconnected. Nothing left to tell them.
		slog.Error("write json response", "error", err)
	}
}

func writeInternalError(w http.ResponseWriter) {
	writeBody(w, http.StatusInternalServerError, internalErrorBody)
}

// statusFor maps an error kind onto an HTTP status.
func statusFor(kind apperr.Kind) int {
	switch kind {
	case apperr.KindInvalid:
		return http.StatusBadRequest
	case apperr.KindNotFound:
		return http.StatusNotFound
	case apperr.KindConflict:
		return http.StatusConflict
	case apperr.KindForbidden:
		return http.StatusForbidden
	default:
		return http.StatusInternalServerError
	}
}

// writeAppError sends err to the client. Anything that is not an *apperr.Error
// becomes a generic internal error, so an unexpected failure cannot leak a
// query or a connection string.
func writeAppError(w http.ResponseWriter, err error) {
	var appErr *apperr.Error
	if !errors.As(err, &appErr) {
		slog.Error("unhandled error", "error", err)
		writeInternalError(w)
		return
	}

	writeJSON(w, statusFor(appErr.Kind), errorResponse{
		Code:  appErr.Code,
		Error: appErr.Message,
	})
}
