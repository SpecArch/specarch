CREATE TYPE copy_condition AS ENUM ('good', 'worn', 'damaged');

CREATE TABLE branches (
    code         SMALLINT PRIMARY KEY,
    name         VARCHAR(100) NOT NULL,
    opened_on    DATE NOT NULL DEFAULT '2020-01-01',
    fine_per_day NUMERIC(6,2) NOT NULL DEFAULT 0.50,
    float_cash   MONEY,
    CONSTRAINT branches_name_unique UNIQUE (name),
    CONSTRAINT branches_fine_not_negative CHECK (fine_per_day >= 0)
);
COMMENT ON TABLE branches IS 'A branch that lends books.';

CREATE TABLE copies (
    id          BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    branch_code SMALLINT NOT NULL REFERENCES branches (code) ON DELETE CASCADE,
    state       VARCHAR(10) NOT NULL DEFAULT 'shelved',
    condition   copy_condition,
    label       CHAR(4),
    shelved_at  TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT now(),
    title       TEXT NOT NULL,
    CONSTRAINT copies_state CHECK (state IN ('shelved', 'lent', 'lost')),
    CONSTRAINT copies_title_not_empty CHECK (length(title) > 0)
);
COMMENT ON COLUMN copies.label IS 'The shelf label.';
CREATE INDEX copies_title ON copies (title);

CREATE TABLE notes (body TEXT);

CREATE VIEW lent_copies AS SELECT id FROM copies WHERE state = 'lent';
