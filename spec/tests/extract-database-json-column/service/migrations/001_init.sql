CREATE TABLE members (
    card_number VARCHAR(10) PRIMARY KEY,
    preferences JSONB NOT NULL,
    history     JSON
);
