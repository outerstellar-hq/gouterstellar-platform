package persistence

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/outerstellar-hq/gouterstellar-platform/internal/model"
	"github.com/outerstellar-hq/gouterstellar-platform/internal/persistence/db"
)

type voteRepo struct {
	q *db.Queries
}

func NewVoteRepository(pool *pgxpool.Pool) VoteRepository {
	return &voteRepo{q: db.New(pool)}
}

func (r *voteRepo) WithTx(tx pgx.Tx) VoteRepository {
	return &voteRepo{q: r.q.WithTx(tx)}
}

func (r *voteRepo) LockMessage(ctx context.Context, syncID string) error {
	_, err := r.q.LockMessageForVote(ctx, syncID)
	return err
}

func (r *voteRepo) FindVote(ctx context.Context, userID uuid.UUID, syncID string) (db.PltMessageVote, error) {
	return r.q.FindMessageVote(ctx, db.FindMessageVoteParams{UserID: userID, MessageSyncID: syncID})
}

func (r *voteRepo) CreateVote(ctx context.Context, userID uuid.UUID, syncID string, direction int16) error {
	_, err := r.q.CreateMessageVote(ctx, db.CreateMessageVoteParams{UserID: userID, MessageSyncID: syncID, Direction: direction})
	return err
}

func (r *voteRepo) UpdateVote(ctx context.Context, userID uuid.UUID, syncID string, direction int16) error {
	rows, err := r.q.UpdateMessageVote(ctx, db.UpdateMessageVoteParams{UserID: userID, MessageSyncID: syncID, Direction: direction})
	if err != nil {
		return err
	}
	if rows != 1 {
		return fmt.Errorf("update message vote: expected one row, updated %d", rows)
	}
	return nil
}

func (r *voteRepo) DeleteVote(ctx context.Context, userID uuid.UUID, syncID string) error {
	_, err := r.q.DeleteMessageVote(ctx, db.DeleteMessageVoteParams{UserID: userID, MessageSyncID: syncID})
	return err
}

func (r *voteRepo) ListScores(ctx context.Context, syncIDs []string, userID *uuid.UUID) (map[string]model.VoteScore, error) {
	scores := make(map[string]model.VoteScore, len(syncIDs))
	for _, syncID := range syncIDs {
		scores[syncID] = model.VoteScore{MessageSyncID: syncID}
	}
	if len(syncIDs) == 0 {
		return scores, nil
	}

	counts, err := r.q.ListMessageVoteCounts(ctx, syncIDs)
	if err != nil {
		return nil, err
	}
	for _, count := range counts {
		score := scores[count.MessageSyncID]
		score.Upvotes = count.Upvotes
		score.Downvotes = count.Downvotes
		score.NetScore = count.Upvotes - count.Downvotes
		scores[count.MessageSyncID] = score
	}

	if userID == nil {
		return scores, nil
	}
	votes, err := r.q.ListUserMessageVotes(ctx, db.ListUserMessageVotesParams{UserID: *userID, MessageSyncIds: syncIDs})
	if err != nil {
		return nil, err
	}
	for _, vote := range votes {
		direction := model.VoteDirection(vote.Direction)
		score := scores[vote.MessageSyncID]
		score.UserVote = &direction
		scores[vote.MessageSyncID] = score
	}
	return scores, nil
}
