package persistence

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/rygel/gouterstellar-platform/internal/model"
	"github.com/rygel/gouterstellar-platform/internal/persistence/db"
)

type contactRepo struct {
	q    *db.Queries
	pool *pgxpool.Pool
}

func NewContactRepository(pool *pgxpool.Pool) ContactRepository {
	return &contactRepo{q: db.New(pool), pool: pool}
}

// WithTx returns a copy of this repository whose underlying sqlc Queries is
// bound to the given transaction. Operations on the returned repository
// participate in the transaction and only persist when the transaction commits.
func (r *contactRepo) WithTx(tx pgx.Tx) ContactRepository {
	return &contactRepo{q: r.q.WithTx(tx), pool: nil}
}

func (r *contactRepo) ListContacts(ctx context.Context, limit, offset int32) ([]db.PltContact, error) {
	return r.q.ListContacts(ctx, db.ListContactsParams{
		Limit:  limit,
		Offset: offset,
	})
}

func (r *contactRepo) CountContacts(ctx context.Context) (int64, error) {
	return r.q.CountContacts(ctx)
}

func (r *contactRepo) ListDirtyContacts(ctx context.Context) ([]db.PltContact, error) {
	return r.q.ListDirtyContacts(ctx)
}

func (r *contactRepo) FindBySyncID(ctx context.Context, syncID string) (db.PltContact, error) {
	return r.q.FindContactBySyncID(ctx, syncID)
}

func (r *contactRepo) FindChangesSince(ctx context.Context, since int64) ([]db.PltContact, error) {
	return r.q.FindContactChangesSince(ctx, since)
}

func (r *contactRepo) CreateServerContact(ctx context.Context, contact *model.StoredContact) (db.PltContact, error) {
	row, err := r.q.CreateServerContact(ctx, db.CreateServerContactParams{
		SyncID:           contact.SyncID,
		Name:             contact.Name,
		Company:          strToPtr(contact.Company),
		CompanyAddress:   strToPtr(contact.CompanyAddress),
		Department:       strToPtr(contact.Department),
		UpdatedAtEpochMs: contact.UpdatedAtEpochMs,
	})
	if err != nil {
		return row, err
	}
	if err := r.setContactSubTables(ctx, row.ID, contact); err != nil {
		return row, err
	}
	return row, nil
}

func (r *contactRepo) CreateLocalContact(ctx context.Context, contact *model.StoredContact) (db.PltContact, error) {
	row, err := r.q.CreateLocalContact(ctx, db.CreateLocalContactParams{
		SyncID:           contact.SyncID,
		Name:             contact.Name,
		Company:          strToPtr(contact.Company),
		CompanyAddress:   strToPtr(contact.CompanyAddress),
		Department:       strToPtr(contact.Department),
		UpdatedAtEpochMs: contact.UpdatedAtEpochMs,
	})
	if err != nil {
		return row, err
	}
	if err := r.setContactSubTables(ctx, row.ID, contact); err != nil {
		return row, err
	}
	return row, nil
}

func (r *contactRepo) UpsertSyncedContact(ctx context.Context, contact *model.SyncContact) (db.PltContact, error) {
	row, err := r.q.UpsertSyncedContact(ctx, db.UpsertSyncedContactParams{
		SyncID:           contact.SyncID,
		Name:             contact.Name,
		Company:          strToPtr(contact.Company),
		CompanyAddress:   strToPtr(contact.CompanyAddress),
		Department:       strToPtr(contact.Department),
		UpdatedAtEpochMs: contact.UpdatedAtEpochMs,
		Deleted:          contact.Deleted,
	})
	if err != nil {
		return row, err
	}
	if err := r.setContactStrings(ctx, row.ID, contact.Emails, contact.Phones, contact.SocialMedia); err != nil {
		return row, err
	}
	return row, nil
}

func (r *contactRepo) SoftDeleteContact(ctx context.Context, syncID string) (db.PltContact, error) {
	return r.q.SoftDeleteContact(ctx, syncID)
}

func (r *contactRepo) RestoreContact(ctx context.Context, syncID string) (db.PltContact, error) {
	return r.q.RestoreContact(ctx, syncID)
}

func (r *contactRepo) UpdateContact(ctx context.Context, syncID string, contact *model.StoredContact, version int64) (db.PltContact, error) {
	row, err := r.q.UpdateContact(ctx, db.UpdateContactParams{
		SyncID:           syncID,
		Name:             contact.Name,
		Company:          strToPtr(contact.Company),
		CompanyAddress:   strToPtr(contact.CompanyAddress),
		Department:       strToPtr(contact.Department),
		UpdatedAtEpochMs: contact.UpdatedAtEpochMs,
		Dirty:            true,
		Version:          version,
	})
	if err != nil {
		return row, err
	}
	if err := r.setContactSubTables(ctx, row.ID, contact); err != nil {
		return row, err
	}
	return row, nil
}

func (r *contactRepo) MarkConflictContact(ctx context.Context, syncID string, conflict string) (db.PltContact, error) {
	return r.q.MarkConflictContact(ctx, db.MarkConflictContactParams{
		SyncID:       syncID,
		SyncConflict: &conflict,
	})
}

func (r *contactRepo) ResolveConflictContact(ctx context.Context, syncID string) (db.PltContact, error) {
	return r.q.ResolveConflictContact(ctx, syncID)
}

func (r *contactRepo) MarkCleanContacts(ctx context.Context) error {
	return r.q.MarkCleanContacts(ctx)
}

func (r *contactRepo) ListContactEmails(ctx context.Context, contactID int64) ([]string, error) {
	return r.q.ListContactEmails(ctx, contactID)
}

func (r *contactRepo) SetContactEmails(ctx context.Context, contactID int64, emails []string) error {
	return r.setContactEmailsList(ctx, contactID, emails)
}

func (r *contactRepo) ListContactPhones(ctx context.Context, contactID int64) ([]string, error) {
	return r.q.ListContactPhones(ctx, contactID)
}

func (r *contactRepo) SetContactPhones(ctx context.Context, contactID int64, phones []string) error {
	return r.setContactPhonesList(ctx, contactID, phones)
}

func (r *contactRepo) ListContactSocials(ctx context.Context, contactID int64) ([]string, error) {
	return r.q.ListContactSocials(ctx, contactID)
}

func (r *contactRepo) SetContactSocials(ctx context.Context, contactID int64, socials []string) error {
	return r.setContactSocialsList(ctx, contactID, socials)
}

func (r *contactRepo) setContactSubTables(ctx context.Context, contactID int64, contact *model.StoredContact) error {
	return r.setContactStrings(ctx, contactID, contact.Emails, contact.Phones, contact.SocialMedia)
}

func (r *contactRepo) setContactStrings(ctx context.Context, contactID int64, emails, phones, socials []string) error {
	if err := r.setContactEmailsList(ctx, contactID, emails); err != nil {
		return err
	}
	if err := r.setContactPhonesList(ctx, contactID, phones); err != nil {
		return err
	}
	if err := r.setContactSocialsList(ctx, contactID, socials); err != nil {
		return err
	}
	return nil
}

func (r *contactRepo) setContactEmailsList(ctx context.Context, contactID int64, emails []string) error {
	if err := r.q.SetContactEmails(ctx, contactID); err != nil {
		return err
	}
	for _, email := range emails {
		if err := r.q.InsertContactEmail(ctx, db.InsertContactEmailParams{
			ContactID: contactID,
			Email:     email,
		}); err != nil {
			return err
		}
	}
	return nil
}

func (r *contactRepo) setContactPhonesList(ctx context.Context, contactID int64, phones []string) error {
	if err := r.q.SetContactPhones(ctx, contactID); err != nil {
		return err
	}
	for _, phone := range phones {
		if err := r.q.InsertContactPhone(ctx, db.InsertContactPhoneParams{
			ContactID: contactID,
			Phone:     phone,
		}); err != nil {
			return err
		}
	}
	return nil
}

func (r *contactRepo) setContactSocialsList(ctx context.Context, contactID int64, socials []string) error {
	if err := r.q.SetContactSocials(ctx, contactID); err != nil {
		return err
	}
	for _, social := range socials {
		if err := r.q.InsertContactSocial(ctx, db.InsertContactSocialParams{
			ContactID:   contactID,
			SocialMedia: social,
		}); err != nil {
			return err
		}
	}
	return nil
}

func strToPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
