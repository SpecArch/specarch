package source

import "testing"

// TestInt checks that Int reads an integer as the schema check does, by
// its tag, whatever base or underscores it is written in.
func TestInt(t *testing.T) {
	for _, c := range []struct {
		text string
		want int64
		ok   bool
	}{
		{"100", 100, true},
		{"0x64", 100, true},
		{"0X64", 100, true},
		{"0o144", 100, true},
		{"0b1100100", 100, true},
		{"1_00", 100, true},
		{"0144", 100, true},
		{"-0x64", -100, true},
		{"+1_00", 100, true},
		{"\"100\"", 0, false},
		{"!!str 100", 0, false},
		{"1.5", 0, false},
		{"9223372036854775808", 0, false},
		{"abc", 0, false},
	} {
		doc := Parse([]byte("v: " + c.text + "\n"))
		got, ok := Int(Child(doc.Root, "v"))
		if got != c.want || ok != c.ok {
			t.Errorf("Int(%s) = %d, %v; want %d, %v", c.text, got, ok, c.want, c.ok)
		}
		text := IntText(Child(doc.Root, "v"))
		if c.ok && text != "100" && text != "-100" {
			t.Errorf("IntText(%s) = %q; want it in decimal", c.text, text)
		}
	}
	if _, ok := Int(nil); ok {
		t.Errorf("Int(nil) is an integer")
	}
}
