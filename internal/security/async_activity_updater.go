package security

import (
	"context"
	"log/slog"
	"sync"

	"github.com/google/uuid"

	"github.com/rygel/gouterstellar-platform/internal/persistence"
)

type AsyncActivityUpdater struct {
	userRepo persistence.UserRepository
	pending  sync.Map
}

func NewAsyncActivityUpdater(userRepo persistence.UserRepository) *AsyncActivityUpdater {
	return &AsyncActivityUpdater{
		userRepo: userRepo,
	}
}

func (u *AsyncActivityUpdater) Record(userID uuid.UUID) {
	u.pending.Store(userID, struct{}{})
}

func (u *AsyncActivityUpdater) Flush() {
	var ids []uuid.UUID
	u.pending.Range(func(key, _ interface{}) bool {
		if id, ok := key.(uuid.UUID); ok {
			ids = append(ids, id)
		}
		return true
	})

	for _, id := range ids {
		u.pending.Delete(id)
	}

	if len(ids) == 0 {
		return
	}

	ctx := context.Background()
	for _, id := range ids {
		if err := u.userRepo.UpdateLastActivity(ctx, id); err != nil {
			slog.Warn("Failed to update last activity", "userID", id, "error", err)
		}
	}
}
