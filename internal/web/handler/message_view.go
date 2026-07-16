package handler

import (
	"context"

	"github.com/google/uuid"

	"github.com/rygel/gouterstellar-platform/internal/model"
	"github.com/rygel/gouterstellar-platform/internal/web/viewmodel"
)

type messageVoteService interface {
	Vote(context.Context, uuid.UUID, string, model.VoteDirection) (*model.VoteScore, error)
	RemoveVote(context.Context, uuid.UUID, string) error
	GetScore(context.Context, string, *uuid.UUID) (*model.VoteScore, error)
	GetScores(context.Context, []string, *uuid.UUID) (map[string]model.VoteScore, error)
}

func buildMessageItems(
	ctx context.Context,
	messages []model.MessageSummary,
	voteService messageVoteService,
	userID uuid.UUID,
	csrfToken string,
	language string,
) ([]viewmodel.MessageItem, error) {
	syncIDs := make([]string, len(messages))
	for i, message := range messages {
		syncIDs[i] = message.SyncID
	}
	scores, err := voteService.GetScores(ctx, syncIDs, &userID)
	if err != nil {
		return nil, err
	}

	items := make([]viewmodel.MessageItem, len(messages))
	for i, message := range messages {
		score := scores[message.SyncID]
		items[i] = viewmodel.MessageItem{
			SyncID:       message.SyncID,
			Author:       message.Author,
			Content:      message.Content,
			UpdatedAt:    message.UpdatedAtLabel(),
			UpdatedLabel: message.UpdatedAtLabel(),
			Dirty:        message.Dirty,
			Version:      message.Version,
			HasConflict:  message.HasConflict,
			Upvotes:      score.Upvotes,
			Downvotes:    score.Downvotes,
			NetScore:     score.NetScore,
			HasUpvoted:   score.UserVote != nil && *score.UserVote == model.VoteUp,
			HasDownvoted: score.UserVote != nil && *score.UserVote == model.VoteDown,
			CSRFToken:    csrfToken,
			Language:     language,
		}
	}
	return items, nil
}
