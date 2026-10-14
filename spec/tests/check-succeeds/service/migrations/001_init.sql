CREATE TABLE branches (code VARCHAR(8) PRIMARY KEY, name VARCHAR(100) NOT NULL);
CREATE TABLE copies (
    id BIGSERIAL PRIMARY KEY,
    branch_code VARCHAR(8) NOT NULL REFERENCES branches (code),
    title VARCHAR(200) NOT NULL,
    label CHAR(4),
    reference UUID NOT NULL DEFAULT gen_random_uuid(),
    CONSTRAINT copies_title_given CHECK (title <> '')
);
CREATE INDEX ix_copies_branch_code ON copies (branch_code);
CREATE INDEX copies_title_lower ON copies (lower(title));
CREATE TABLE audit_log (id BIGSERIAL PRIMARY KEY, said TEXT NOT NULL);
