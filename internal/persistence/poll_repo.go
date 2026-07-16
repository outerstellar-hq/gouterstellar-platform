package persistence

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/outerstellar-hq/gouterstellar-platform/internal/persistence/db"
)

type pollRepo struct {
	q *db.Queries
}

func NewPollRepository(pool *pgxpool.Pool) PollRepository {
	return &pollRepo{q: db.New(pool)}
}

func (r *pollRepo) WithTx(tx pgx.Tx) PollRepository {
	return &pollRepo{q: r.q.WithTx(tx)}
}

func (r *pollRepo) CreatePoll(ctx context.Context, syncID string, creatorID uuid.UUID, question string, multiChoice bool, deadline *time.Time) (db.PltPoll, error) {
	var deadlineValue pgtype.Timestamptz
	if deadline != nil {
		deadlineValue = pgtype.Timestamptz{Time: *deadline, Valid: true}
	}
	return r.q.CreatePoll(ctx, db.CreatePollParams{
		SyncID: syncID, CreatorID: creatorID, Question: question,
		MultiChoice: multiChoice, Deadline: deadlineValue,
	})
}

func (r *pollRepo) CreateOption(ctx context.Context, pollID int64, position int16, optionText string) (db.PltPollOption, error) {
	return r.q.CreatePollOption(ctx, db.CreatePollOptionParams{PollID: pollID, Position: position, OptionText: optionText})
}

func (r *pollRepo) FindBySyncID(ctx context.Context, syncID string) (db.PltPoll, error) {
	return r.q.FindPollBySyncID(ctx, syncID)
}

func (r *pollRepo) LockBySyncID(ctx context.Context, syncID string) (db.PltPoll, error) {
	return r.q.LockPollBySyncID(ctx, syncID)
}

func (r *pollRepo) ListOptions(ctx context.Context, pollID int64) ([]db.PltPollOption, error) {
	return r.q.ListPollOptions(ctx, pollID)
}

func (r *pollRepo) FindOption(ctx context.Context, pollID, optionID int64) (db.PltPollOption, error) {
	return r.q.FindPollOption(ctx, db.FindPollOptionParams{PollID: pollID, ID: optionID})
}

func (r *pollRepo) CastVote(ctx context.Context, pollID, optionID int64, userID uuid.UUID) error {
	_, err := r.q.CastPollVote(ctx, db.CastPollVoteParams{PollID: pollID, OptionID: optionID, UserID: userID})
	return err
}

func (r *pollRepo) RemoveVote(ctx context.Context, pollID, optionID int64, userID uuid.UUID) error {
	_, err := r.q.RemovePollVote(ctx, db.RemovePollVoteParams{PollID: pollID, OptionID: optionID, UserID: userID})
	return err
}

func (r *pollRepo) ListUserVotes(ctx context.Context, pollID int64, userID uuid.UUID) ([]int64, error) {
	return r.q.ListUserPollVotes(ctx, db.ListUserPollVotesParams{PollID: pollID, UserID: userID})
}

func (r *pollRepo) ListVoteCounts(ctx context.Context, pollID int64) (map[int64]int32, error) {
	rows, err := r.q.ListPollVoteCounts(ctx, pollID)
	if err != nil {
		return nil, err
	}
	counts := make(map[int64]int32, len(rows))
	for _, row := range rows {
		counts[row.OptionID] = row.VoteCount
	}
	return counts, nil
}

func (r *pollRepo) Close(ctx context.Context, pollID int64) error {
	rows, err := r.q.ClosePoll(ctx, pollID)
	if err != nil {
		return err
	}
	if rows != 1 {
		return fmt.Errorf("close poll: expected one row, updated %d", rows)
	}
	return nil
}

func (r *pollRepo) Delete(ctx context.Context, pollID int64) error {
	rows, err := r.q.DeletePoll(ctx, pollID)
	if err != nil {
		return err
	}
	if rows != 1 {
		return fmt.Errorf("delete poll: expected one row, deleted %d", rows)
	}
	return nil
}

func (r *pollRepo) ListOpen(ctx context.Context, limit, offset int32) ([]db.ListOpenPollsRow, error) {
	return r.q.ListOpenPolls(ctx, db.ListOpenPollsParams{Limit: limit, Offset: offset})
}
