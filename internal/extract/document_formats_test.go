package extract

import (
	"os"
	"strings"
	"testing"
)

func TestWordNumbering(t *testing.T) {
	wn := &wordNumbering{
		abstract: map[string]map[int]wordLevel{
			"0": {0: {start: 1, format: "decimal", text: "%1."}, 1: {start: 1, format: "decimal", text: "%1.%2"}, 2: {start: 1, format: "lowerRoman", text: "(%3)"}},
			"1": {0: {start: 1, format: "upperLetter", text: "Annex %1"}},
			"2": {0: {start: 1, format: "upperRoman", text: "%1"}, 1: {start: 1, format: "lowerLetter", text: "%1.%2", legal: true}},
			"3": {0: {start: 1, format: "ordinalText", text: "%1"}},
		},
		nums:     map[string]string{"1": "0", "2": "0", "3": "1", "4": "0", "5": "2", "6": "3"},
		starts:   map[string]map[int]int{"4": {0: 7}},
		counters: map[string][]int{},
	}
	steps := []struct{ num, lvl, want string }{
		{"1", "0", "1."}, {"1", "1", "1.1"}, {"1", "1", "1.2"}, {"1", "2", "(i)"}, {"1", "2", "(ii)"},
		{"2", "0", "2."}, // another num of the same abstract numbering counts on
		{"1", "1", "2.1"}, {"1", "2", "(i)"},
		{"3", "0", "Annex A"}, {"3", "0", "Annex B"},
		{"4", "0", "7."}, {"4", "1", "7.1"}, // a start override restarts its own list
		{"5", "1", "1.1"}, // a skipped level counts from its start; legal numbering is decimal
		{"0", "0", ""}, {"", "0", ""}, {"9", "0", ""},
	}
	for _, s := range steps {
		if got, _ := wn.next(s.num, s.lvl); got != s.want {
			t.Errorf("next(%s, %s) = %q, want %q", s.num, s.lvl, got, s.want)
		}
	}
	if got, f := wn.next("6", "0"); got != "" || f != "ordinalText" {
		t.Errorf("ordinalText gave %q, %q", got, f)
	}
}

func TestRoman(t *testing.T) {
	for n, want := range map[int]string{1: "I", 4: "IV", 9: "IX", 14: "XIV", 1999: "MCMXCIX", 0: "", 4000: ""} {
		if got := roman(n); got != want {
			t.Errorf("roman(%d) = %s, want %s", n, got, want)
		}
	}
}

// A PDF encrypted with a user password is refused with the reason, not as
// a file that does not parse.
func TestEncryptedPDF(t *testing.T) {
	data, err := os.ReadFile("../../spec/tests/extract-documents-pdf-encrypted/docs/notice.pdf")
	if err != nil {
		t.Fatal(err)
	}
	_, err = readPDF("notice.pdf", data, 0)
	if err == nil || !strings.Contains(err.Error(), "the PDF is encrypted with a password") {
		t.Fatalf("readPDF on an encrypted PDF gave %v, want the password named", err)
	}
}
