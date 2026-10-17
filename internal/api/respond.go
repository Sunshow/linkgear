package api

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/sunshow/linkgear/internal/service"
)

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, service.ErrNotFound):
		writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
	case errors.Is(err, service.ErrUnauthorized):
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": err.Error()})
	case errors.Is(err, service.ErrBadRequest),
		errors.Is(err, service.ErrInvalidBotType),
		errors.Is(err, service.ErrTokenUsed),
		errors.Is(err, service.ErrTokenExpired):
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
	default:
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
}

func bearer(r *http.Request) string {
	h := r.Header.Get("Authorization")
	const p = "Bearer "
	if len(h) > len(p) && (h[:7] == "Bearer " || h[:7] == "bearer ") {
		return h[7:]
	}
	return ""
}

func adminToken(r *http.Request) string {
	if t := bearer(r); t != "" {
		return t
	}
	if t := r.Header.Get("X-Admin-Token"); t != "" {
		return t
	}
	return r.URL.Query().Get("admin_token")
}
