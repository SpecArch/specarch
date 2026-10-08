# Branch handbook

The handbook the staff of a branch library work from. Edition 4.

## 1 Members

1.1 A member is known by a card number. The branch records each member's
full name and phone number, which are personal.

| Field | Type | Required | Sensitivity |
|---|---|---|---|
| card number | text | yes | internal |
| full name | text | yes | personal |

1.2 Staff should check the card at every visit.

## 2 Loans

2.1 A loan lasts 14 days. It must be returned to the branch that lent it.

| Field | Type | Required |
|---|---|---|
| loaned on | date | yes |
| fee | money | no |
| phone number | text | no |

2.2 The branch shall send a reminder before the due date:

- by text message, and
- by post when the member has no phone.

## 3 Opening

The branch opens at 9:00.

| Day | Hours |
|---|---|
| Monday | 9:00 to 17:00 |

```
open 09:00
```
