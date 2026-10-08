CREATE TABLE members (
    card_number CHAR(10) PRIMARY KEY,
    full_name   VARCHAR(200) NOT NULL
);

CREATE TABLE books (
    barcode VARCHAR(20) PRIMARY KEY,
    title   VARCHAR(300) NOT NULL
);

CREATE TABLE loans (
    id          BIGSERIAL PRIMARY KEY,
    card_number CHAR(10) NOT NULL REFERENCES members (card_number),
    barcode     VARCHAR(20) NOT NULL REFERENCES books (barcode),
    loaned_on   DATE NOT NULL,
    due_on      DATE NOT NULL,
    returned_on DATE,
    CONSTRAINT loans_loan_period CHECK (due_on = loaned_on + 14)
);
