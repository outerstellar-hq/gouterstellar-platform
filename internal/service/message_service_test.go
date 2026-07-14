package service

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/rygel/gouterstellar-platform/internal/model"
	"github.com/rygel/gouterstellar-platform/internal/persistence"
	"github.com/rygel/gouterstellar-platform/internal/persistence/db"
)

type mockMessageRepo struct {
	mock.Mock
}

func (m *mockMessageRepo) ListMessages(ctx context.Context, limit, offset int32) ([]db.PltMessage, error) {
	args := m.Called(ctx, limit, offset)
	return args.Get(0).([]db.PltMessage), args.Error(1)
}

func (m *mockMessageRepo) ListMessagesByYear(ctx context.Context, year int, limit, offset int32) ([]db.PltMessage, error) {
	args := m.Called(ctx, year, limit, offset)
	return args.Get(0).([]db.PltMessage), args.Error(1)
}

func (m *mockMessageRepo) CountMessagesByYear(ctx context.Context, year int) (int64, error) {
	args := m.Called(ctx, year)
	return args.Get(0).(int64), args.Error(1)
}

func (m *mockMessageRepo) ListMessageYears(ctx context.Context) ([]int32, error) {
	args := m.Called(ctx)
	return args.Get(0).([]int32), args.Error(1)
}

func (m *mockMessageRepo) SearchMessages(ctx context.Context, query string, limit, offset int32) ([]db.PltMessage, error) {
	args := m.Called(ctx, query, limit, offset)
	return args.Get(0).([]db.PltMessage), args.Error(1)
}

func (m *mockMessageRepo) CountSearchMessages(ctx context.Context, query string) (int64, error) {
	args := m.Called(ctx, query)
	return args.Get(0).(int64), args.Error(1)
}

func (m *mockMessageRepo) CountMessages(ctx context.Context) (int64, error) {
	args := m.Called(ctx)
	return args.Get(0).(int64), args.Error(1)
}

func (m *mockMessageRepo) FindBySyncID(ctx context.Context, syncID string) (db.PltMessage, error) {
	args := m.Called(ctx, syncID)
	return args.Get(0).(db.PltMessage), args.Error(1)
}

func (m *mockMessageRepo) CreateServerMessage(ctx context.Context, syncID, author, content string, updatedAtEpochMs int64) (db.PltMessage, error) {
	args := m.Called(ctx, syncID, author, content, updatedAtEpochMs)
	return args.Get(0).(db.PltMessage), args.Error(1)
}

func (m *mockMessageRepo) CreateLocalMessage(ctx context.Context, syncID, author, content string, updatedAtEpochMs int64) (db.PltMessage, error) {
	args := m.Called(ctx, syncID, author, content, updatedAtEpochMs)
	return args.Get(0).(db.PltMessage), args.Error(1)
}

func (m *mockMessageRepo) UpsertSyncedMessage(ctx context.Context, syncID, author, content string, updatedAtEpochMs int64, deleted bool) (db.PltMessage, error) {
	args := m.Called(ctx, syncID, author, content, updatedAtEpochMs, deleted)
	return args.Get(0).(db.PltMessage), args.Error(1)
}

func (m *mockMessageRepo) FindChangesSince(ctx context.Context, since int64) ([]db.PltMessage, error) {
	args := m.Called(ctx, since)
	return args.Get(0).([]db.PltMessage), args.Error(1)
}

func (m *mockMessageRepo) ListDirtyMessages(ctx context.Context) ([]db.PltMessage, error) {
	args := m.Called(ctx)
	return args.Get(0).([]db.PltMessage), args.Error(1)
}

func (m *mockMessageRepo) CountDirtyMessages(ctx context.Context) (int64, error) {
	args := m.Called(ctx)
	return args.Get(0).(int64), args.Error(1)
}

func (m *mockMessageRepo) SoftDeleteMessage(ctx context.Context, syncID string) (db.PltMessage, error) {
	args := m.Called(ctx, syncID)
	return args.Get(0).(db.PltMessage), args.Error(1)
}

func (m *mockMessageRepo) RestoreMessage(ctx context.Context, syncID string) (db.PltMessage, error) {
	args := m.Called(ctx, syncID)
	return args.Get(0).(db.PltMessage), args.Error(1)
}

func (m *mockMessageRepo) UpdateMessage(ctx context.Context, syncID, author, content string, updatedAtEpochMs int64, dirty bool, version int64) (db.PltMessage, error) {
	args := m.Called(ctx, syncID, author, content, updatedAtEpochMs, dirty, version)
	return args.Get(0).(db.PltMessage), args.Error(1)
}

func (m *mockMessageRepo) MarkConflictMessage(ctx context.Context, syncID string, conflict string) (db.PltMessage, error) {
	args := m.Called(ctx, syncID, conflict)
	return args.Get(0).(db.PltMessage), args.Error(1)
}

func (m *mockMessageRepo) ResolveConflictMessage(ctx context.Context, syncID string) (db.PltMessage, error) {
	args := m.Called(ctx, syncID)
	return args.Get(0).(db.PltMessage), args.Error(1)
}

func (m *mockMessageRepo) MarkCleanMessages(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *mockMessageRepo) WithTx(tx pgx.Tx) persistence.MessageRepository {
	args := m.Called(tx)
	if args.Get(0) == nil {
		return nil
	}
	return args.Get(0).(persistence.MessageRepository)
}

type mockOutboxRepo struct {
	mock.Mock
}

func (m *mockOutboxRepo) SaveOutbox(ctx context.Context, id uuid.UUID, payloadType, payload, status string) error {
	args := m.Called(ctx, id, payloadType, payload, status)
	return args.Error(0)
}

func (m *mockOutboxRepo) ListPending(ctx context.Context, limit int32) ([]db.ListPendingOutboxRow, error) {
	args := m.Called(ctx, limit)
	return args.Get(0).([]db.ListPendingOutboxRow), args.Error(1)
}

func (m *mockOutboxRepo) MarkProcessed(ctx context.Context, id uuid.UUID) (db.MarkOutboxProcessedRow, error) {
	args := m.Called(ctx, id)
	return args.Get(0).(db.MarkOutboxProcessedRow), args.Error(1)
}

func (m *mockOutboxRepo) MarkFailed(ctx context.Context, id uuid.UUID, lastError *string) (db.MarkOutboxFailedRow, error) {
	args := m.Called(ctx, id, lastError)
	return args.Get(0).(db.MarkOutboxFailedRow), args.Error(1)
}

func (m *mockOutboxRepo) GetStats(ctx context.Context) (db.GetOutboxStatsRow, error) {
	args := m.Called(ctx)
	return args.Get(0).(db.GetOutboxStatsRow), args.Error(1)
}

func (m *mockOutboxRepo) ListFailed(ctx context.Context, limit int32) ([]db.ListFailedOutboxRow, error) {
	args := m.Called(ctx, limit)
	return args.Get(0).([]db.ListFailedOutboxRow), args.Error(1)
}

func (m *mockOutboxRepo) ClaimPending(ctx context.Context, limit int32) ([]db.ClaimPendingOutboxRow, error) {
	args := m.Called(ctx, limit)
	return args.Get(0).([]db.ClaimPendingOutboxRow), args.Error(1)
}

func (m *mockOutboxRepo) UpdateStatus(ctx context.Context, id uuid.UUID, status string, lastError *string) (db.UpdateOutboxStatusRow, error) {
	args := m.Called(ctx, id, status, lastError)
	return args.Get(0).(db.UpdateOutboxStatusRow), args.Error(1)
}

func (m *mockOutboxRepo) ListDeadLetter(ctx context.Context, limit int32) ([]db.ListDeadLetterOutboxRow, error) {
	args := m.Called(ctx, limit)
	return args.Get(0).([]db.ListDeadLetterOutboxRow), args.Error(1)
}

func (m *mockOutboxRepo) WithTx(tx pgx.Tx) persistence.OutboxRepository {
	args := m.Called(tx)
	if args.Get(0) == nil {
		return nil
	}
	return args.Get(0).(persistence.OutboxRepository)
}

func (m *mockOutboxRepo) SaveOutboxTx(ctx context.Context, tx pgx.Tx, id uuid.UUID, payloadType, payload, status string) error {
	args := m.Called(ctx, tx, id, payloadType, payload, status)
	return args.Error(0)
}

type mockAuditRepo struct {
	mock.Mock
}

func (m *mockAuditRepo) LogAudit(ctx context.Context, actorID *uuid.UUID, actorUsername *string, targetID *uuid.UUID, targetUsername *string, action, detail string) (db.PltAuditLog, error) {
	args := m.Called(ctx, actorID, actorUsername, targetID, targetUsername, action, detail)
	return args.Get(0).(db.PltAuditLog), args.Error(1)
}

func (m *mockAuditRepo) FindRecent(ctx context.Context, limit int32) ([]db.PltAuditLog, error) {
	args := m.Called(ctx, limit)
	return args.Get(0).([]db.PltAuditLog), args.Error(1)
}

func (m *mockAuditRepo) FindPage(ctx context.Context, limit, offset int32) ([]db.PltAuditLog, error) {
	args := m.Called(ctx, limit, offset)
	return args.Get(0).([]db.PltAuditLog), args.Error(1)
}

func (m *mockAuditRepo) CountAll(ctx context.Context) (int64, error) {
	args := m.Called(ctx)
	return args.Get(0).(int64), args.Error(1)
}

func TestCreateServerMessage_BlankValidation(t *testing.T) {
	repo := new(mockMessageRepo)
	outbox := new(mockOutboxRepo)
	cache := persistence.NewMessageCache(60)
	svc := NewMessageService(repo, outbox, &FakeTxRunner{}, cache, &NoOpEventPublisher{}, nil, nil)

	_, err := svc.CreateServerMessage(context.Background(), "", "hello")
	assert.Error(t, err)
	assert.IsType(t, &model.ValidationError{}, err)
}

func TestCreateServerMessage_Success(t *testing.T) {
	repo := new(mockMessageRepo)
	outbox := new(mockOutboxRepo)
	cache := persistence.NewMessageCache(60)
	svc := NewMessageService(repo, outbox, &FakeTxRunner{}, cache, &NoOpEventPublisher{}, nil, nil)

	// WithTx returns the same mock so the tx-bound write uses the same stubs.
	repo.On("WithTx", mock.Anything).Return(repo)
	repo.On("CreateServerMessage", mock.Anything, mock.MatchedBy(func(s string) bool {
		return len(s) > 4 && s[:4] == "srv_"
	}), "alice", "hello world", mock.AnythingOfType("int64")).Return(db.PltMessage{
		SyncID:           "srv_test",
		Author:           "alice",
		Content:          "hello world",
		UpdatedAtEpochMs: 1000,
		Version:          1,
	}, nil)

	outbox.On("SaveOutboxTx", mock.Anything, mock.Anything, mock.AnythingOfType("uuid.UUID"), "MESSAGE_SYNC", mock.AnythingOfType("string"), "PENDING").Return(nil)

	msg, err := svc.CreateServerMessage(context.Background(), "alice", "hello world")

	assert.NoError(t, err)
	assert.Equal(t, "srv_test", msg.SyncID)
	assert.Equal(t, "alice", msg.Author)
	assert.Equal(t, "hello world", msg.Content)
	repo.AssertExpectations(t)
	outbox.AssertExpectations(t)
}
