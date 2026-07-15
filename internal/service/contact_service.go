package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/rygel/gouterstellar-platform/internal/model"
	"github.com/rygel/gouterstellar-platform/internal/persistence"
	"github.com/rygel/gouterstellar-platform/internal/persistence/db"
)

type ContactService struct {
	repo     persistence.ContactRepository
	outbox   persistence.OutboxRepository
	eventPub EventPublisher
}

func NewContactService(
	repo persistence.ContactRepository,
	outbox persistence.OutboxRepository,
	eventPub EventPublisher,
) *ContactService {
	return &ContactService{
		repo:     repo,
		outbox:   outbox,
		eventPub: eventPub,
	}
}

func (s *ContactService) ListContacts(ctx context.Context, limit, offset int32) ([]model.ContactSummary, error) {
	contacts, err := s.repo.ListContacts(ctx, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("list contacts: %w", err)
	}

	summaries := make([]model.ContactSummary, len(contacts))
	for i, c := range contacts {
		emails, _ := s.repo.ListContactEmails(ctx, c.ID)
		phones, _ := s.repo.ListContactPhones(ctx, c.ID)
		socials, _ := s.repo.ListContactSocials(ctx, c.ID)
		summaries[i] = pltContactToSummary(c, emails, phones, socials)
	}
	return summaries, nil
}

func (s *ContactService) CountContacts(ctx context.Context) (int64, error) {
	return s.repo.CountContacts(ctx)
}

func (s *ContactService) ListDeletedContacts(ctx context.Context, limit, offset int32) ([]model.ContactSummary, error) {
	contacts, err := s.repo.ListDeletedContacts(ctx, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("list deleted contacts: %w", err)
	}
	summaries := make([]model.ContactSummary, len(contacts))
	for i, contact := range contacts {
		emails, _ := s.repo.ListContactEmails(ctx, contact.ID)
		phones, _ := s.repo.ListContactPhones(ctx, contact.ID)
		socials, _ := s.repo.ListContactSocials(ctx, contact.ID)
		summaries[i] = pltContactToSummary(contact, emails, phones, socials)
	}
	return summaries, nil
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

	c, err := s.repo.CreateServerContact(ctx, contact)
	if err != nil {
		return nil, fmt.Errorf("create contact: %w", err)
	}

	stored := pltContactToStored(c, emails, phones, socials)

	s.saveContactOutboxEntry(ctx, c)
	s.eventPub.PublishRefresh("contacts")

	return stored, nil
}

func (s *ContactService) UpdateContact(ctx context.Context, contact *model.StoredContact) (*model.StoredContact, error) {
	existing, err := s.repo.FindBySyncID(ctx, contact.SyncID)
	if err != nil {
		return nil, &model.ContactNotFoundError{SyncID: contact.SyncID}
	}

	c, err := s.repo.UpdateContact(ctx, contact.SyncID, contact, existing.Version)
	if err != nil {
		return nil, fmt.Errorf("update contact: %w", err)
	}

	stored := pltContactToStored(c, contact.Emails, contact.Phones, contact.SocialMedia)

	s.saveContactOutboxEntry(ctx, c)
	s.eventPub.PublishRefresh("contacts")

	return stored, nil
}

func (s *ContactService) DeleteContact(ctx context.Context, syncID string) error {
	_, err := s.repo.SoftDeleteContact(ctx, syncID)
	if err != nil {
		return fmt.Errorf("delete contact: %w", err)
	}
	s.eventPub.PublishRefresh("contacts")
	return nil
}

func (s *ContactService) RestoreContact(ctx context.Context, syncID string) error {
	if _, err := s.repo.RestoreContact(ctx, syncID); err != nil {
		return fmt.Errorf("restore contact: %w", err)
	}
	s.eventPub.PublishRefresh("contacts")
	return nil
}

func (s *ContactService) GetChangesSince(ctx context.Context, since int64) (*model.SyncPullContactResponse, error) {
	contacts, err := s.repo.FindChangesSince(ctx, since)
	if err != nil {
		return nil, fmt.Errorf("find contact changes since: %w", err)
	}

	syncContacts := make([]model.SyncContact, len(contacts))
	for i, c := range contacts {
		emails, _ := s.repo.ListContactEmails(ctx, c.ID)
		phones, _ := s.repo.ListContactPhones(ctx, c.ID)
		socials, _ := s.repo.ListContactSocials(ctx, c.ID)
		syncContacts[i] = pltContactToSyncContact(c, emails, phones, socials)
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

	s.eventPub.PublishRefresh("contacts")

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

func ptrToString(p *string) string {
	if p == nil {
		return ""
	}
	return *p
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
