package service

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/require"

	"github.com/rygel/gouterstellar-platform/internal/model"
	"github.com/rygel/gouterstellar-platform/internal/persistence"
	"github.com/rygel/gouterstellar-platform/internal/persistence/db"
)

type fakePollRepo struct {
	poll    db.PltPoll
	exists  bool
	options []db.PltPollOption
	votes   map[uuid.UUID]map[int64]struct{}
}

var _ persistence.PollRepository = (*fakePollRepo)(nil)

func (r *fakePollRepo) WithTx(pgx.Tx) persistence.PollRepository { return r }
func (r *fakePollRepo) CreatePoll(_ context.Context, syncID string, creatorID uuid.UUID, question string, multiChoice bool, deadline *time.Time) (db.PltPoll, error) {
	r.exists = true
	r.poll = db.PltPoll{ID: 1, SyncID: syncID, CreatorID: creatorID, Question: question, MultiChoice: multiChoice, CreatedAt: time.Now(), UpdatedAt: time.Now()}
	if deadline != nil {
		r.poll.Deadline = pgtype.Timestamptz{Time: *deadline, Valid: true}
	}
	return r.poll, nil
}
func (r *fakePollRepo) CreateOption(_ context.Context, pollID int64, position int16, text string) (db.PltPollOption, error) {
	option := db.PltPollOption{ID: int64(position) + 10, PollID: pollID, Position: position, OptionText: text}
	r.options = append(r.options, option)
	return option, nil
}
func (r *fakePollRepo) FindBySyncID(context.Context, string) (db.PltPoll, error) {
	if !r.exists {
		return db.PltPoll{}, pgx.ErrNoRows
	}
	return r.poll, nil
}
func (r *fakePollRepo) LockBySyncID(ctx context.Context, syncID string) (db.PltPoll, error) {
	return r.FindBySyncID(ctx, syncID)
}
func (r *fakePollRepo) ListOptions(context.Context, int64) ([]db.PltPollOption, error) {
	return r.options, nil
}
func (r *fakePollRepo) FindOption(_ context.Context, pollID, optionID int64) (db.PltPollOption, error) {
	for _, option := range r.options {
		if option.PollID == pollID && option.ID == optionID {
			return option, nil
		}
	}
	return db.PltPollOption{}, pgx.ErrNoRows
}
func (r *fakePollRepo) CastVote(_ context.Context, _ int64, optionID int64, userID uuid.UUID) error {
	if r.votes[userID] == nil {
		r.votes[userID] = make(map[int64]struct{})
	}
	r.votes[userID][optionID] = struct{}{}
	return nil
}
func (r *fakePollRepo) RemoveVote(_ context.Context, _ int64, optionID int64, userID uuid.UUID) error {
	delete(r.votes[userID], optionID)
	return nil
}
func (r *fakePollRepo) ListUserVotes(_ context.Context, _ int64, userID uuid.UUID) ([]int64, error) {
	result := make([]int64, 0, len(r.votes[userID]))
	for optionID := range r.votes[userID] {
		result = append(result, optionID)
	}
	return result, nil
}
func (r *fakePollRepo) ListVoteCounts(context.Context, int64) (map[int64]int32, error) {
	counts := make(map[int64]int32)
	for _, userVotes := range r.votes {
		for optionID := range userVotes {
			counts[optionID]++
		}
	}
	return counts, nil
}
func (r *fakePollRepo) Close(context.Context, int64) error {
	r.poll.ClosedAt = pgtype.Timestamptz{Time: time.Now(), Valid: true}
	return nil
}
func (r *fakePollRepo) Delete(context.Context, int64) error { r.exists = false; return nil }
func (r *fakePollRepo) ListOpen(context.Context, int32, int32) ([]db.ListOpenPollsRow, error) {
	if !r.exists {
		return []db.ListOpenPollsRow{}, nil
	}
	var totalVotes int32
	for _, votes := range r.votes {
		totalVotes += int32(len(votes))
	}
	return []db.ListOpenPollsRow{{
		ID: r.poll.ID, SyncID: r.poll.SyncID, CreatorID: r.poll.CreatorID, Question: r.poll.Question,
		MultiChoice: r.poll.MultiChoice, ClosedAt: r.poll.ClosedAt, Deadline: r.poll.Deadline,
		CreatedAt: r.poll.CreatedAt, UpdatedAt: r.poll.UpdatedAt, TotalVotes: totalVotes,
	}}, nil
}

func testPollRepo(creatorID uuid.UUID, multiChoice bool) *fakePollRepo {
	return &fakePollRepo{
		exists:  true,
		poll:    db.PltPoll{ID: 1, SyncID: "poll-1", CreatorID: creatorID, Question: "Choose a color", MultiChoice: multiChoice, CreatedAt: time.Now(), UpdatedAt: time.Now()},
		options: []db.PltPollOption{{ID: 10, PollID: 1, Position: 0, OptionText: "Red"}, {ID: 11, PollID: 1, Position: 1, OptionText: "Blue"}},
		votes:   make(map[uuid.UUID]map[int64]struct{}),
	}
}

func TestPollServiceCreatesValidatedPollWithOptions(t *testing.T) {
	creatorID := uuid.New()
	repo := &fakePollRepo{votes: make(map[uuid.UUID]map[int64]struct{})}
	service := NewPollService(repo, &FakeTxRunner{})

	result, err := service.Create(context.Background(), model.CreatePollInput{Question: "  Choose? ", Options: []string{" A ", "B"}}, creatorID)
	require.NoError(t, err)
	require.Equal(t, "Choose?", result.Poll.Question)
	require.Equal(t, []string{"A", "B"}, []string{result.Options[0].OptionText, result.Options[1].OptionText})

	_, err = service.Create(context.Background(), model.CreatePollInput{Question: "Choose?", Options: []string{"only one"}}, creatorID)
	var validation *model.ValidationError
	require.ErrorAs(t, err, &validation)

	_, err = service.Create(context.Background(), model.CreatePollInput{Question: "Choose?", Options: []string{"Same", " same "}}, creatorID)
	require.ErrorAs(t, err, &validation)
}

func TestPollServiceSingleChoiceIsIdempotentAndRejectsDifferentOption(t *testing.T) {
	creatorID, userID := uuid.New(), uuid.New()
	repo := testPollRepo(creatorID, false)
	service := NewPollService(repo, &FakeTxRunner{})

	first, err := service.CastVote(context.Background(), "poll-1", 10, userID)
	require.NoError(t, err)
	require.Equal(t, int32(1), first.TotalVotes)

	repeated, err := service.CastVote(context.Background(), "poll-1", 10, userID)
	require.NoError(t, err)
	require.Equal(t, int32(1), repeated.TotalVotes)

	_, err = service.CastVote(context.Background(), "poll-1", 11, userID)
	var conflict *model.PollConflictError
	require.ErrorAs(t, err, &conflict)
}

func TestPollServiceMultiChoiceAcceptsMultipleOptions(t *testing.T) {
	creatorID, userID := uuid.New(), uuid.New()
	repo := testPollRepo(creatorID, true)
	service := NewPollService(repo, &FakeTxRunner{})

	_, err := service.CastVote(context.Background(), "poll-1", 10, userID)
	require.NoError(t, err)
	result, err := service.CastVote(context.Background(), "poll-1", 11, userID)
	require.NoError(t, err)
	require.Equal(t, int32(2), result.TotalVotes)
	require.ElementsMatch(t, []int64{10, 11}, result.UserVotedOptionIDs)
}

func TestPollServiceRejectsClosedExpiredAndForeignOptions(t *testing.T) {
	creatorID, userID := uuid.New(), uuid.New()
	repo := testPollRepo(creatorID, false)
	service := NewPollService(repo, &FakeTxRunner{})

	repo.poll.ClosedAt = pgtype.Timestamptz{Time: time.Now(), Valid: true}
	_, err := service.CastVote(context.Background(), "poll-1", 10, userID)
	var conflict *model.PollConflictError
	require.ErrorAs(t, err, &conflict)

	repo.poll.ClosedAt = pgtype.Timestamptz{}
	repo.poll.Deadline = pgtype.Timestamptz{Time: time.Now().Add(-time.Minute), Valid: true}
	_, err = service.CastVote(context.Background(), "poll-1", 10, userID)
	require.ErrorAs(t, err, &conflict)

	repo.poll.Deadline = pgtype.Timestamptz{}
	_, err = service.CastVote(context.Background(), "poll-1", 99, userID)
	var validation *model.ValidationError
	require.ErrorAs(t, err, &validation)
}

func TestPollServiceCreatorControlsLifecycle(t *testing.T) {
	creatorID := uuid.New()
	repo := testPollRepo(creatorID, false)
	service := NewPollService(repo, &FakeTxRunner{})

	err := service.Close(context.Background(), "poll-1", uuid.New())
	var permission *model.InsufficientPermissionError
	require.ErrorAs(t, err, &permission)

	require.NoError(t, service.Close(context.Background(), "poll-1", creatorID))
	require.True(t, repo.poll.ClosedAt.Valid)
	require.NoError(t, service.Delete(context.Background(), "poll-1", creatorID))
	require.False(t, repo.exists)
}

func TestPollServiceOpenListIncludesVoteTotals(t *testing.T) {
	creatorID, userID := uuid.New(), uuid.New()
	repo := testPollRepo(creatorID, false)
	repo.votes[userID] = map[int64]struct{}{10: {}}
	service := NewPollService(repo, &FakeTxRunner{})

	polls, err := service.ListOpen(context.Background(), 20, 0)
	require.NoError(t, err)
	require.Len(t, polls, 1)
	require.Equal(t, int32(1), polls[0].TotalVotes)
}
