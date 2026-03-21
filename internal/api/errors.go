package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"sandbox-runtime/internal/state"
)

// writeJSON writes a JSON response with the given status code.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	_ = json.NewEncoder(w).Encode(v)
}

// writeError writes a structured JSON error response.
func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, ErrorResponse{
		Error: msg,
	})
}

// mapError converts internal errors into HTTP responses.
func (s *Server) mapError(w http.ResponseWriter, err error) {
	if err == nil {
		return
	}

	switch {
	case errors.Is(err, state.ErrInvalidSandbox):
		writeError(w, http.StatusBadRequest, err.Error())

	case errors.Is(err, state.ErrSandboxNotFound):
		writeError(w, http.StatusNotFound, err.Error())

	case errors.Is(err, state.ErrSandboxAlreadyExists):
		writeError(w, http.StatusConflict, err.Error())

	default:
		msg := "internal server error"
		// Expose full error if in development mode
		if s.Debug {
			msg = err.Error()
		}
		writeError(w, http.StatusInternalServerError, msg)
	}
}
