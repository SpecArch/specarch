// Package lending is the lending desk's service, as built. It is the code
// source of the lending-desk example.
package lending

import "time"

// LoanPeriod is how long a book may be kept.
const LoanPeriod = 14 * 24 * time.Hour

// MaxOpenLoans is how many books one member may have out at a time.
const MaxOpenLoans = 5

// Member is a card holder.
type Member struct {
	CardNumber string // printed on the card, 10 digits
	FullName   string
	Address    *Address // nil when the member gave none
}

// Address is where a member lives. One that is given has its street and
// city; the postcode may be left out.
type Address struct {
	Street   string
	City     string
	Postcode string
}

// Book is one copy on the shelves.
type Book struct {
	Barcode string
	Title   string
}

// Loan is one book lent to one member.
type Loan struct {
	ID         int64
	CardNumber string
	Barcode    string
	LoanedOn   time.Time
	DueOn      time.Time
	ReturnedOn *time.Time
}
