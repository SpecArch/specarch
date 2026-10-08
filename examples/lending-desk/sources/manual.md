# Lending desk manual

Edition 2025. The manual the desk staff of a small lending library work
from. It is the prose source of the `lending-desk` example: a specification
is built from it and from the code beside it, by the procedure in
`docs/from-sources.md`.

## 1 Purpose

The lending desk lends books to registered members and takes them back.

## 2 Members

The desk keeps these details of each member.

| Field | Type | Required | Sensitivity |
|---|---|---|---|
| Card number | text | yes | internal |
| Full name | text | yes | personal |
| Phone number | text | no | personal |

2.1 Anyone with a library card may borrow. A member is known by the card
number printed on the card, and the desk records the member's full name.

2.2 Desk staff register new members and hand out the card.

## 3 Lending

3.1 Desk staff lend a book by scanning the member's card and the book's
barcode.

3.2 A member may have at most five books on loan at a time.

3.3 The loan period is 21 days. The due date is printed on the slip.

3.4 The catalogue team keeps the book records: it adds each copy with its
barcode and title, and changes them. The desk reads them and never edits
them.

## 4 Renewals

4.1 A member may renew a loan once, for one more loan period, by asking at
the desk before the due date.

## 5 Returns

5.1 Desk staff check a returned book in by scanning its barcode, which
closes the loan.

## 6 Opening hours

The desk is open from 9:00 to 17:00 on weekdays.
