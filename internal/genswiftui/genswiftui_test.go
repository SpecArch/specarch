package genswiftui

import (
	"io/fs"
	"regexp"
	"testing"

	"github.com/SpecArch/specarch/internal/genscreens"
)

// Every key of the screens' own texts a component names has a text, so
// no screen shows a key in place of words.
func TestScreenStringsCoverComponents(t *testing.T) {
	key := regexp.MustCompile(`"(screens\.[A-Za-z.]+)"`)
	err := fs.WalkDir(screens, "screens", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		data, err := screens.ReadFile(path)
		if err != nil {
			return err
		}
		for _, m := range key.FindAllStringSubmatch(string(data), -1) {
			if _, ok := genscreens.ScreenStrings[m[1]]; !ok {
				t.Errorf("%s names %s, which the screens' strings do not hold", path, m[1])
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// Names are written as Swift writes them: a page as a type and a case,
// a keyword in backticks.
func TestNames(t *testing.T) {
	for _, c := range []struct{ in, pascal, camel string }{
		{"sign-in", "SignIn", "signIn"},
		{"members-list", "MembersList", "membersList"},
		{"memberId", "MemberId", "memberId"},
		{"default", "Default", "`default`"},
	} {
		if p := pascalCase(c.in); p != c.pascal {
			t.Errorf("pascalCase(%q) = %q, want %q", c.in, p, c.pascal)
		}
		if p := camelCase(c.in); p != c.camel {
			t.Errorf("camelCase(%q) = %q, want %q", c.in, p, c.camel)
		}
	}
}
