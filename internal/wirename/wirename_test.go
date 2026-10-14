package wirename

import "testing"

func TestSnake(t *testing.T) {
	for in, want := range map[string]string{
		"id": "id", "loanedOn": "loaned_on", "userID": "user_id", "userId": "user_id",
		"HTTPServer": "http_server", "address1Line": "address1_line", "line2": "line2",
	} {
		if got := Snake(in); got != want {
			t.Errorf("Snake(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestCamel(t *testing.T) {
	for in, want := range map[string]struct {
		name string
		ok   bool
	}{
		"card_number": {"cardNumber", true}, "id": {"id", true}, "address1_line": {"address1Line", true},
		"line_2": {"line2", false}, "card__number": {"cardNumber", false}, "_id": {"Id", false}, "user_i_d": {"userID", false},
	} {
		got, ok := Camel(in)
		if got != want.name || ok != want.ok {
			t.Errorf("Camel(%q) = %q, %v, want %q, %v", in, got, ok, want.name, want.ok)
		}
	}
}

func TestRole(t *testing.T) {
	for name, want := range map[string]string{"desk-supervisor": "DESK_SUPERVISOR", "auditor": "AUDITOR", "level-2-clerk": "LEVEL_2_CLERK", "desk_staff": "DESK_STAFF"} {
		if got := Role(UpperSnake, name); got != want {
			t.Errorf("Role(%q) = %q, want %q", name, got, want)
		}
	}
	if got := Role("", "desk-supervisor"); got != "desk-supervisor" {
		t.Errorf("Role with no rule = %q, want the name as written", got)
	}
}
