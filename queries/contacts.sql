-- name: ListContacts :many
SELECT id, sync_id, name, company, company_address, department, created_at, updated_at_epoch_ms, deleted, dirty, version, sync_conflict
FROM plt_contacts
WHERE deleted = false
ORDER BY name ASC
LIMIT $1 OFFSET $2;

-- name: CountContacts :one
SELECT COUNT(*) FROM plt_contacts WHERE deleted = false;

-- name: ListDirtyContacts :many
SELECT id, sync_id, name, company, company_address, department, created_at, updated_at_epoch_ms, deleted, dirty, version, sync_conflict
FROM plt_contacts
WHERE dirty = true AND deleted = false
ORDER BY updated_at_epoch_ms ASC;

-- name: FindContactBySyncID :one
SELECT id, sync_id, name, company, company_address, department, created_at, updated_at_epoch_ms, deleted, dirty, version, sync_conflict
FROM plt_contacts
WHERE sync_id = $1;

-- name: FindContactChangesSince :many
SELECT id, sync_id, name, company, company_address, department, created_at, updated_at_epoch_ms, deleted, dirty, version, sync_conflict
FROM plt_contacts
WHERE updated_at_epoch_ms > $1
ORDER BY updated_at_epoch_ms ASC;

-- name: CreateServerContact :one
INSERT INTO plt_contacts (sync_id, name, company, company_address, department, updated_at_epoch_ms, dirty, deleted, version)
VALUES ($1, $2, $3, $4, $5, $6, false, false, 1)
RETURNING id, sync_id, name, company, company_address, department, created_at, updated_at_epoch_ms, deleted, dirty, version, sync_conflict;

-- name: CreateLocalContact :one
INSERT INTO plt_contacts (sync_id, name, company, company_address, department, updated_at_epoch_ms, dirty, deleted, version)
VALUES ($1, $2, $3, $4, $5, $6, true, false, 1)
RETURNING id, sync_id, name, company, company_address, department, created_at, updated_at_epoch_ms, deleted, dirty, version, sync_conflict;

-- name: UpsertSyncedContact :one
INSERT INTO plt_contacts (sync_id, name, company, company_address, department, updated_at_epoch_ms, dirty, deleted, version)
VALUES ($1, $2, $3, $4, $5, $6, false, $7, 1)
ON CONFLICT (sync_id) DO UPDATE SET
    name = EXCLUDED.name,
    company = EXCLUDED.company,
    company_address = EXCLUDED.company_address,
    department = EXCLUDED.department,
    updated_at_epoch_ms = EXCLUDED.updated_at_epoch_ms,
    dirty = false,
    deleted = EXCLUDED.deleted,
    version = plt_contacts.version + 1,
    sync_conflict = NULL
RETURNING id, sync_id, name, company, company_address, department, created_at, updated_at_epoch_ms, deleted, dirty, version, sync_conflict;

-- name: SoftDeleteContact :one
UPDATE plt_contacts
SET deleted = true, dirty = true, version = version + 1
WHERE sync_id = $1
RETURNING id, sync_id, name, company, company_address, department, created_at, updated_at_epoch_ms, deleted, dirty, version, sync_conflict;

-- name: RestoreContact :one
UPDATE plt_contacts
SET deleted = false, dirty = true, version = version + 1
WHERE sync_id = $1
RETURNING id, sync_id, name, company, company_address, department, created_at, updated_at_epoch_ms, deleted, dirty, version, sync_conflict;

-- name: UpdateContact :one
UPDATE plt_contacts
SET name = $2, company = $3, company_address = $4, department = $5, updated_at_epoch_ms = $6, dirty = $7, version = version + 1
WHERE sync_id = $1 AND version = $8
RETURNING id, sync_id, name, company, company_address, department, created_at, updated_at_epoch_ms, deleted, dirty, version, sync_conflict;

-- name: MarkConflictContact :one
UPDATE plt_contacts
SET sync_conflict = $2, version = version + 1
WHERE sync_id = $1
RETURNING id, sync_id, name, company, company_address, department, created_at, updated_at_epoch_ms, deleted, dirty, version, sync_conflict;

-- name: ResolveConflictContact :one
UPDATE plt_contacts
SET sync_conflict = NULL, version = version + 1
WHERE sync_id = $1
RETURNING id, sync_id, name, company, company_address, department, created_at, updated_at_epoch_ms, deleted, dirty, version, sync_conflict;

-- name: MarkCleanContacts :exec
UPDATE plt_contacts SET dirty = false WHERE dirty = true;

-- name: ListContactEmails :many
SELECT email FROM plt_contact_emails WHERE contact_id = $1;

-- name: SetContactEmails :exec
DELETE FROM plt_contact_emails WHERE contact_id = $1;

-- name: InsertContactEmail :exec
INSERT INTO plt_contact_emails (contact_id, email) VALUES ($1, $2);

-- name: ListContactPhones :many
SELECT phone FROM plt_contact_phones WHERE contact_id = $1;

-- name: SetContactPhones :exec
DELETE FROM plt_contact_phones WHERE contact_id = $1;

-- name: InsertContactPhone :exec
INSERT INTO plt_contact_phones (contact_id, phone) VALUES ($1, $2);

-- name: ListContactSocials :many
SELECT social_media FROM plt_contact_socials WHERE contact_id = $1;

-- name: SetContactSocials :exec
DELETE FROM plt_contact_socials WHERE contact_id = $1;

-- name: InsertContactSocial :exec
INSERT INTO plt_contact_socials (contact_id, social_media) VALUES ($1, $2);
