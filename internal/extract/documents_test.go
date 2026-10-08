package extract

import (
	"reflect"
	"testing"
)

// TestSentences splits at a stop followed by a capital or a digit, and
// keeps a time of day, a decimal and an abbreviation followed by a small
// letter inside one sentence.
func TestSentences(t *testing.T) {
	got := sentences("The desk opens at 9:00. A fee of 0.50 is due, e.g. for a late book. 2 copies are kept! Is it open?")
	want := []string{"The desk opens at 9:00.", "A fee of 0.50 is due, e.g. for a late book.", "2 copies are kept!", "Is it open?"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %q\nwant %q", got, want)
	}
}

// TestCommitment reads a modal, a quantity, a frequency and a time limit,
// and leaves an edition number and plain prose alone.
func TestCommitment(t *testing.T) {
	for s, want := range map[string]string{
		"Staff shall check the card.":                 "must",
		"Staff should check the card.":                "should",
		"The loan period is 21 days.":                 "must",
		"A member may have at most five books.":       "must",
		"A loan may be renewed once.":                 "must",
		"Books are returned before the due date.":     "must",
		"The desk opens at 9:00.":                     "must",
		"Edition 2025.":                               "",
		"A member is known by the card number.":       "",
		"The desk lends books to registered members.": "",
	} {
		got, ok := commitment(s)
		if ok != (want != "") || got != want {
			t.Errorf("commitment(%q) = %q, %v; want %q", s, got, ok, want)
		}
	}
}

// TestDaysCommitment reads the subject and the days of a statement.
func TestDaysCommitment(t *testing.T) {
	subject, days, ok := daysCommitment("The loan period is 21 days.")
	if !ok || days != 21 || !reflect.DeepEqual(subject, []string{"loan", "period"}) {
		t.Errorf("got %v %d %v", subject, days, ok)
	}
	if _, days, ok := daysCommitment("The reminder period is two days."); !ok || days != 2 {
		t.Errorf("two days: got %d %v", days, ok)
	}
	if _, _, ok := daysCommitment("A loan lasts 14 days and a renewal 7 days."); ok {
		t.Errorf("two numbers of days should not be read as one")
	}
	if d, ok := checkDays(`dueOn == loanedOn + duration("P14D")`); !ok || d != 14 {
		t.Errorf("checkDays: got %d %v", d, ok)
	}
	if _, ok := checkDays(`dueOn > loanedOn`); ok {
		t.Errorf("a check with no days should give none")
	}
}
