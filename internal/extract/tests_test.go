package extract

import (
	"strings"
	"testing"
)

// TestReadFeature checks the parts of Gherkin a feature file is read by:
// a Rule's Background after the Feature's, two Examples blocks each under
// its own header, a doc string and a data table joined to their step, a
// step whose text holds a colon, and the tags that say a scenario is red.
func TestReadFeature(t *testing.T) {
	text := `Feature: Notes
  Background:
    Given a signed-in writer

  Rule: Keeping
    Background:
      Given an empty board

    @negative
    Scenario: Keep a note at 10:00
      When the writer sends POST /notes with
        """
        { "text": "hello" }
        """
      Then it answers 201
      And the board shows:
        | text  |
        | hello |

  Rule: Reading
    Scenario Outline: Read a note
      Given a note <id>
      When the writer reads it at <time>
      Then it answers <status>

      Examples: kept
        | id | time  | status |
        | n1 | 10:00 | 200    |

      Examples: missing
        | status | id |
        | 404    | n9 |
        |        | n8 |
`
	tests, err := readFeature(&scriptFile{Name: "notes.feature", Key: "notes"}, text)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, tc := range tests {
		var gaps []string
		for _, part := range []string{"given", "when", "then"} {
			for _, g := range tc.unfilled[part] {
				gaps = append(gaps, part+" "+g.why)
			}
		}
		got = append(got, strings.Join([]string{tc.title, tc.scenario, strings.Join(tc.given, "; "), strings.Join(tc.when, "; "), strings.Join(tc.then, "; "), tc.subject, strings.Join(gaps, "; ")}, " | "))
	}
	want := []string{
		`Keep a note at 10:00 | red | a signed-in writer; an empty board | the writer sends POST /notes with: { "text": "hello" } | it answers 201; the board shows: text; hello | POST /notes | `,
		`Read a note 1 |  | a signed-in writer; a note n1 | the writer reads it at 10:00 | it answers 200 |  | `,
		`Read a note 2 |  | a signed-in writer; a note n9 | the writer reads it at <time> | it answers 404 |  | when the Examples row on line 32 has no column for <time>`,
		`Read a note 3 |  | a signed-in writer; a note n8 | the writer reads it at <time> | it answers <status> |  | when the Examples row on line 33 has no column for <time>; then the Examples row on line 33 leaves <status> blank, which may be an empty value on purpose or one left out`,
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("read\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

// TestTestNamesNeverCollide checks that a test is never written over an
// earlier one: a title that ends in a number, an Outline's numbered rows
// and plain scenarios of the same title each get a name of their own, the
// first number from 2 that is free, in either order.
func TestTestNamesNeverCollide(t *testing.T) {
	for _, c := range []struct {
		titles []string
		want   string
	}{
		{[]string{"Read a note 2", "Read a note", "Read a note"}, "read-a-note-2 read-a-note read-a-note-3"},
		{[]string{"Read a note", "Read a note", "Read a note 2"}, "read-a-note read-a-note-2 read-a-note-2-2"},
		{[]string{"Read a note 1", "Read a note 2", "Read a note", "Read a note"}, "read-a-note-1 read-a-note-2 read-a-note read-a-note-3"},
	} {
		f := &scriptFile{Name: "notes.feature", Key: "notes"}
		var tests []*scriptTest
		for i, title := range c.titles {
			tests = append(tests, &scriptTest{title: title, heading: title, file: f, clause: "line " + string(rune('1'+i)), when: []string{"the writer reads it"}, then: []string{"it answers 200"}})
		}
		res := &Result{Tree: newTree()}
		writeTests(res, t.TempDir(), &Read{Commit: "0000000"}, []*scriptFile{f}, tests)
		var got []string
		seen := map[string]bool{}
		for path := range res.Tree.files {
			if name, ok := strings.CutSuffix(strings.TrimPrefix(path, "tests/"), "/test.yaml"); ok && strings.HasPrefix(path, "tests/") {
				seen[name] = true
			}
		}
		for _, name := range strings.Fields(c.want) {
			if seen[name] {
				got = append(got, name)
			}
		}
		if len(seen) != len(c.titles) || strings.Join(got, " ") != c.want {
			t.Errorf("titles %q: wrote %v, want %s", c.titles, seen, c.want)
		}
	}
}
