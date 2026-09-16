package httpapi

import (
	"encoding/json"
	"net/http"
)

// Error codes of the D-52 error envelope.
const (
	CodeValidation   = "VALIDATION_ERROR"
	CodeUnauthorized = "UNAUTHORIZED"
	CodeForbidden    = "FORBIDDEN"
	CodeNotFound     = "NOT_FOUND"
	CodeConflict     = "CONFLICT"
	CodeRateLimited  = "RATE_LIMITED"
	CodeInternal     = "INTERNAL"
)

// ErrorBody is the JSON shape of every error response (D-52).
type ErrorBody struct {
	Error ErrorDetail `json:"error"`
}

// ErrorDetail carries a machine-readable code, a human-readable message and optional per-field details.
type ErrorDetail struct {
	Code    string              `json:"code"`
	Message string              `json:"message"`
	Details map[string][]string `json:"details,omitempty"`
}

// writeJSON encodes v as the response body with the given status.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	// The status line is already on the wire; an encode failure here can only be a broken
	// connection, which the client will observe on its own.
	_ = json.NewEncoder(w).Encode(v)
}

// writeError writes a D-52 envelope. details may be nil.
func writeError(w http.ResponseWriter, status int, code, message string, details map[string][]string) {
	writeJSON(w, status, ErrorBody{Error: ErrorDetail{Code: code, Message: message, Details: details}})
}
