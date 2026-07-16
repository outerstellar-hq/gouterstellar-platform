package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/rygel/gouterstellar-platform/internal/model"
	"github.com/rygel/gouterstellar-platform/internal/web"
)

type stubMessageVoteService struct {
	direction model.VoteDirection
	syncID    string
}

func (s *stubMessageVoteService) Vote(_ context.Context, _ uuid.UUID, syncID string, direction model.VoteDirection) (*model.VoteScore, error) {
	s.direction = direction
	s.syncID = syncID
	return &model.VoteScore{MessageSyncID: syncID, Upvotes: 1, NetScore: 1, UserVote: &direction}, nil
}
func (s *stubMessageVoteService) RemoveVote(context.Context, uuid.UUID, string) error { return nil }
func (s *stubMessageVoteService) GetScore(_ context.Context, syncID string, _ *uuid.UUID) (*model.VoteScore, error) {
	return &model.VoteScore{MessageSyncID: syncID}, nil
}
func (s *stubMessageVoteService) GetScores(context.Context, []string, *uuid.UUID) (map[string]model.VoteScore, error) {
	return nil, nil
}

func TestVoteAPIPostValidatesAndReturnsScore(t *testing.T) {
	stub := &stubMessageVoteService{}
	api := NewVoteAPI(stub)
	router := chi.NewRouter()
	router.Post("/messages/{syncId}/vote", api.Post)
	request := httptest.NewRequest(http.MethodPost, "/messages/srv_abc/vote", strings.NewReader(`{"direction":1}`))
	request = web.WithUser(request, &model.User{ID: uuid.New()})
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.JSONEq(t, `{"messageSyncId":"srv_abc","upvotes":1,"downvotes":0,"netScore":1,"userVote":1}`, recorder.Body.String())
	require.Equal(t, model.VoteUp, stub.direction)
	require.Equal(t, "srv_abc", stub.syncID)
}

func TestVoteAPIPostRejectsUnauthenticatedAndMalformedRequests(t *testing.T) {
	api := NewVoteAPI(&stubMessageVoteService{})
	router := chi.NewRouter()
	router.Post("/messages/{syncId}/vote", api.Post)

	unauthenticated := httptest.NewRecorder()
	router.ServeHTTP(unauthenticated, httptest.NewRequest(http.MethodPost, "/messages/srv_abc/vote", strings.NewReader(`{"direction":1}`)))
	require.Equal(t, http.StatusUnauthorized, unauthenticated.Code)

	request := httptest.NewRequest(http.MethodPost, "/messages/srv_abc/vote", strings.NewReader(`{"direction":0}`))
	request = web.WithUser(request, &model.User{ID: uuid.New()})
	invalid := httptest.NewRecorder()
	router.ServeHTTP(invalid, request)
	require.Equal(t, http.StatusBadRequest, invalid.Code)
}

func TestVoteComponentFallsBackToMessageListWithoutHTMX(t *testing.T) {
	stub := &stubMessageVoteService{}
	handler := NewComponentsHandler(nil, nil, stub, nil, nil)
	router := chi.NewRouter()
	router.Post("/components/messages/{syncId}/vote", handler.Vote)
	form := url.Values{"direction": {"-1"}}
	request := httptest.NewRequest(http.MethodPost, "/components/messages/srv_abc/vote", strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request = web.WithUser(request, &model.User{ID: uuid.New()})
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusSeeOther, recorder.Code)
	require.Equal(t, "/messages", recorder.Header().Get("Location"))
	require.Equal(t, model.VoteDown, stub.direction)
}
