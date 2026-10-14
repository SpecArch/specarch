CREATE TABLE notes (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    author_id  UUID NOT NULL,
    shelf      VARCHAR(40) NOT NULL,
    written_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    edited_at  TIMESTAMPTZ DEFAULT now(),
    label      TEXT DEFAULT md5(random()::text)
);

CREATE INDEX ix_notes_author_id_written_at ON notes (author_id, written_at);
CREATE INDEX notes_by_shelf ON notes (shelf);
CREATE INDEX notes_recent ON notes (written_at DESC);
CREATE INDEX notes_shelf_lower ON notes (lower(shelf));
