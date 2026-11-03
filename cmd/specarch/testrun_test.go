package main

import (
	"strings"
	"testing"

	"github.com/SpecArch/specarch/internal/gentests"
)

// TestMapTestCase checks each way a test case's name maps to a test or to
// an algorithm's or a call mapping's worked examples, and that a name
// matching two of them or none maps to nothing, with why.
func TestMapTestCase(t *testing.T) {
	tests := []string{"loan-limit", "loan-2-limit", "loan2-limit", "member-key"}
	worked := []gentests.Worked{
		{Functions: []string{"TestAlgorithmLateFee", "testAlgorithmLateFee"}, Display: "algorithm late-fee", Element: "#/algorithms/late-fee"},
		{Functions: []string{"TestAlgorithmMayLend", "testAlgorithmMayLend"}, Display: "algorithm may-lend", Element: "#/algorithms/may-lend"},
		{Functions: []string{"TestMappingReturnLoanFeeLedgerPostCharge", "testMappingReturnLoanFeeLedgerPostChargeSent", "testMappingReturnLoanFeeLedgerPostChargeKept"},
			Display: "mapping of returnLoan's call to feeLedger postCharge", Element: "#/paths/~1loans~1{loanId}~1return"},
	}
	for _, c := range []struct{ name, test, element, why string }{
		{"loan-limit", "loan-limit", "", ""},
		{"TestLoanLimit", "loan-limit", "", ""},
		{"testLoanLimit()", "loan-limit", "", ""},
		{"TestMemberKey/opaque/second", "member-key", "", ""},
		{"member-key: entity Member, golden scenario", "member-key", "", ""},
		{"TestAlgorithmLateFee/returned on time", "", "#/algorithms/late-fee", ""},
		{"TestAlgorithmMayLend", "", "#/algorithms/may-lend", ""},
		{"testAlgorithmLateFee(example:input:expected:)", "", "#/algorithms/late-fee", ""},
		{"algorithm late-fee: worked examples", "", "#/algorithms/late-fee", ""},
		{"algorithm late-fee returned on time", "", "#/algorithms/late-fee", ""},
		{"TestMappingReturnLoanFeeLedgerPostCharge/a late return", "", "#/paths/~1loans~1{loanId}~1return", ""},
		{"testMappingReturnLoanFeeLedgerPostChargeKept(example:answer:kept:)", "", "#/paths/~1loans~1{loanId}~1return", ""},
		{"mapping of returnLoan's call to feeLedger postCharge: what is sent", "", "#/paths/~1loans~1{loanId}~1return", ""},
		{"algorithm late-feed", "", "", "no test of the specification is named so"},
		{"TestAlgorithmLateFee2", "", "", "no test of the specification is named so"},
		{"TestLoan2Limit", "", "", "it maps to loan-2-limit and loan2-limit, more than one test"},
		{"TestAlgorithmLateFee()", "", "#/algorithms/late-fee", ""},
		{"TestRenewal", "", "", "no test of the specification is named so"},
		{"loanLimit", "", "", "no test of the specification is named so"},
	} {
		test, element, why := mapTestCase(c.name, tests, worked)
		if test != c.test || element != c.element || !strings.HasPrefix(why, c.why) {
			t.Errorf("mapTestCase(%q) = %q, %q, %q; want %q, %q, a why starting %q", c.name, test, element, why, c.test, c.element, c.why)
		}
	}
	// A spec test whose Pascal name is a generated algorithm test's name
	// is two matches, so neither is taken.
	clash := []string{"algorithm-late-fee"}
	_, _, why := mapTestCase("TestAlgorithmLateFee", clash, worked)
	if why != "it maps to algorithm-late-fee and the worked examples of algorithm late-fee, more than one test or worked example" {
		t.Errorf("a name matching a test and an algorithm's examples gave %q", why)
	}
	if test, _, why := mapTestCase("algorithm-late-fee: its own display name", clash, worked); test != "algorithm-late-fee" || why != "" {
		t.Errorf("the test's own display name gave %q, %q", test, why)
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
