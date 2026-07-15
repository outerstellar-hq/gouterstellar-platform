package service

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"

	"github.com/rygel/gouterstellar-platform/internal/model"
	"github.com/rygel/gouterstellar-platform/internal/persistence"
	"github.com/rygel/gouterstellar-platform/internal/persistence/db"
)

type fakeVoteRepo struct {
	messageExists bool
	votes         map[uuid.UUID]int16
}

var _ persistence.VoteRepository = (*fakeVoteRepo)(nil)

func (r *fakeVoteRepo) WithTx(pgx.Tx) persistence.VoteRepository { return r }
func (r *fakeVoteRepo) LockMessage(context.Context, string) error {
	if !r.messageExists {
		return pgx.ErrNoRows
	}
	return nil
}
func (r *fakeVoteRepo) FindVote(_ context.Context, userID uuid.UUID, syncID string) (db.PltMessageVote, error) {
	direction, ok := r.votes[userID]
	if !ok {
		return db.PltMessageVote{}, pgx.ErrNoRows
	}
	return db.PltMessageVote{MessageSyncID: syncID, UserID: userID, Direction: direction}, nil
}
func (r *fakeVoteRepo) CreateVote(_ context.Context, userID uuid.UUID, _ string, direction int16) error {
	r.votes[userID] = direction
	return nil
}
func (r *fakeVoteRepo) UpdateVote(_ context.Context, userID uuid.UUID, _ string, direction int16) error {
	r.votes[userID] = direction
	return nil
}
func (r *fakeVoteRepo) DeleteVote(_ context.Context, userID uuid.UUID, _ string) error {
	delete(r.votes, userID)
	return nil
}
func (r *fakeVoteRepo) ListScores(_ context.Context, syncIDs []string, userID *uuid.UUID) (map[string]model.VoteScore, error) {
	scores := make(map[string]model.VoteScore, len(syncIDs))
	for _, syncID := range syncIDs {
		score := model.VoteScore{MessageSyncID: syncID}
		for _, direction := range r.votes {
			if direction == int16(model.VoteUp) {
				score.Upvotes++
			} else {
				score.Downvotes++
			}
		}
		score.NetScore = score.Upvotes - score.Downvotes
		if userID != nil {
			if direction, ok := r.votes[*userID]; ok {
				vote := model.VoteDirection(direction)
				score.UserVote = &vote
			}
		}
		scores[syncID] = score
	}
	return scores, nil
}

func TestVoteServiceStateTransitions(t *testing.T) {
	userID := uuid.New()
	repo := &fakeVoteRepo{messageExists: true, votes: make(map[uuid.UUID]int16)}
	service := NewVoteService(repo, &FakeTxRunner{})

	created, err := service.Vote(context.Background(), userID, "message-1", model.VoteUp)
	require.NoError(t, err)
	require.Equal(t, int32(1), created.NetScore)
	require.NotNil(t, created.UserVote)
	require.Equal(t, model.VoteUp, *created.UserVote)

	toggled, err := service.Vote(context.Background(), userID, "message-1", model.VoteUp)
	require.NoError(t, err)
	require.Zero(t, toggled.NetScore)
	require.Nil(t, toggled.UserVote)

	_, err = service.Vote(context.Background(), userID, "message-1", model.VoteUp)
	require.NoError(t, err)
	flipped, err := service.Vote(context.Background(), userID, "message-1", model.VoteDown)
	require.NoError(t, err)
	require.Equal(t, int32(-1), flipped.NetScore)
	require.Equal(t, model.VoteDown, *flipped.UserVote)
}

func TestVoteServiceRejectsInvalidDirectionAndMissingMessage(t *testing.T) {
	userID := uuid.New()
	repo := &fakeVoteRepo{messageExists: false, votes: make(map[uuid.UUID]int16)}
	service := NewVoteService(repo, &FakeTxRunner{})

	_, err := service.Vote(context.Background(), userID, "message-1", 0)
	var validation *model.ValidationError
	require.ErrorAs(t, err, &validation)

	_, err = service.Vote(context.Background(), userID, "missing", model.VoteUp)
	var notFound *model.MessageNotFoundError
	require.ErrorAs(t, err, &notFound)
}

func TestVoteServiceRemoveIsIdempotent(t *testing.T) {
	userID := uuid.New()
	repo := &fakeVoteRepo{messageExists: true, votes: make(map[uuid.UUID]int16)}
	service := NewVoteService(repo, &FakeTxRunner{})

	require.NoError(t, service.RemoveVote(context.Background(), userID, "message-1"))
	require.NoError(t, service.RemoveVote(context.Background(), userID, "message-1"))
}
