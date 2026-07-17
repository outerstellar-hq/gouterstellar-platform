ALTER TABLE plt_message_votes
    DROP CONSTRAINT plt_message_votes_message_sync_id_fkey;

ALTER TABLE plt_messages
    ALTER COLUMN sync_id TYPE VARCHAR(64);

ALTER TABLE plt_contacts
    ALTER COLUMN sync_id TYPE VARCHAR(64);

ALTER TABLE plt_message_votes
    ALTER COLUMN message_sync_id TYPE VARCHAR(64),
    ADD CONSTRAINT plt_message_votes_message_sync_id_fkey
        FOREIGN KEY (message_sync_id) REFERENCES plt_messages(sync_id) ON DELETE CASCADE;
