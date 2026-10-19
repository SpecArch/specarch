package extract

import "testing"

// TestKebab starts a word at a capital after a small letter or a digit,
// keeps a run of capitals as one word up to the capital a small letter
// follows, and joins the separators into single dashes.
func TestKebab(t *testing.T) {
	for in, want := range map[string]string{
		"createLoan":             "create-loan",
		"getMembersByCardNumber": "get-members-by-card-number",
		"ARCHITECTURE":           "architecture",
		"README":                 "readme",
		"FSD-Login":              "fsd-login",
		"HTTPServer":             "http-server",
		"userID":                 "user-id",
		"V2Report":               "v2-report",
		"ORDER_CREATE":           "order-create",
		"Loan Policy.v2":         "loan-policy-v2",
		"_private":               "private",
	} {
		if got := kebab(in); got != want {
			t.Errorf("kebab(%q): got %q, want %q", in, got, want)
		}
	}
}

// TestEncodeQuotesNumericText writes a string whose text reads as a
// number, such as a commit of digits, in quotes, so a reader of the file
// gets the string back and a schema that asks for one accepts it.
func TestEncodeQuotesNumericText(t *testing.T) {
	for _, commit := range []string{"1111111111111111111111111111111111111111", "1234567"} {
		out, err := encode(mapping("commit", commit, "depth", 3))
		if err != nil {
			t.Fatalf("encode: %v", err)
		}
		want := "commit: \"" + commit + "\"\ndepth: 3\n"
		if string(out) != want {
			t.Errorf("encode of commit %s:\n got %q\nwant %q", commit, out, want)
		}
	}
}
