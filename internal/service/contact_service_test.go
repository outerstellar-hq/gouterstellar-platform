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

type mockContactRepo struct {
	mock.Mock
}

func (m *mockContactRepo) ListContacts(ctx context.Context, limit, offset int32) ([]db.PltContact, error) {
	args := m.Called(ctx, limit, offset)
	return args.Get(0).([]db.PltContact), args.Error(1)
}

func (m *mockContactRepo) SearchContacts(ctx context.Context, query string, limit, offset int32) ([]db.PltContact, error) {
	args := m.Called(ctx, query, limit, offset)
	return args.Get(0).([]db.PltContact), args.Error(1)
}

func (m *mockContactRepo) CountSearchContacts(ctx context.Context, query string) (int64, error) {
	args := m.Called(ctx, query)
	return args.Get(0).(int64), args.Error(1)
}

func (m *mockContactRepo) CountContacts(ctx context.Context) (int64, error) {
	args := m.Called(ctx)
	return args.Get(0).(int64), args.Error(1)
}

func (m *mockContactRepo) ListDirtyContacts(ctx context.Context) ([]db.PltContact, error) {
	args := m.Called(ctx)
	return args.Get(0).([]db.PltContact), args.Error(1)
}

func (m *mockContactRepo) FindBySyncID(ctx context.Context, syncID string) (db.PltContact, error) {
	args := m.Called(ctx, syncID)
	return args.Get(0).(db.PltContact), args.Error(1)
}

func (m *mockContactRepo) FindChangesSince(ctx context.Context, since int64) ([]db.PltContact, error) {
	args := m.Called(ctx, since)
	return args.Get(0).([]db.PltContact), args.Error(1)
}

func (m *mockContactRepo) CreateServerContact(ctx context.Context, contact *model.StoredContact) (db.PltContact, error) {
	args := m.Called(ctx, contact)
	return args.Get(0).(db.PltContact), args.Error(1)
}

func (m *mockContactRepo) CreateLocalContact(ctx context.Context, contact *model.StoredContact) (db.PltContact, error) {
	args := m.Called(ctx, contact)
	return args.Get(0).(db.PltContact), args.Error(1)
}

func (m *mockContactRepo) UpsertSyncedContact(ctx context.Context, contact *model.SyncContact) (db.PltContact, error) {
	args := m.Called(ctx, contact)
	return args.Get(0).(db.PltContact), args.Error(1)
}

func (m *mockContactRepo) SoftDeleteContact(ctx context.Context, syncID string) (db.PltContact, error) {
	args := m.Called(ctx, syncID)
	return args.Get(0).(db.PltContact), args.Error(1)
}

func (m *mockContactRepo) RestoreContact(ctx context.Context, syncID string) (db.PltContact, error) {
	args := m.Called(ctx, syncID)
	return args.Get(0).(db.PltContact), args.Error(1)
}

func (m *mockContactRepo) UpdateContact(ctx context.Context, syncID string, contact *model.StoredContact, version int64) (db.PltContact, error) {
	args := m.Called(ctx, syncID, contact, version)
	return args.Get(0).(db.PltContact), args.Error(1)
}

func (m *mockContactRepo) MarkConflictContact(ctx context.Context, syncID string, conflict string) (db.PltContact, error) {
	args := m.Called(ctx, syncID, conflict)
	return args.Get(0).(db.PltContact), args.Error(1)
}

func (m *mockContactRepo) ResolveConflictContact(ctx context.Context, syncID string) (db.PltContact, error) {
	args := m.Called(ctx, syncID)
	return args.Get(0).(db.PltContact), args.Error(1)
}

func (m *mockContactRepo) MarkCleanContacts(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *mockContactRepo) WithTx(tx pgx.Tx) persistence.ContactRepository {
	args := m.Called(tx)
	if args.Get(0) == nil {
		return nil
	}
	return args.Get(0).(persistence.ContactRepository)
}

func (m *mockContactRepo) ListContactEmails(ctx context.Context, contactID int64) ([]string, error) {
	args := m.Called(ctx, contactID)
	if args.Get(0) == nil {
		return []string{}, args.Error(1)
	}
	return args.Get(0).([]string), args.Error(1)
}

func (m *mockContactRepo) SetContactEmails(ctx context.Context, contactID int64, emails []string) error {
	args := m.Called(ctx, contactID, emails)
	return args.Error(0)
}

func (m *mockContactRepo) ListContactPhones(ctx context.Context, contactID int64) ([]string, error) {
	args := m.Called(ctx, contactID)
	if args.Get(0) == nil {
		return []string{}, args.Error(1)
	}
	return args.Get(0).([]string), args.Error(1)
}

func (m *mockContactRepo) SetContactPhones(ctx context.Context, contactID int64, phones []string) error {
	args := m.Called(ctx, contactID, phones)
	return args.Error(0)
}

func (m *mockContactRepo) ListContactSocials(ctx context.Context, contactID int64) ([]string, error) {
	args := m.Called(ctx, contactID)
	if args.Get(0) == nil {
		return []string{}, args.Error(1)
	}
	return args.Get(0).([]string), args.Error(1)
}

func (m *mockContactRepo) SetContactSocials(ctx context.Context, contactID int64, socials []string) error {
	args := m.Called(ctx, contactID, socials)
	return args.Error(0)
}

func (m *mockContactRepo) LoadSubTablesBatch(ctx context.Context, contactIDs []int64) (persistence.ContactSubTables, error) {
	args := m.Called(ctx, contactIDs)
	if args.Get(0) == nil {
		return persistence.ContactSubTables{}, args.Error(1)
	}
	return args.Get(0).(persistence.ContactSubTables), args.Error(1)
}

type mockContactOutboxRepo struct {
	mock.Mock
}

func (m *mockContactOutboxRepo) ListPending(ctx context.Context, limit int32) ([]db.ListPendingOutboxRow, error) {
	args := m.Called(ctx, limit)
	return args.Get(0).([]db.ListPendingOutboxRow), args.Error(1)
}

func (m *mockContactOutboxRepo) MarkProcessed(ctx context.Context, id uuid.UUID) (db.MarkOutboxProcessedRow, error) {
	args := m.Called(ctx, id)
	return args.Get(0).(db.MarkOutboxProcessedRow), args.Error(1)
}

func (m *mockContactOutboxRepo) MarkFailed(ctx context.Context, id uuid.UUID, lastError *string) (db.MarkOutboxFailedRow, error) {
	args := m.Called(ctx, id, lastError)
	return args.Get(0).(db.MarkOutboxFailedRow), args.Error(1)
}

func (m *mockContactOutboxRepo) GetStats(ctx context.Context) (db.GetOutboxStatsRow, error) {
	args := m.Called(ctx)
	return args.Get(0).(db.GetOutboxStatsRow), args.Error(1)
}

func (m *mockContactOutboxRepo) ListFailed(ctx context.Context, limit int32) ([]db.ListFailedOutboxRow, error) {
	args := m.Called(ctx, limit)
	return args.Get(0).([]db.ListFailedOutboxRow), args.Error(1)
}

func (m *mockContactOutboxRepo) ClaimPending(ctx context.Context, limit int32) ([]db.ClaimPendingOutboxRow, error) {
	args := m.Called(ctx, limit)
	return args.Get(0).([]db.ClaimPendingOutboxRow), args.Error(1)
}

func (m *mockContactOutboxRepo) UpdateStatus(ctx context.Context, id uuid.UUID, status string, lastError *string) (db.UpdateOutboxStatusRow, error) {
	args := m.Called(ctx, id, status, lastError)
	return args.Get(0).(db.UpdateOutboxStatusRow), args.Error(1)
}

func (m *mockContactOutboxRepo) ListDeadLetter(ctx context.Context, limit int32) ([]db.ListDeadLetterOutboxRow, error) {
	args := m.Called(ctx, limit)
	return args.Get(0).([]db.ListDeadLetterOutboxRow), args.Error(1)
}

func (m *mockContactOutboxRepo) WithTx(tx pgx.Tx) persistence.OutboxRepository {
	args := m.Called(tx)
	if args.Get(0) == nil {
		return nil
	}
	return args.Get(0).(persistence.OutboxRepository)
}

func (m *mockContactOutboxRepo) SaveOutboxTx(ctx context.Context, tx pgx.Tx, id uuid.UUID, payloadType, payload, status string) error {
	args := m.Called(ctx, tx, id, payloadType, payload, status)
	return args.Error(0)
}

func TestCreateContact_BlankName(t *testing.T) {
	repo := new(mockContactRepo)
	outbox := new(mockContactOutboxRepo)
	svc := NewContactService(repo, outbox, &FakeTxRunner{}, &NoOpEventPublisher{}, nil)

	_, err := svc.CreateContact(context.Background(), "", nil, nil, nil, "", "", "")
	assert.Error(t, err)
	assert.IsType(t, &model.ValidationError{}, err)
}

func TestCreateContact_Success(t *testing.T) {
	repo := new(mockContactRepo)
	outbox := new(mockContactOutboxRepo)
	svc := NewContactService(repo, outbox, &FakeTxRunner{}, &NoOpEventPublisher{}, nil)

	// WithTx returns the same mock so the tx-bound write uses the same stubs.
	repo.On("WithTx", mock.Anything).Return(repo)
	repo.On("CreateServerContact", mock.Anything, mock.AnythingOfType("*model.StoredContact")).Return(db.PltContact{
		ID:               1,
		SyncID:           "srv_test",
		Name:             "Alice",
		UpdatedAtEpochMs: 1000,
		Version:          1,
		Company:          strPtr("Acme"),
	}, nil)
	repo.On("ListContactEmails", mock.Anything, int64(1)).Return([]string{"alice@example.com"}, nil)
	repo.On("ListContactPhones", mock.Anything, int64(1)).Return([]string{}, nil)
	repo.On("ListContactSocials", mock.Anything, int64(1)).Return([]string{}, nil)
	outbox.On("SaveOutboxTx", mock.Anything, mock.Anything, mock.AnythingOfType("uuid.UUID"), "CONTACT_SYNC", mock.AnythingOfType("string"), "PENDING").Return(nil)

	contact, err := svc.CreateContact(context.Background(), "Alice", []string{"alice@example.com"}, nil, nil, "Acme", "", "")

	assert.NoError(t, err)
	assert.Equal(t, "Alice", contact.Name)
	assert.Equal(t, []string{"alice@example.com"}, contact.Emails)
	assert.Equal(t, "Acme", contact.Company)
	repo.AssertExpectations(t)
	outbox.AssertExpectations(t)
}

func strPtr(s string) *string {
	return &s
}

func TestDeleteContact_Success(t *testing.T) {
	repo := new(mockContactRepo)
	outbox := new(mockContactOutboxRepo)
	svc := NewContactService(repo, outbox, &FakeTxRunner{}, &NoOpEventPublisher{}, nil)

	// WithTx returns the same mock so the tx-bound reads/writes use the same stubs.
	repo.On("WithTx", mock.Anything).Return(repo)
	repo.On("FindBySyncID", mock.Anything, "srv_test").Return(db.PltContact{
		ID:               1,
		SyncID:           "srv_test",
		Name:             "Alice",
		UpdatedAtEpochMs: 1000,
		Version:          1,
	}, nil)
	repo.On("SoftDeleteContact", mock.Anything, "srv_test").Return(db.PltContact{
		ID:     1,
		SyncID: "srv_test",
	}, nil)
	repo.On("ListContactEmails", mock.Anything, int64(1)).Return([]string{}, nil)
	repo.On("ListContactPhones", mock.Anything, int64(1)).Return([]string{}, nil)
	repo.On("ListContactSocials", mock.Anything, int64(1)).Return([]string{}, nil)
	outbox.On("SaveOutboxTx", mock.Anything, mock.Anything, mock.AnythingOfType("uuid.UUID"), "CONTACT_SYNC", mock.AnythingOfType("string"), "PENDING").Return(nil)

	err := svc.DeleteContact(context.Background(), "srv_test")

	assert.NoError(t, err)
	repo.AssertExpectations(t)
	outbox.AssertExpectations(t)
}
