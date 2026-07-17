package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	extplatform "github.com/outerstellar-hq/gouterstellar-platform/platform"

	"github.com/outerstellar-hq/gouterstellar-platform/internal/model"
	"github.com/outerstellar-hq/gouterstellar-platform/internal/web"
)

const maxVoteRequestBytes = 1024

type VoteAPI struct {
	voteService messageVoteService
}

func NewVoteAPI(voteService messageVoteService) *VoteAPI {
	return &VoteAPI{voteService: voteService}
}

func (h *VoteAPI) ContributeRoutes(ctx *extplatform.ContributionContext) error {
	ctx.Routes.API(http.MethodGet, "/api/v1/messages/{syncId}/vote", "Get message vote score", http.HandlerFunc(h.Get))
	ctx.Routes.API(http.MethodPost, "/api/v1/messages/{syncId}/vote", "Vote on message", http.HandlerFunc(h.Post))
	ctx.Routes.API(http.MethodDelete, "/api/v1/messages/{syncId}/vote", "Remove message vote", http.HandlerFunc(h.Delete))
	return nil
}

func (h *VoteAPI) Get(w http.ResponseWriter, r *http.Request) {
	var userID *uuid.UUID
	if user := web.UserFromRequest(r); user != nil {
		userID = &user.ID
	}
	score, err := h.voteService.GetScore(r.Context(), chi.URLParam(r, "syncId"), userID)
	if err != nil {
		handleServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, score)
}

func (h *VoteAPI) Post(w http.ResponseWriter, r *http.Request) {
	user := web.UserFromRequest(r)
	if user == nil {
		writeError(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	var request model.VoteRequest
	if err := decodeJSONBody(w, r, maxVoteRequestBytes, &request); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid vote request")
		return
	}
	if !request.Direction.Valid() {
		writeError(w, http.StatusBadRequest, "direction must be 1 or -1")
		return
	}

	score, err := h.voteService.Vote(r.Context(), user.ID, chi.URLParam(r, "syncId"), request.Direction)
	if err != nil {
		handleServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, score)
}

func (h *VoteAPI) Delete(w http.ResponseWriter, r *http.Request) {
	user := web.UserFromRequest(r)
	if user == nil {
		writeError(w, http.StatusUnauthorized, "Authentication required")
		return
	}
	if err := h.voteService.RemoveVote(r.Context(), user.ID, chi.URLParam(r, "syncId")); err != nil {
		handleServiceError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
