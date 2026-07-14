package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/rygel/gouterstellar-platform/internal/model"
	"github.com/rygel/gouterstellar-platform/internal/persistence"
	"github.com/rygel/gouterstellar-platform/internal/persistence/db"
)

type ContactService struct {
	repo               persistence.ContactRepository
	outbox             persistence.OutboxRepository
	txMgr              TransactionRunner
	eventPub           EventPublisher
	notificationService *NotificationService
}

func NewContactService(
	repo persistence.ContactRepository,
	outbox persistence.OutboxRepository,
	txMgr TransactionRunner,
	eventPub EventPublisher,
	notificationService *NotificationService,
) *ContactService {
	return &ContactService{
		repo:               repo,
		outbox:             outbox,
		txMgr:              txMgr,
		eventPub:           eventPub,
		notificationService: notificationService,
	}
}

func (s *ContactService) ListContacts(ctx context.Context, limit, offset int32) ([]model.ContactSummary, error) {
	contacts, err := s.repo.ListContacts(ctx, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("list contacts: %w", err)
	}

	subs, err := loadContactSubs(ctx, s.repo, contacts)
	if err != nil {
		return nil, err
	}

	summaries := make([]model.ContactSummary, len(contacts))
	for i, c := range contacts {
		summaries[i] = pltContactToSummary(c, subs.Emails[c.ID], subs.Phones[c.ID], subs.Socials[c.ID])
	}
	return summaries, nil
}

func (s *ContactService) CountContacts(ctx context.Context) (int64, error) {
	return s.repo.CountContacts(ctx)
}

// SearchContacts returns one page of non-deleted contacts whose name or company
// match the query (case-insensitive ILIKE), plus the total match count. Mirrors
// MessageService.SearchMessages.
func (s *ContactService) SearchContacts(ctx context.Context, query string, limit, offset int32) ([]model.ContactSummary, int64, error) {
	total, err := s.repo.CountSearchContacts(ctx, query)
	if err != nil {
		return nil, 0, fmt.Errorf("count search contacts: %w", err)
	}

	contacts, err := s.repo.SearchContacts(ctx, query, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("search contacts: %w", err)
	}

	subs, err := loadContactSubs(ctx, s.repo, contacts)
	if err != nil {
		return nil, 0, err
	}

	summaries := make([]model.ContactSummary, len(contacts))
	for i, c := range contacts {
		summaries[i] = pltContactToSummary(c, subs.Emails[c.ID], subs.Phones[c.ID], subs.Socials[c.ID])
	}
	return summaries, total, nil
}

func (s *ContactService) GetContactBySyncID(ctx context.Context, syncID string) (*model.StoredContact, error) {
	c, err := s.repo.FindBySyncID(ctx, syncID)
	if err != nil {
		return nil, &model.ContactNotFoundError{SyncID: syncID}
	}

	emails, _ := s.repo.ListContactEmails(ctx, c.ID)
	phones, _ := s.repo.ListContactPhones(ctx, c.ID)
	socials, _ := s.repo.ListContactSocials(ctx, c.ID)

	return pltContactToStored(c, emails, phones, socials), nil
}

func (s *ContactService) CreateContact(ctx context.Context, name string, emails, phones, socials []string, company, companyAddress, department string) (*model.StoredContact, error) {
	if strings.TrimSpace(name) == "" {
		return nil, &model.ValidationError{Errors: []string{"Name must not be blank"}}
	}

	syncID := "srv_" + uuid.New().String()
	now := time.Now().UnixMilli()

	contact := &model.StoredContact{
		SyncID:           syncID,
		Name:             name,
		Emails:           emails,
		Phones:           phones,
		SocialMedia:      socials,
		Company:          company,
		CompanyAddress:   companyAddress,
		Department:       department,
		UpdatedAtEpochMs: now,
	}

	var stored *model.StoredContact
	err := s.txMgr.InTransaction(ctx, func(tx pgx.Tx) error {
		txRepo := s.repo.WithTx(tx)
		created, err := txRepo.CreateServerContact(ctx, contact)
		if err != nil {
			return fmt.Errorf("create contact: %w", err)
		}
		if err := s.saveContactOutboxEntryTx(ctx, tx, txRepo, created); err != nil {
			return err
		}
		stored = pltContactToStored(created, emails, phones, socials)
		return nil
	})
	if err != nil {
		return nil, err
	}

	s.eventPub.PublishRefresh(ActorUserIDFromContext(ctx), "contacts")

	s.notifyActor(ctx, "New Contact", name, "contact")

	return stored, nil
}

func (s *ContactService) UpdateContact(ctx context.Context, contact *model.StoredContact) (*model.StoredContact, error) {
	existing, err := s.repo.FindBySyncID(ctx, contact.SyncID)
	if err != nil {
		return nil, &model.ContactNotFoundError{SyncID: contact.SyncID}
	}

	var stored *model.StoredContact
	err = s.txMgr.InTransaction(ctx, func(tx pgx.Tx) error {
		txRepo := s.repo.WithTx(tx)
		c, err := txRepo.UpdateContact(ctx, contact.SyncID, contact, existing.Version)
		if err != nil {
			return fmt.Errorf("update contact: %w", err)
		}
		if err := s.saveContactOutboxEntryTx(ctx, tx, txRepo, c); err != nil {
			return err
		}
		stored = pltContactToStored(c, contact.Emails, contact.Phones, contact.SocialMedia)
		return nil
	})
	if err != nil {
		return nil, err
	}

	s.eventPub.PublishRefresh(ActorUserIDFromContext(ctx), "contacts")

	return stored, nil
}

func (s *ContactService) DeleteContact(ctx context.Context, syncID string) error {
	err := s.txMgr.InTransaction(ctx, func(tx pgx.Tx) error {
		txRepo := s.repo.WithTx(tx)
		contact, err := txRepo.FindBySyncID(ctx, syncID)
		if err != nil {
			return fmt.Errorf("find contact for delete: %w", err)
		}
		if _, err := txRepo.SoftDeleteContact(ctx, syncID); err != nil {
			return fmt.Errorf("delete contact: %w", err)
		}
		return s.saveContactOutboxEntryTx(ctx, tx, txRepo, contact)
	})
	if err != nil {
		return err
	}

	s.eventPub.PublishRefresh(ActorUserIDFromContext(ctx), "contacts")
	return nil
}

func (s *ContactService) GetChangesSince(ctx context.Context, since int64) (*model.SyncPullContactResponse, error) {
	contacts, err := s.repo.FindChangesSince(ctx, since)
	if err != nil {
		return nil, fmt.Errorf("find contact changes since: %w", err)
	}

	subs, err := loadContactSubs(ctx, s.repo, contacts)
	if err != nil {
		return nil, err
	}

	syncContacts := make([]model.SyncContact, len(contacts))
	for i, c := range contacts {
		syncContacts[i] = pltContactToSyncContact(c, subs.Emails[c.ID], subs.Phones[c.ID], subs.Socials[c.ID])
	}

	return &model.SyncPullContactResponse{
		Contacts:        syncContacts,
		ServerTimestamp: time.Now().UnixMilli(),
	}, nil
}

func (s *ContactService) ProcessPushRequest(ctx context.Context, req *model.SyncPushContactRequest) (*model.SyncPushContactResponse, error) {
	appliedCount := 0
	var conflicts []model.SyncContactConflict

	for _, c := range req.Contacts {
		existing, err := s.repo.FindBySyncID(ctx, c.SyncID)
		if err != nil {
			_, err := s.repo.UpsertSyncedContact(ctx, &c)
			if err != nil {
				conflicts = append(conflicts, model.SyncContactConflict{
					SyncID:        c.SyncID,
					Reason:        fmt.Sprintf("Failed to upsert: %v", err),
					ServerContact: nil,
				})
				continue
			}
			appliedCount++
			continue
		}

		if existing.Version > 0 && existing.UpdatedAtEpochMs > c.UpdatedAtEpochMs {
			emails, _ := s.repo.ListContactEmails(ctx, existing.ID)
			phones, _ := s.repo.ListContactPhones(ctx, existing.ID)
			socials, _ := s.repo.ListContactSocials(ctx, existing.ID)
			serverContact := pltContactToSyncContact(existing, emails, phones, socials)
			conflicts = append(conflicts, model.SyncContactConflict{
				SyncID:        c.SyncID,
				Reason:        "Server has newer version",
				ServerContact: &serverContact,
			})
			continue
		}

		_, err = s.repo.UpsertSyncedContact(ctx, &c)
		if err != nil {
			conflicts = append(conflicts, model.SyncContactConflict{
				SyncID:        c.SyncID,
				Reason:        fmt.Sprintf("Failed to upsert: %v", err),
				ServerContact: nil,
			})
			continue
		}
		appliedCount++
	}

	if conflicts == nil {
		conflicts = []model.SyncContactConflict{}
	}

	s.eventPub.PublishRefresh(ActorUserIDFromContext(ctx), "contacts")

	return &model.SyncPushContactResponse{
		AppliedCount: appliedCount,
		Conflicts:    conflicts,
	}, nil
}

func (s *ContactService) saveContactOutboxEntry(ctx context.Context, c db.PltContact) {
	emails, _ := s.repo.ListContactEmails(ctx, c.ID)
	phones, _ := s.repo.ListContactPhones(ctx, c.ID)
	socials, _ := s.repo.ListContactSocials(ctx, c.ID)
	syncContact := pltContactToSyncContact(c, emails, phones, socials)
	payload, err := model.SyncContactToJSON(syncContact)
	if err != nil {
		slog.Error("Failed to serialize contact outbox payload", "syncID", c.SyncID, "error", err)
		return
	}

	if err := s.outbox.SaveOutbox(ctx, uuid.New(), "CONTACT_SYNC", payload, "PENDING"); err != nil {
		slog.Error("Failed to save contact outbox entry", "syncID", c.SyncID, "error", err)
	}
}

// saveContactOutboxEntryTx serializes and inserts a contact outbox entry within
// a caller-supplied transaction. The tx-bound repo is used for the sub-table
// reads so the reads also participate in the transaction. It is the
// transactional counterpart of saveContactOutboxEntry.
func (s *ContactService) saveContactOutboxEntryTx(ctx context.Context, tx pgx.Tx, repo persistence.ContactRepository, c db.PltContact) error {
	emails, _ := repo.ListContactEmails(ctx, c.ID)
	phones, _ := repo.ListContactPhones(ctx, c.ID)
	socials, _ := repo.ListContactSocials(ctx, c.ID)
	syncContact := pltContactToSyncContact(c, emails, phones, socials)
	payload, err := model.SyncContactToJSON(syncContact)
	if err != nil {
		return fmt.Errorf("serialize contact outbox payload: %w", err)
	}
	return s.outbox.SaveOutboxTx(ctx, tx, uuid.New(), "CONTACT_SYNC", payload, "PENDING")
}

func ptrToString(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

// loadContactSubs collects the contact IDs from a page of contacts and fetches
// their emails/phones/socials in a single batched call per sub-table, rather
// than looping ListContactEmails/Phones/Socials per row (an N+1). Missing IDs
// resolve to nil slices, which the plt*To* converters treat as empty — matching
// the prior per-row behavior.
func loadContactSubs(ctx context.Context, repo persistence.ContactRepository, contacts []db.PltContact) (persistence.ContactSubTables, error) {
	ids := make([]int64, len(contacts))
	for i, c := range contacts {
		ids[i] = c.ID
	}
	subs, err := repo.LoadSubTablesBatch(ctx, ids)
	if err != nil {
		return persistence.ContactSubTables{}, fmt.Errorf("load contact sub-tables: %w", err)
	}
	return subs, nil
}

func pltContactToStored(c db.PltContact, emails, phones, socials []string) *model.StoredContact {
	return &model.StoredContact{
		SyncID:           c.SyncID,
		Name:             c.Name,
		Emails:           emails,
		Phones:           phones,
		SocialMedia:      socials,
		Company:          ptrToString(c.Company),
		CompanyAddress:   ptrToString(c.CompanyAddress),
		Department:       ptrToString(c.Department),
		UpdatedAtEpochMs: c.UpdatedAtEpochMs,
		Dirty:            c.Dirty,
		Deleted:          c.Deleted,
		Version:          c.Version,
		SyncConflict:     c.SyncConflict,
	}
}

func pltContactToSummary(c db.PltContact, emails, phones, socials []string) model.ContactSummary {
	return model.ContactSummary{
		SyncID:           c.SyncID,
		Name:             c.Name,
		Emails:           emails,
		Phones:           phones,
		SocialMedia:      socials,
		Company:          ptrToString(c.Company),
		CompanyAddress:   ptrToString(c.CompanyAddress),
		Department:       ptrToString(c.Department),
		UpdatedAtEpochMs: c.UpdatedAtEpochMs,
		Dirty:            c.Dirty,
		Version:          c.Version,
		HasConflict:      c.SyncConflict != nil,
	}
}

func pltContactToSyncContact(c db.PltContact, emails, phones, socials []string) model.SyncContact {
	return model.SyncContact{
		SyncID:           c.SyncID,
		Name:             c.Name,
		Emails:           emails,
		Phones:           phones,
		SocialMedia:      socials,
		Company:          ptrToString(c.Company),
		CompanyAddress:   ptrToString(c.CompanyAddress),
		Department:       ptrToString(c.Department),
		UpdatedAtEpochMs: c.UpdatedAtEpochMs,
		Deleted:          c.Deleted,
	}
}

// Keep json import used - needed for conflict resolution in future
var _ = json.Marshal

// notifyActor records a best-effort notification for the user acting on the
// current request. It is nil-safe: if no NotificationService is wired or no
// authenticated user is present in the context, it does nothing. Notification
// failures are logged but never propagated.
func (s *ContactService) notifyActor(ctx context.Context, title, body, nType string) {
	if s.notificationService == nil {
		return
	}
	user := UserFromContext(ctx)
	if user == nil {
		return
	}
	if err := s.notificationService.Create(ctx, user.ID, title, body, nType); err != nil {
		slog.Warn("Failed to create notification", "title", title, "error", err)
	}
}
