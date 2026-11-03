package main

import (
	"strings"
	"testing"
)

// TestMapTestCase checks each way a test case's name maps to a test, and
// that a name matching two tests or none maps to nothing, with why.
func TestMapTestCase(t *testing.T) {
	tests := []string{"loan-limit", "loan-2-limit", "loan2-limit", "member-key"}
	for _, c := range []struct{ name, want, why string }{
		{"loan-limit", "loan-limit", ""},
		{"TestLoanLimit", "loan-limit", ""},
		{"testLoanLimit()", "loan-limit", ""},
		{"TestMemberKey/opaque/second", "member-key", ""},
		{"member-key: entity Member, golden scenario", "member-key", ""},
		{"TestLoan2Limit", "", "it maps to loan-2-limit and loan2-limit, more than one test"},
		{"TestRenewal", "", "no test of the specification is named so"},
		{"loanLimit", "", "no test of the specification is named so"},
	} {
		got, why := mapTestCase(c.name, tests)
		if got != c.want || !strings.HasPrefix(why, c.why) {
			t.Errorf("mapTestCase(%q) = %q, %q; want %q, a why starting %q", c.name, got, why, c.want, c.why)
		}
	}
}

// TestReadJUnit checks the testcases are read in the file's order at any
// depth, under a testsuites or a bare testsuite root, with their outcome
// and message, and that another root is refused.
func TestReadJUnit(t *testing.T) {
	cases, err := readJUnit([]byte(`<testsuite name="a">
  <testcase name="one"/>
  <testsuite name="b">
    <testcase name="two"><error message="">panic: nil map
at line 3</error></testcase>
  </testsuite>
  <testcase name="three"><failure message="  want 1,
 got 2 ">detail</failure></testcase>
  <testcase name="four"><skipped/></testcase>
</testsuite>`))
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, c := range cases {
		got = append(got, c.name+"="+c.outcome+"="+c.message)
	}
	want := "one=passed= two=error=panic: nil map three=failed=want 1, got 2 four=skipped="
	if strings.Join(got, " ") != want {
		t.Errorf("readJUnit read\n  %s\nwant\n  %s", strings.Join(got, " "), want)
	}
	if _, err := readJUnit([]byte(`<report><testcase name="x"/></report>`)); err == nil || !strings.Contains(err.Error(), "its root element is report") {
		t.Errorf("a report root was read, or refused with %v", err)
	}
	if _, err := readJUnit([]byte(`<testsuites><testcase name="x">`)); err == nil {
		t.Error("XML cut short was read")
	}
}
