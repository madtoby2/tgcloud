package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
)

// Shared helpers
func getInt64Param(r *http.Request, key string) int64 {
	s := chi.URLParam(r, key)
	if s == "" {
		s = r.URL.Query().Get(key)
	}
	v, _ := strconv.ParseInt(s, 10, 64)
	return v
}

func getQueryInt(r *http.Request, key string, def int) int {
	s := r.URL.Query().Get(key)
	if s == "" {
		return def
	}
	v, _ := strconv.Atoi(s)
	return v
}

func decodeJSON(r *http.Request, v interface{}) error {
	return json.NewDecoder(r.Body).Decode(v)
}
