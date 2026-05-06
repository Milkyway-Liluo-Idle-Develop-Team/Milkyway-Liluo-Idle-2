// Package httpx contains transport helpers for the HTTP API: JSON encoding,
// error translation from apperror, and small middleware utilities. It must
// not import any business module.
package httpx

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/Milkyway-Liluo-Idle-Develop-Team/Milkyway-Liluo-Idle-2/backend/internal/apperror"
	"github.com/Milkyway-Liluo-Idle-Develop-Team/Milkyway-Liluo-Idle-2/backend/internal/logging"
)

// envelope is the single JSON shape every HTTP response uses.
//
//	{ "data": ... }              // success
//	{ "error": { ... } }         // failure
type envelope struct {
	Data  any            `json:"data,omitempty"`
	Error *apperror.AppError `json:"error,omitempty"`
}

// JSON writes a successful JSON response.
func JSON(w http.ResponseWriter, status int, data any) {
	writeJSON(w, status, envelope{Data: data})
}

// NoContent writes a 204 with no body.
func NoContent(w http.ResponseWriter) {
	w.WriteHeader(http.StatusNoContent)
}

// Error translates any error into the canonical JSON envelope. Non-AppError
// values become a generic "internal" error and are logged (with the original
// cause) before the response is written.
func Error(w http.ResponseWriter, r *http.Request, err error) {
	if err == nil {
		return
	}

	ae, ok := apperror.As(err)
	if !ok {
		// Wrap unknown errors as internal so we never leak details.
		logging.FromContext(r.Context()).Error("internal error", "err", err.Error(), "path", r.URL.Path)
		ae = apperror.Internal("internal server error").WithCause(err)
	} else if ae.Code == apperror.CodeInternal {
		logging.FromContext(r.Context()).Error("internal error",
			"err", ae.Error(), "path", r.URL.Path)
	}

	writeJSON(w, statusFor(ae.Code), envelope{Error: ae})
}

// DecodeJSON reads a JSON body into dst, applying common safety rules
// (size limit, disallow unknown fields). Returns an apperror.BadRequest on
// failure so handlers can pass it straight to httpx.Error.
func DecodeJSON(r *http.Request, dst any) error {
	const maxBody = 1 << 20 // 1 MiB; tighten per route if needed
	r.Body = http.MaxBytesReader(nil, r.Body, maxBody)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return apperror.BadRequest("invalid JSON body").WithCause(err)
	}
	// Forbid trailing garbage so clients can't sneak in extra payloads.
	if dec.More() {
		return apperror.BadRequest("unexpected trailing data in body")
	}
	return nil
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(body); err != nil && !errors.Is(err, http.ErrHandlerTimeout) {
		// Header already written; just log.
		_ = err
	}
}

func statusFor(c apperror.Code) int {
	switch c {
	case apperror.CodeBadRequest, apperror.CodeValidation:
		return http.StatusBadRequest
	case apperror.CodeUnauthorized:
		return http.StatusUnauthorized
	case apperror.CodeForbidden:
		return http.StatusForbidden
	case apperror.CodeNotFound:
		return http.StatusNotFound
	case apperror.CodeConflict:
		return http.StatusConflict
	case apperror.CodeRateLimited:
		return http.StatusTooManyRequests
	case apperror.CodeUnavailable:
		return http.StatusServiceUnavailable
	default:
		return http.StatusInternalServerError
	}
}
