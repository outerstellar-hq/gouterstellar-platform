package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/rygel/gouterstellar-platform/internal/model"
	"github.com/rygel/gouterstellar-platform/internal/persistence"
)

type VoteService struct {
	repo  persistence.VoteRepository
	txMgr TransactionRunner
}

func NewVoteService(repo persistence.VoteRepository, txMgr TransactionRunner) *VoteService {
	return &VoteService{repo: repo, txMgr: txMgr}
}

// Vote applies one atomic state transition: create, toggle off, or flip.
func (s *VoteService) Vote(ctx context.Context, userID uuid.UUID, syncID string, direction model.VoteDirection) (*model.VoteScore, error) {
	if !direction.Valid() {
		return nil, &model.ValidationError{Errors: []string{"direction must be 1 or -1"}}
	}

	err := s.txMgr.InTransaction(ctx, func(tx pgx.Tx) error {
		repo := s.repo.WithTx(tx)
		if err := repo.LockMessage(ctx, syncID); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return &model.MessageNotFoundError{SyncID: syncID}
			}
			return fmt.Errorf("lock message for vote: %w", err)
		}

		existing, err := repo.FindVote(ctx, userID, syncID)
		switch {
		case errors.Is(err, pgx.ErrNoRows):
			if err := repo.CreateVote(ctx, userID, syncID, int16(direction)); err != nil {
				return fmt.Errorf("create message vote: %w", err)
			}
		case err != nil:
			return fmt.Errorf("find message vote: %w", err)
		case existing.Direction == int16(direction):
			if err := repo.DeleteVote(ctx, userID, syncID); err != nil {
				return fmt.Errorf("toggle message vote: %w", err)
			}
		default:
			if err := repo.UpdateVote(ctx, userID, syncID, int16(direction)); err != nil {
				return fmt.Errorf("flip message vote: %w", err)
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return s.GetScore(ctx, syncID, &userID)
}

// RemoveVote is deliberately idempotent.
func (s *VoteService) RemoveVote(ctx context.Context, userID uuid.UUID, syncID string) error {
	if err := s.repo.DeleteVote(ctx, userID, syncID); err != nil {
		return fmt.Errorf("remove message vote: %w", err)
	}
	return nil
}

func (s *VoteService) GetScore(ctx context.Context, syncID string, userID *uuid.UUID) (*model.VoteScore, error) {
	scores, err := s.GetScores(ctx, []string{syncID}, userID)
	if err != nil {
		return nil, err
	}
	score := scores[syncID]
	return &score, nil
}

func (s *VoteService) GetScores(ctx context.Context, syncIDs []string, userID *uuid.UUID) (map[string]model.VoteScore, error) {
	scores, err := s.repo.ListScores(ctx, syncIDs, userID)
	if err != nil {
		return nil, fmt.Errorf("list message vote scores: %w", err)
	}
	return scores, nil
}
