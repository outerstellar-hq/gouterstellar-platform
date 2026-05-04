package handler

import (
	"encoding/json"
	"errors"
	"log/slog"
	"math"
	"net/http"
	"strconv"

	"github.com/rygel/gouterstellar-platform/internal/model"
)

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

func getIntParam(r *http.Request, name string, defaultVal int) int {
	v := r.URL.Query().Get(name)
	if v == "" {
		return defaultVal
	}
	i, err := strconv.Atoi(v)
	if err != nil {
		return defaultVal
	}
	return i
}

func getInt64Param(r *http.Request, name string, defaultVal int64) int64 {
	v := r.URL.Query().Get(name)
	if v == "" {
		return defaultVal
	}
	i, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		return defaultVal
	}
	return i
}

func safeInt32(v int) int32 {
	if v > math.MaxInt32 {
		return math.MaxInt32
	}
	if v < 0 {
		return 0
	}
	return int32(v)
}

func handleServiceError(w http.ResponseWriter, err error) {
	var userNotFound *model.UserNotFoundError
	var weakPassword *model.WeakPasswordError
	var usernameExists *model.UsernameAlreadyExistsError
	var insufficientPerm *model.InsufficientPermissionError
	var validationErr *model.ValidationError
	var messageNotFound *model.MessageNotFoundError
	var contactNotFound *model.ContactNotFoundError

	switch {
	case errors.As(err, &userNotFound):
		writeError(w, http.StatusNotFound, err.Error())
	case errors.As(err, &weakPassword):
		writeError(w, http.StatusBadRequest, err.Error())
	case errors.As(err, &usernameExists):
		writeError(w, http.StatusConflict, err.Error())
	case errors.As(err, &insufficientPerm):
		writeError(w, http.StatusForbidden, err.Error())
	case errors.As(err, &validationErr):
		writeError(w, http.StatusBadRequest, err.Error())
	case errors.As(err, &messageNotFound):
		writeError(w, http.StatusNotFound, err.Error())
	case errors.As(err, &contactNotFound):
		writeError(w, http.StatusNotFound, err.Error())
	default:
		slog.Error("Unhandled service error", "error", err)
		writeError(w, http.StatusInternalServerError, "Internal server error")
	}
}
