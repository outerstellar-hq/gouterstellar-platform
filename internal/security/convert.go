package security

import (
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/rygel/gouterstellar-platform/internal/model"
	"github.com/rygel/gouterstellar-platform/internal/persistence/db"
)

func PltUserToModel(u db.PltUser) *model.User {
	return &model.User{
		ID:                        u.ID,
		Username:                  u.Username,
		Email:                     u.Email,
		PasswordHash:              u.PasswordHash,
		Role:                      model.UserRole(u.Role),
		Enabled:                   u.Enabled,
		LastActivityAt:            pgtypeTimestampToTimePtr(u.LastActivityAt),
		AvatarURL:                 u.AvatarUrl,
		EmailNotificationsEnabled: u.EmailNotificationsEnabled,
		PushNotificationsEnabled:  u.PushNotificationsEnabled,
		Language:                  u.Language,
		Theme:                     u.Theme,
		Layout:                    u.Layout,
	}
}

func pgtypeTimestampToTimePtr(t pgtype.Timestamp) *time.Time {
	if !t.Valid {
		return nil
	}
	return &t.Time
}

func pltApiKeySummaryToModel(a db.PltApiKey) model.ApiKeySummary {
	var lastUsed *string
	if a.LastUsedAt.Valid {
		s := a.LastUsedAt.Time.Format("2006-01-02T15:04:05Z")
		lastUsed = &s
	}
	return model.ApiKeySummary{
		ID:         a.ID,
		KeyPrefix:  a.KeyPrefix,
		Name:       a.Name,
		Enabled:    a.Enabled,
		CreatedAt:  a.CreatedAt.Time.Format("2006-01-02T15:04:05Z"),
		LastUsedAt: lastUsed,
	}
}
