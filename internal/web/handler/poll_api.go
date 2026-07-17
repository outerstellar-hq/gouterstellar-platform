package handler

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	extplatform "github.com/outerstellar-hq/gouterstellar-platform/platform"

	"github.com/outerstellar-hq/gouterstellar-platform/internal/model"
	"github.com/outerstellar-hq/gouterstellar-platform/internal/web"
)

const (
	maxPollRequestBytes = 16 * 1024
	maxPollPageSize     = 100
)

type pollService interface {
	Create(context.Context, model.CreatePollInput, uuid.UUID) (*model.PollWithResults, error)
	Get(context.Context, string, *uuid.UUID) (*model.PollWithResults, error)
	CastVote(context.Context, string, int64, uuid.UUID) (*model.PollWithResults, error)
	RemoveVote(context.Context, string, int64, uuid.UUID) error
	Close(context.Context, string, uuid.UUID) error
	Delete(context.Context, string, uuid.UUID) error
	ListOpen(context.Context, int32, int32) ([]model.PollSummary, error)
}

type PollAPI struct {
	service pollService
}

func NewPollAPI(service pollService) *PollAPI {
	return &PollAPI{service: service}
}

func (h *PollAPI) ContributeRoutes(ctx *extplatform.ContributionContext) error {
	ctx.Routes.API(http.MethodPost, "/api/v1/polls", "Create poll", http.HandlerFunc(h.Create))
	ctx.Routes.API(http.MethodGet, "/api/v1/polls", "List open polls", http.HandlerFunc(h.List))
	ctx.Routes.API(http.MethodGet, "/api/v1/polls/{syncId}", "Get poll results", http.HandlerFunc(h.Get))
	ctx.Routes.API(http.MethodPost, "/api/v1/polls/{syncId}/vote", "Cast poll vote", http.HandlerFunc(h.Vote))
	ctx.Routes.API(http.MethodDelete, "/api/v1/polls/{syncId}/vote", "Remove poll vote", http.HandlerFunc(h.RemoveVote))
	ctx.Routes.API(http.MethodPost, "/api/v1/polls/{syncId}/close", "Close poll", http.HandlerFunc(h.Close))
	ctx.Routes.API(http.MethodDelete, "/api/v1/polls/{syncId}", "Delete poll", http.HandlerFunc(h.Delete))
	return nil
}

func (h *PollAPI) Create(w http.ResponseWriter, r *http.Request) {
	user := web.UserFromRequest(r)
	if user == nil {
		writeError(w, http.StatusUnauthorized, "Authentication required")
		return
	}
	var request model.CreatePollRequest
	if err := decodeJSONBody(w, r, maxPollRequestBytes, &request); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid poll request")
		return
	}
	deadline, ok := parsePollDeadline(w, request.Deadline)
	if !ok {
		return
	}
	result, err := h.service.Create(r.Context(), model.CreatePollInput{
		Question: request.Question, Options: request.Options,
		MultiChoice: request.MultiChoice, Deadline: deadline,
	}, user.ID)
	if err != nil {
		handleServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, result)
}

func parsePollDeadline(w http.ResponseWriter, deadline *string) (*time.Time, bool) {
	if deadline == nil {
		return nil, true
	}
	parsed, err := time.Parse(time.RFC3339, *deadline)
	if err != nil {
		writeError(w, http.StatusBadRequest, "deadline must be an RFC 3339 timestamp")
		return nil, false
	}
	return &parsed, true
}

func (h *PollAPI) List(w http.ResponseWriter, r *http.Request) {
	limit := getIntParam(r, "limit", 20)
	offset := getIntParam(r, "offset", 0)
	if limit <= 0 || limit > maxPollPageSize || offset < 0 {
		writeError(w, http.StatusBadRequest, "limit must be between 1 and 100 and offset must not be negative")
		return
	}
	polls, err := h.service.ListOpen(r.Context(), safeInt32(limit), safeInt32(offset))
	if err != nil {
		handleServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, polls)
}

func (h *PollAPI) Get(w http.ResponseWriter, r *http.Request) {
	var userID *uuid.UUID
	if user := web.UserFromRequest(r); user != nil {
		userID = &user.ID
	}
	result, err := h.service.Get(r.Context(), chi.URLParam(r, "syncId"), userID)
	if err != nil {
		handleServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *PollAPI) Vote(w http.ResponseWriter, r *http.Request) {
	user := web.UserFromRequest(r)
	if user == nil {
		writeError(w, http.StatusUnauthorized, "Authentication required")
		return
	}
	var request model.CastPollVoteRequest
	if err := decodeJSONBody(w, r, maxPollRequestBytes, &request); err != nil || request.OptionID <= 0 {
		writeError(w, http.StatusBadRequest, "optionId must be a positive integer")
		return
	}
	result, err := h.service.CastVote(r.Context(), chi.URLParam(r, "syncId"), request.OptionID, user.ID)
	if err != nil {
		handleServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *PollAPI) RemoveVote(w http.ResponseWriter, r *http.Request) {
	user := web.UserFromRequest(r)
	if user == nil {
		writeError(w, http.StatusUnauthorized, "Authentication required")
		return
	}
	optionID, err := strconv.ParseInt(r.URL.Query().Get("optionId"), 10, 64)
	if err != nil || optionID <= 0 {
		writeError(w, http.StatusBadRequest, "optionId must be a positive integer")
		return
	}
	if err := h.service.RemoveVote(r.Context(), chi.URLParam(r, "syncId"), optionID, user.ID); err != nil {
		handleServiceError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *PollAPI) Close(w http.ResponseWriter, r *http.Request) {
	user := web.UserFromRequest(r)
	if user == nil {
		writeError(w, http.StatusUnauthorized, "Authentication required")
		return
	}
	if err := h.service.Close(r.Context(), chi.URLParam(r, "syncId"), user.ID); err != nil {
		handleServiceError(w, err)
		return
	}
	writeText(w, http.StatusOK, "Poll closed")
}

func (h *PollAPI) Delete(w http.ResponseWriter, r *http.Request) {
	user := web.UserFromRequest(r)
	if user == nil {
		writeError(w, http.StatusUnauthorized, "Authentication required")
		return
	}
	if err := h.service.Delete(r.Context(), chi.URLParam(r, "syncId"), user.ID); err != nil {
		handleServiceError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
