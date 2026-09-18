package httpapi

import (
	"encoding/json"
	"net/http"
)

// decodeJSON decodes the request body into dst. On a missing or malformed body it writes the
// D-52 VALIDATION_ERROR envelope and returns false; callers must stop handling the request
// when it does.
func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	if err := json.NewDecoder(r.Body).Decode(dst); err != nil {
		writeError(w, http.StatusBadRequest, CodeValidation, "Request body must be valid JSON", nil)
		return false
	}
	return true
}
