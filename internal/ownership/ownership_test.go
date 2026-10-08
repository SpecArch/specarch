package ownership

import "testing"

func TestCovers(t *testing.T) {
	o := Of(map[string]any{"mappings": map[string]any{
		"#/entities/Book":      map[string]any{"target": "table books", "ownedBy": "catalogue-team"},
		"#/entities/Loan":      map[string]any{"target": "table loans"},
		"#/paths/~1shelves":    map[string]any{"target": "shelf service", "ownedBy": "catalogue-team"},
		"#/paths/~1loans/post": map[string]any{"target": "LendBook", "ownedBy": ""},
	}})
	for ptr, want := range map[string]bool{
		Entity("entities", "Book"):         true,
		"#/entities/Book/properties/title": true,
		Entity("entities", "Booking"):      false,
		Entity("entities", "Loan"):         false,
		Operation("/shelves", "get"):       true,
		Operation("/loans", "post"):        false,
	} {
		if got := o.Covers(ptr); got != want {
			t.Errorf("Covers(%s) = %v, want %v", ptr, got, want)
		}
	}
}
