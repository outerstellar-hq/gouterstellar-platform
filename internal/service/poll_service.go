package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/outerstellar-hq/gouterstellar-platform/internal/model"
	"github.com/outerstellar-hq/gouterstellar-platform/internal/persistence"
	"github.com/outerstellar-hq/gouterstellar-platform/internal/persistence/db"
)

const (
	minPollOptions       = 2
	maxPollOptions       = 10
	maxPollQuestionChars = 500
	maxPollOptionChars   = 200
)

type PollService struct {
	repo  persistence.PollRepository
	txMgr TransactionRunner
	now   func() time.Time
}

func NewPollService(repo persistence.PollRepository, txMgr TransactionRunner) *PollService {
	return &PollService{repo: repo, txMgr: txMgr, now: time.Now}
}

func (s *PollService) Create(ctx context.Context, input model.CreatePollInput, creatorID uuid.UUID) (*model.PollWithResults, error) {
	question, options, err := validatePoll(input.Question, input.Options)
	if err != nil {
		return nil, err
	}

	var result *model.PollWithResults
	err = s.txMgr.InTransaction(ctx, func(tx pgx.Tx) error {
		repo := s.repo.WithTx(tx)
		created, err := repo.CreatePoll(ctx, uuid.New().String(), creatorID, question, input.MultiChoice, input.Deadline)
		if err != nil {
			return fmt.Errorf("create poll: %w", err)
		}
		for position, option := range options {
			if _, err := repo.CreateOption(ctx, created.ID, int16(position), option); err != nil {
				return fmt.Errorf("create poll option: %w", err)
			}
		}
		result, err = pollResults(ctx, repo, created, &creatorID)
		return err
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func validatePoll(question string, options []string) (string, []string, error) {
	question = strings.TrimSpace(question)
	if question == "" {
		return "", nil, &model.ValidationError{Errors: []string{"Question must not be blank"}}
	}
	if len([]rune(question)) > maxPollQuestionChars {
		return "", nil, &model.ValidationError{Errors: []string{"Question must be at most 500 characters"}}
	}
	if len(options) < minPollOptions || len(options) > maxPollOptions {
		return "", nil, &model.ValidationError{Errors: []string{"Poll must have between 2 and 10 options"}}
	}

	normalized := make([]string, len(options))
	seen := make(map[string]struct{}, len(options))
	for i, option := range options {
		option = strings.TrimSpace(option)
		if option == "" {
			return "", nil, &model.ValidationError{Errors: []string{"Option text must not be blank"}}
		}
		if len([]rune(option)) > maxPollOptionChars {
			return "", nil, &model.ValidationError{Errors: []string{"Option text must be at most 200 characters"}}
		}
		key := strings.ToLower(option)
		if _, exists := seen[key]; exists {
			return "", nil, &model.ValidationError{Errors: []string{"Poll options must be unique"}}
		}
		seen[key] = struct{}{}
		normalized[i] = option
	}
	return question, normalized, nil
}

func (s *PollService) Get(ctx context.Context, syncID string, userID *uuid.UUID) (*model.PollWithResults, error) {
	poll, err := s.repo.FindBySyncID(ctx, syncID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, &model.PollNotFoundError{SyncID: syncID}
	}
	if err != nil {
		return nil, fmt.Errorf("find poll: %w", err)
	}
	return pollResults(ctx, s.repo, poll, userID)
}

func pollResults(ctx context.Context, repo persistence.PollRepository, poll db.PltPoll, userID *uuid.UUID) (*model.PollWithResults, error) {
	options, err := repo.ListOptions(ctx, poll.ID)
	if err != nil {
		return nil, fmt.Errorf("list poll options: %w", err)
	}
	counts, err := repo.ListVoteCounts(ctx, poll.ID)
	if err != nil {
		return nil, fmt.Errorf("list poll vote counts: %w", err)
	}
	userVotes := []int64{}
	if userID != nil {
		userVotes, err = repo.ListUserVotes(ctx, poll.ID, *userID)
		if err != nil {
			return nil, fmt.Errorf("list user poll votes: %w", err)
		}
	}

	resultOptions := make([]model.PollOption, len(options))
	for i, option := range options {
		resultOptions[i] = model.PollOption{ID: option.ID, PollID: option.PollID, Position: option.Position, OptionText: option.OptionText}
	}
	var total int32
	for _, count := range counts {
		total += count
	}
	return &model.PollWithResults{
		Poll: pollToModel(poll), Options: resultOptions, VoteCounts: counts,
		TotalVotes: total, UserVotedOptionIDs: userVotes,
	}, nil
}

func (s *PollService) CastVote(ctx context.Context, syncID string, optionID int64, userID uuid.UUID) (*model.PollWithResults, error) {
	var result *model.PollWithResults
	err := s.txMgr.InTransaction(ctx, func(tx pgx.Tx) error {
		repo := s.repo.WithTx(tx)
		// The poll-row lock makes the single-choice check and insert one atomic
		// transition across retries and instances. Its ceiling is one concurrent
		// mutation per poll, which is preferable to contradictory votes.
		poll, err := repo.LockBySyncID(ctx, syncID)
		if errors.Is(err, pgx.ErrNoRows) {
			return &model.PollNotFoundError{SyncID: syncID}
		}
		if err != nil {
			return fmt.Errorf("lock poll: %w", err)
		}
		if poll.ClosedAt.Valid {
			return &model.PollConflictError{Message: "Poll is closed"}
		}
		if poll.Deadline.Valid && s.now().After(poll.Deadline.Time) {
			return &model.PollConflictError{Message: "Poll deadline has passed"}
		}
		if _, err := repo.FindOption(ctx, poll.ID, optionID); errors.Is(err, pgx.ErrNoRows) {
			return &model.ValidationError{Errors: []string{"Option does not belong to this poll"}}
		} else if err != nil {
			return fmt.Errorf("find poll option: %w", err)
		}

		userVotes, err := repo.ListUserVotes(ctx, poll.ID, userID)
		if err != nil {
			return fmt.Errorf("list existing poll votes: %w", err)
		}
		if !poll.MultiChoice && len(userVotes) > 0 && !containsOption(userVotes, optionID) {
			return &model.PollConflictError{Message: "Already voted on a different option in this single-choice poll"}
		}
		if err := repo.CastVote(ctx, poll.ID, optionID, userID); err != nil {
			return fmt.Errorf("cast poll vote: %w", err)
		}
		result, err = pollResults(ctx, repo, poll, &userID)
		return err
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func containsOption(options []int64, optionID int64) bool {
	for _, existing := range options {
		if existing == optionID {
			return true
		}
	}
	return false
}

func (s *PollService) RemoveVote(ctx context.Context, syncID string, optionID int64, userID uuid.UUID) error {
	return s.txMgr.InTransaction(ctx, func(tx pgx.Tx) error {
		repo := s.repo.WithTx(tx)
		poll, err := repo.LockBySyncID(ctx, syncID)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("lock poll for vote removal: %w", err)
		}
		if err := repo.RemoveVote(ctx, poll.ID, optionID, userID); err != nil {
			return fmt.Errorf("remove poll vote: %w", err)
		}
		return nil
	})
}

func (s *PollService) Close(ctx context.Context, syncID string, creatorID uuid.UUID) error {
	return s.creatorMutation(ctx, syncID, creatorID, "close", func(repo persistence.PollRepository, poll db.PltPoll) error {
		return repo.Close(ctx, poll.ID)
	})
}

func (s *PollService) Delete(ctx context.Context, syncID string, creatorID uuid.UUID) error {
	return s.creatorMutation(ctx, syncID, creatorID, "delete", func(repo persistence.PollRepository, poll db.PltPoll) error {
		return repo.Delete(ctx, poll.ID)
	})
}

func (s *PollService) creatorMutation(ctx context.Context, syncID string, creatorID uuid.UUID, action string, mutate func(persistence.PollRepository, db.PltPoll) error) error {
	return s.txMgr.InTransaction(ctx, func(tx pgx.Tx) error {
		repo := s.repo.WithTx(tx)
		poll, err := repo.LockBySyncID(ctx, syncID)
		if errors.Is(err, pgx.ErrNoRows) {
			return &model.PollNotFoundError{SyncID: syncID}
		}
		if err != nil {
			return fmt.Errorf("lock poll to %s: %w", action, err)
		}
		if poll.CreatorID != creatorID {
			return &model.InsufficientPermissionError{Message: "Only the creator can " + action + " this poll"}
		}
		if err := mutate(repo, poll); err != nil {
			return fmt.Errorf("%s poll: %w", action, err)
		}
		return nil
	})
}

func (s *PollService) ListOpen(ctx context.Context, limit, offset int32) ([]model.PollSummary, error) {
	polls, err := s.repo.ListOpen(ctx, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("list open polls: %w", err)
	}
	summaries := make([]model.PollSummary, len(polls))
	for i, poll := range polls {
		summaries[i] = model.PollSummary{
			SyncID: poll.SyncID, Question: poll.Question, MultiChoice: poll.MultiChoice,
			Closed: poll.ClosedAt.Valid, Deadline: timestamptzPtr(poll.Deadline), TotalVotes: poll.TotalVotes,
		}
	}
	return summaries, nil
}

func pollToModel(poll db.PltPoll) model.Poll {
	return model.Poll{
		ID: poll.ID, SyncID: poll.SyncID, CreatorID: poll.CreatorID, Question: poll.Question,
		MultiChoice: poll.MultiChoice, ClosedAt: timestamptzPtr(poll.ClosedAt), Deadline: timestamptzPtr(poll.Deadline),
		CreatedAt: poll.CreatedAt, UpdatedAt: poll.UpdatedAt,
	}
}

func timestamptzPtr(value pgtype.Timestamptz) *time.Time {
	if !value.Valid {
		return nil
	}
	timestamp := value.Time
	return &timestamp
}
