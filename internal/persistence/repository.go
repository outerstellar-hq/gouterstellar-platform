package persistence

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/rygel/gouterstellar-platform/internal/model"
	"github.com/rygel/gouterstellar-platform/internal/persistence/db"
)

type MessageRepository interface {
	ListMessages(ctx context.Context, limit, offset int32) ([]db.PltMessage, error)
	ListMessagesByYear(ctx context.Context, year int, limit, offset int32) ([]db.PltMessage, error)
	CountMessagesByYear(ctx context.Context, year int) (int64, error)
	ListMessageYears(ctx context.Context) ([]int32, error)
	SearchMessages(ctx context.Context, query string, limit, offset int32) ([]db.PltMessage, error)
	CountSearchMessages(ctx context.Context, query string) (int64, error)
	CountMessages(ctx context.Context) (int64, error)
	ListDeletedMessages(ctx context.Context, limit, offset int32) ([]db.PltMessage, error)
	CountDeletedMessages(ctx context.Context) (int64, error)
	FindBySyncID(ctx context.Context, syncID string) (db.PltMessage, error)
	CreateServerMessage(ctx context.Context, syncID, author, content string, updatedAtEpochMs int64) (db.PltMessage, error)
	CreateLocalMessage(ctx context.Context, syncID, author, content string, updatedAtEpochMs int64) (db.PltMessage, error)
	UpsertSyncedMessage(ctx context.Context, syncID, author, content string, updatedAtEpochMs int64, deleted bool) (db.PltMessage, error)
	FindChangesSince(ctx context.Context, since int64) ([]db.PltMessage, error)
	ListDirtyMessages(ctx context.Context) ([]db.PltMessage, error)
	CountDirtyMessages(ctx context.Context) (int64, error)
	SoftDeleteMessage(ctx context.Context, syncID string) (db.PltMessage, error)
	RestoreMessage(ctx context.Context, syncID string) (db.PltMessage, error)
	UpdateMessage(ctx context.Context, syncID, author, content string, updatedAtEpochMs int64, dirty bool, version int64) (db.PltMessage, error)
	MarkConflictMessage(ctx context.Context, syncID string, conflict string) (db.PltMessage, error)
	ResolveConflictMessage(ctx context.Context, syncID string) (db.PltMessage, error)
	MarkCleanMessages(ctx context.Context) error
	// WithTx returns a copy of this repository bound to the given transaction.
	// Calls on the returned repository participate in the transaction and only
	// commit when the transaction commits.
	WithTx(tx pgx.Tx) MessageRepository
}

type VoteRepository interface {
	LockMessage(ctx context.Context, syncID string) error
	FindVote(ctx context.Context, userID uuid.UUID, syncID string) (db.PltMessageVote, error)
	CreateVote(ctx context.Context, userID uuid.UUID, syncID string, direction int16) error
	UpdateVote(ctx context.Context, userID uuid.UUID, syncID string, direction int16) error
	DeleteVote(ctx context.Context, userID uuid.UUID, syncID string) error
	ListScores(ctx context.Context, syncIDs []string, userID *uuid.UUID) (map[string]model.VoteScore, error)
	WithTx(tx pgx.Tx) VoteRepository
}

type PollRepository interface {
	CreatePoll(ctx context.Context, syncID string, creatorID uuid.UUID, question string, multiChoice bool, deadline *time.Time) (db.PltPoll, error)
	CreateOption(ctx context.Context, pollID int64, position int16, optionText string) (db.PltPollOption, error)
	FindBySyncID(ctx context.Context, syncID string) (db.PltPoll, error)
	LockBySyncID(ctx context.Context, syncID string) (db.PltPoll, error)
	ListOptions(ctx context.Context, pollID int64) ([]db.PltPollOption, error)
	FindOption(ctx context.Context, pollID, optionID int64) (db.PltPollOption, error)
	CastVote(ctx context.Context, pollID, optionID int64, userID uuid.UUID) error
	RemoveVote(ctx context.Context, pollID, optionID int64, userID uuid.UUID) error
	ListUserVotes(ctx context.Context, pollID int64, userID uuid.UUID) ([]int64, error)
	ListVoteCounts(ctx context.Context, pollID int64) (map[int64]int32, error)
	Close(ctx context.Context, pollID int64) error
	Delete(ctx context.Context, pollID int64) error
	ListOpen(ctx context.Context, limit, offset int32) ([]db.ListOpenPollsRow, error)
	WithTx(tx pgx.Tx) PollRepository
}

type ContactRepository interface {
	ListContacts(ctx context.Context, limit, offset int32) ([]db.PltContact, error)
	SearchContacts(ctx context.Context, query string, limit, offset int32) ([]db.PltContact, error)
	CountSearchContacts(ctx context.Context, query string) (int64, error)
	CountContacts(ctx context.Context) (int64, error)
	ListDeletedContacts(ctx context.Context, limit, offset int32) ([]db.PltContact, error)
	CountDeletedContacts(ctx context.Context) (int64, error)
	ListDirtyContacts(ctx context.Context) ([]db.PltContact, error)
	FindBySyncID(ctx context.Context, syncID string) (db.PltContact, error)
	FindChangesSince(ctx context.Context, since int64) ([]db.PltContact, error)
	CreateServerContact(ctx context.Context, contact *model.StoredContact) (db.PltContact, error)
	CreateLocalContact(ctx context.Context, contact *model.StoredContact) (db.PltContact, error)
	UpsertSyncedContact(ctx context.Context, contact *model.SyncContact) (db.PltContact, error)
	SoftDeleteContact(ctx context.Context, syncID string) (db.PltContact, error)
	RestoreContact(ctx context.Context, syncID string) (db.PltContact, error)
	UpdateContact(ctx context.Context, syncID string, contact *model.StoredContact, version int64) (db.PltContact, error)
	MarkConflictContact(ctx context.Context, syncID string, conflict string) (db.PltContact, error)
	ResolveConflictContact(ctx context.Context, syncID string) (db.PltContact, error)
	MarkCleanContacts(ctx context.Context) error
	// WithTx returns a copy of this repository bound to the given transaction.
	// Calls on the returned repository participate in the transaction and only
	// commit when the transaction commits.
	WithTx(tx pgx.Tx) ContactRepository
	ListContactEmails(ctx context.Context, contactID int64) ([]string, error)
	SetContactEmails(ctx context.Context, contactID int64, emails []string) error
	ListContactPhones(ctx context.Context, contactID int64) ([]string, error)
	SetContactPhones(ctx context.Context, contactID int64, phones []string) error
	ListContactSocials(ctx context.Context, contactID int64) ([]string, error)
	SetContactSocials(ctx context.Context, contactID int64, socials []string) error
	// LoadSubTablesBatch fetches emails/phones/socials for all given contact IDs
	// in three queries (one per sub-table), returning a map keyed by contact ID.
	// It replaces the previous N+1 pattern of looping ListContactEmails/Phones/
	// Socials per row.
	LoadSubTablesBatch(ctx context.Context, contactIDs []int64) (ContactSubTables, error)
}

// ContactSubTables holds the emails/phones/socials for a set of contacts, keyed
// by contact database ID. Returned by LoadSubTablesBatch.
type ContactSubTables struct {
	Emails  map[int64][]string
	Phones  map[int64][]string
	Socials map[int64][]string
}

type UserRepository interface {
	FindByID(ctx context.Context, id uuid.UUID) (db.PltUser, error)
	FindByUsername(ctx context.Context, username string) (db.PltUser, error)
	FindByEmail(ctx context.Context, email string) (db.PltUser, error)
	CreateUser(ctx context.Context, id uuid.UUID, username, email, passwordHash string, role string, enabled bool) (db.PltUser, error)
	FindAll(ctx context.Context) ([]db.PltUser, error)
	FindPage(ctx context.Context, limit, offset int32) ([]db.PltUser, error)
	CountAll(ctx context.Context) (int64, error)
	CountByRole(ctx context.Context, role string) (int64, error)
	UpdateRole(ctx context.Context, id uuid.UUID, role string) (db.PltUser, error)
	UpdateEnabled(ctx context.Context, id uuid.UUID, enabled bool) (db.PltUser, error)
	UpdatePasswordHash(ctx context.Context, userID uuid.UUID, passwordHash string) error
	UpdateLastActivity(ctx context.Context, id uuid.UUID) error
	DeleteByID(ctx context.Context, id uuid.UUID) error
	UpdateUsername(ctx context.Context, id uuid.UUID, username string) (db.PltUser, error)
	UpdateEmail(ctx context.Context, id uuid.UUID, email string) (db.PltUser, error)
	UpdateAvatarURL(ctx context.Context, id uuid.UUID, avatarURL *string) (db.PltUser, error)
	UpdateNotificationPreferences(ctx context.Context, id uuid.UUID, emailEnabled, pushEnabled bool) (db.PltUser, error)
	UpdatePreferences(ctx context.Context, id uuid.UUID, language, theme, layout *string) (db.PltUser, error)
	SeedAdminUser(ctx context.Context, id uuid.UUID, username, email, passwordHash string) (db.PltUser, error)
	IncrementFailedLoginAttempts(ctx context.Context, id uuid.UUID) (int32, error)
	ResetLoginFailures(ctx context.Context, id uuid.UUID) error
	LockUserUntil(ctx context.Context, id uuid.UUID, until time.Time) error
}

type SessionRepository interface {
	CreateSession(ctx context.Context, tokenHash string, userID uuid.UUID, expiresAt time.Time) (db.PltSession, error)
	FindByTokenHash(ctx context.Context, tokenHash string) (db.PltSession, error)
	FindByTokenHashIncludingExpired(ctx context.Context, tokenHash string) (db.PltSession, error)
	UpdateExpiresAt(ctx context.Context, tokenHash string, expiresAt time.Time) (db.PltSession, error)
	DeleteByTokenHash(ctx context.Context, tokenHash string) error
	DeleteByUserID(ctx context.Context, userID uuid.UUID) error
	DeleteExpired(ctx context.Context) (int64, error)
	ListForUser(ctx context.Context, userID uuid.UUID) ([]db.ListSessionsForUserRow, error)
}

type TOTPRepository interface {
	CreateChallenge(ctx context.Context, tokenHash string, userID uuid.UUID, expiresAt time.Time) error
	TakeChallengeAttempt(ctx context.Context, tokenHash string, maxAttempts int32) (db.PltTotpChallenge, error)
	DeleteChallenge(ctx context.Context, tokenHash string) (bool, error)
	DeleteExpiredChallenges(ctx context.Context) (int64, error)
	Enable(ctx context.Context, userID uuid.UUID, secret, backupCodes string) error
	Disable(ctx context.Context, userID uuid.UUID) error
	IncrementFailedAttempts(ctx context.Context, userID uuid.UUID) (int32, error)
	ResetFailedAttempts(ctx context.Context, userID uuid.UUID) error
	ReplaceBackupCodes(ctx context.Context, userID uuid.UUID, expected string, replacement *string) (bool, error)
}

type ApiKeyRepository interface {
	CreateApiKey(ctx context.Context, userID uuid.UUID, keyHash, keyPrefix, name string) (db.PltApiKey, error)
	FindByKeyHash(ctx context.Context, keyHash string) (db.PltApiKey, error)
	FindByUserID(ctx context.Context, userID uuid.UUID) ([]db.PltApiKey, error)
	DeleteApiKey(ctx context.Context, id int64, userID uuid.UUID) (int64, error)
	UpdateLastUsed(ctx context.Context, id int64) error
}

type OutboxRepository interface {
	ListPending(ctx context.Context, limit int32) ([]db.ListPendingOutboxRow, error)
	MarkProcessed(ctx context.Context, id uuid.UUID) (db.MarkOutboxProcessedRow, error)
	MarkFailed(ctx context.Context, id uuid.UUID, lastError *string) (db.MarkOutboxFailedRow, error)
	GetStats(ctx context.Context) (db.GetOutboxStatsRow, error)
	ListFailed(ctx context.Context, limit int32) ([]db.ListFailedOutboxRow, error)
	ClaimPending(ctx context.Context, limit int32) ([]db.ClaimPendingOutboxRow, error)
	UpdateStatus(ctx context.Context, id uuid.UUID, status string, lastError *string) (db.UpdateOutboxStatusRow, error)
	ListDeadLetter(ctx context.Context, limit int32) ([]db.ListDeadLetterOutboxRow, error)
	// WithTx returns a copy of this repository bound to the given transaction.
	// Calls on the returned repository participate in the transaction and only
	// commit when the transaction is committed by TransactionManager.InTransaction.
	WithTx(tx pgx.Tx) OutboxRepository
	// SaveOutboxTx inserts an outbox entry using an existing transaction so the
	// insert participates in the caller's transaction. This is the transactional
	// counterpart of SaveOutbox.
	SaveOutboxTx(ctx context.Context, tx pgx.Tx, id uuid.UUID, payloadType, payload, status string) error
}

type AuditRepository interface {
	LogAudit(ctx context.Context, actorID *uuid.UUID, actorUsername *string, targetID *uuid.UUID, targetUsername *string, action, detail string) (db.PltAuditLog, error)
	FindRecent(ctx context.Context, limit int32) ([]db.PltAuditLog, error)
	FindPage(ctx context.Context, limit, offset int32) ([]db.PltAuditLog, error)
	CountAll(ctx context.Context) (int64, error)
}

type NotificationRepository interface {
	SaveNotification(ctx context.Context, id, userID uuid.UUID, title, body, notificationType string) (db.PltNotification, error)
	FindByUserID(ctx context.Context, userID uuid.UUID, limit, offset int32) ([]db.PltNotification, error)
	CountUnread(ctx context.Context, userID uuid.UUID) (int64, error)
	MarkRead(ctx context.Context, id, userID uuid.UUID) (db.PltNotification, error)
	MarkAllRead(ctx context.Context, userID uuid.UUID) (int64, error)
	DeleteNotification(ctx context.Context, id, userID uuid.UUID) (int64, error)
}

type DeviceTokenRepository interface {
	UpsertDeviceToken(ctx context.Context, userID uuid.UUID, platform, token string, appBundle *string) (db.PltDeviceToken, error)
	DeleteDeviceToken(ctx context.Context, id int64, userID uuid.UUID) (int64, error)
	DeleteDeviceTokenByValue(ctx context.Context, token string, userID uuid.UUID) (int64, error)
	FindByUserID(ctx context.Context, userID uuid.UUID) ([]db.PltDeviceToken, error)
	DeleteAllForUser(ctx context.Context, userID uuid.UUID) (int64, error)
}

type OAuthRepository interface {
	FindByProviderSubject(ctx context.Context, provider, subject string) (db.PltOauthConnection, error)
	SaveOAuthConnection(ctx context.Context, userID uuid.UUID, provider, subject string, email *string) (db.PltOauthConnection, error)
	FindByUserID(ctx context.Context, userID uuid.UUID) ([]db.PltOauthConnection, error)
	DeleteOAuthConnection(ctx context.Context, id int64, userID uuid.UUID) (int64, error)
}

type PasswordResetRepository interface {
	SavePasswordResetToken(ctx context.Context, userID uuid.UUID, token string, expiresAt time.Time) (db.PltPasswordResetToken, error)
	FindByToken(ctx context.Context, token string) (db.PltPasswordResetToken, error)
	MarkUsed(ctx context.Context, token string) (db.PltPasswordResetToken, error)
}
