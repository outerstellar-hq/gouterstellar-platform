package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/rygel/gouterstellar-platform/internal/model"
	"github.com/rygel/gouterstellar-platform/internal/web"
)

type stubPollService struct {
	created  model.CreatePollRequest
	deadline *time.Time
	voteErr  error
}

func (s *stubPollService) Create(_ context.Context, input model.CreatePollInput, creatorID uuid.UUID) (*model.PollWithResults, error) {
	s.created = model.CreatePollRequest{Question: input.Question, Options: input.Options, MultiChoice: input.MultiChoice}
	s.deadline = input.Deadline
	return &model.PollWithResults{Poll: model.Poll{SyncID: "poll-1", CreatorID: creatorID, Question: input.Question}, Options: []model.PollOption{}}, nil
}
func (s *stubPollService) Get(context.Context, string, *uuid.UUID) (*model.PollWithResults, error) {
	return &model.PollWithResults{}, nil
}
func (s *stubPollService) CastVote(context.Context, string, int64, uuid.UUID) (*model.PollWithResults, error) {
	if s.voteErr != nil {
		return nil, s.voteErr
	}
	return &model.PollWithResults{}, nil
}
func (s *stubPollService) RemoveVote(context.Context, string, int64, uuid.UUID) error { return nil }
func (s *stubPollService) Close(context.Context, string, uuid.UUID) error             { return nil }
func (s *stubPollService) Delete(context.Context, string, uuid.UUID) error            { return nil }
func (s *stubPollService) ListOpen(context.Context, int32, int32) ([]model.PollSummary, error) {
	return []model.PollSummary{}, nil
}

func TestPollAPICreateRequiresAuthAndParsesDeadline(t *testing.T) {
	stub := &stubPollService{}
	api := NewPollAPI(stub)
	router := chi.NewRouter()
	router.Post("/polls", api.Create)
	body := `{"question":"Choose?","options":["A","B"],"deadline":"2030-01-02T03:04:05Z"}`

	unauthenticated := httptest.NewRecorder()
	router.ServeHTTP(unauthenticated, httptest.NewRequest(http.MethodPost, "/polls", strings.NewReader(body)))
	require.Equal(t, http.StatusUnauthorized, unauthenticated.Code)

	request := web.WithUser(httptest.NewRequest(http.MethodPost, "/polls", strings.NewReader(body)), &model.User{ID: uuid.New()})
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusCreated, recorder.Code)
	require.Equal(t, "Choose?", stub.created.Question)
	require.NotNil(t, stub.deadline)
	require.Equal(t, "2030-01-02T03:04:05Z", stub.deadline.Format(time.RFC3339))
}

func TestPollAPIVoteMapsConflictsAndRejectsInvalidOption(t *testing.T) {
	stub := &stubPollService{voteErr: &model.PollConflictError{Message: "Poll is closed"}}
	api := NewPollAPI(stub)
	router := chi.NewRouter()
	router.Post("/polls/{syncId}/vote", api.Vote)
	user := &model.User{ID: uuid.New()}

	invalidRequest := web.WithUser(httptest.NewRequest(http.MethodPost, "/polls/poll-1/vote", strings.NewReader(`{"optionId":0}`)), user)
	invalid := httptest.NewRecorder()
	router.ServeHTTP(invalid, invalidRequest)
	require.Equal(t, http.StatusBadRequest, invalid.Code)

	conflictRequest := web.WithUser(httptest.NewRequest(http.MethodPost, "/polls/poll-1/vote", strings.NewReader(`{"optionId":10}`)), user)
	conflict := httptest.NewRecorder()
	router.ServeHTTP(conflict, conflictRequest)
	require.Equal(t, http.StatusConflict, conflict.Code)
	require.Contains(t, conflict.Body.String(), "Poll is closed")
}

func TestPollAPIRejectsMalformedDeadline(t *testing.T) {
	api := NewPollAPI(&stubPollService{})
	router := chi.NewRouter()
	router.Post("/polls", api.Create)
	request := httptest.NewRequest(http.MethodPost, "/polls", strings.NewReader(`{"question":"Choose?","options":["A","B"],"deadline":"tomorrow"}`))
	request = web.WithUser(request, &model.User{ID: uuid.New()})
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusBadRequest, recorder.Code)
	require.Contains(t, recorder.Body.String(), "RFC 3339")
}

func TestPollAPIListEnforcesPageSizeBoundary(t *testing.T) {
	api := NewPollAPI(&stubPollService{})
	router := chi.NewRouter()
	router.Get("/polls", api.List)

	accepted := httptest.NewRecorder()
	router.ServeHTTP(accepted, httptest.NewRequest(http.MethodGet, "/polls?limit=100", nil))
	require.Equal(t, http.StatusOK, accepted.Code)

	rejected := httptest.NewRecorder()
	router.ServeHTTP(rejected, httptest.NewRequest(http.MethodGet, "/polls?limit=101", nil))
	require.Equal(t, http.StatusBadRequest, rejected.Code)
}
