CREATE TABLE events (
    id   UUID        NOT NULL DEFAULT gen_random_uuid(),
    at   TIMESTAMPTZ NOT NULL,
    kind VARCHAR(20) NOT NULL,
    note TEXT,
    PRIMARY KEY (id, at, kind),
    CONSTRAINT events_kind_given CHECK (kind <> '')
) PARTITION BY RANGE (at);

CREATE TABLE events_2026 PARTITION OF events
    FOR VALUES FROM ('2026-01-01') TO ('2027-01-01')
    PARTITION BY LIST (kind);
CREATE TABLE events_2026_sale PARTITION OF events_2026 (note DEFAULT 'sale')
    FOR VALUES IN ('sale');
CREATE TABLE events_default PARTITION OF events DEFAULT;

CREATE INDEX idx_at ON events (at);
CREATE INDEX events_default_note ON events_default (note);
ALTER TABLE events_default ADD CONSTRAINT events_default_note_given CHECK (note <> '');
