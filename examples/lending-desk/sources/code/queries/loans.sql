-- A loan is due fourteen days after it is lent (the loans_loan_period
-- check), and a member may have five books out at once (MaxOpenLoans).

-- name: LendBook :one
INSERT INTO loans (card_number, barcode, loaned_on, due_on)
VALUES ($1, $2, CURRENT_DATE, CURRENT_DATE + 14)
RETURNING id, card_number, barcode, loaned_on, due_on, returned_on, reference;

-- name: CountOpenLoans :one
SELECT count(*) AS open_loans
FROM loans
WHERE card_number = $1
  AND returned_on IS NULL;

-- name: ReturnBook :execrows
UPDATE loans
SET returned_on = CURRENT_DATE
WHERE id = $1
  AND returned_on IS NULL;

-- name: ListMemberLoans :many
SELECT id, card_number, barcode, loaned_on, due_on, returned_on, reference
FROM loans
WHERE card_number = $1
ORDER BY loaned_on DESC, id;
