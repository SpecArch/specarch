package canonical

import (
	"encoding/json"
	"testing"
)

// The numbers of RFC 8785's appendix B and the edges of ECMAScript's
// Number::toString.
func TestNumbers(t *testing.T) {
	for in, want := range map[string]string{
		"0":                      "0",
		"-0":                     "0",
		"1":                      "1",
		"-1.5":                   "-1.5",
		"100":                    "100",
		"1e20":                   "100000000000000000000",
		"1e21":                   "1e+21",
		"0.000001":               "0.000001",
		"1e-7":                   "1e-7",
		"123456789012345680000":  "123456789012345680000",
		"9007199254740993":       "9007199254740992",
		"4.35":                   "4.35",
		"0.002":                  "0.002",
		"1.7976931348623157e308": "1.7976931348623157e+308",
		"5e-324":                 "5e-324",
		"333333333.3333333":      "333333333.3333333",
	} {
		got, err := Marshal(json.Number(in))
		if err != nil {
			t.Fatalf("%s: %v", in, err)
		}
		if string(got) != want {
			t.Errorf("%s: got %s, want %s", in, got, want)
		}
	}
}

func TestObject(t *testing.T) {
	v := map[string]any{
		"b": []any{true, nil, "x\n\"\\\u0001é/"},
		"a": map[string]any{"\U0001F600": 1, "דּ": 2},
		"":  "",
	}
	got, err := Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	// U+1F600 is the surrogate pair D83D DE00, which sorts before FB33.
	want := `{"":"","a":{"` + "\U0001F600" + `":1,"` + "דּ" + `":2},"b":[true,null,"x\n\"\\\u0001é/"]}`
	if string(got) != want {
		t.Errorf("got  %s\nwant %s", got, want)
	}
}
