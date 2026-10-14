-- A loan's reference is what a slip prints, so a loan can be found without
-- its key; the database draws it. The desk lists a member's loans and the
-- loans falling due, so both are indexed.
ALTER TABLE loans
    ADD COLUMN reference UUID NOT NULL DEFAULT gen_random_uuid();

CREATE INDEX ix_loans_card_number ON loans (card_number);
CREATE INDEX loans_falling_due ON loans (due_on, returned_on);
