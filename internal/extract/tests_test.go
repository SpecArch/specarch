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
`
	tests, err := readFeature(&scriptFile{Name: "notes.feature", Key: "notes"}, text)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, tc := range tests {
		got = append(got, strings.Join([]string{tc.title, tc.scenario, strings.Join(tc.given, "; "), strings.Join(tc.when, "; "), strings.Join(tc.then, "; "), tc.subject}, " | "))
	}
	want := []string{
		`Keep a note at 10:00 | red | a signed-in writer; an empty board | the writer sends POST /notes with: { "text": "hello" } | it answers 201; the board shows: text; hello | POST /notes`,
		`Read a note 1 |  | a signed-in writer; a note n1 | the writer reads it at 10:00 | it answers 200 | `,
		`Read a note 2 |  | a signed-in writer; a note n9 | the writer reads it at <time> | it answers 404 | `,
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("read\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}
