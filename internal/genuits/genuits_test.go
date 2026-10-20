package genuits

import (
	"io/fs"
	"regexp"
	"testing"
)

// Every key of the screens' own texts a component names has a text, so
// no screen shows a key in place of words.
func TestScreenStringsCoverComponents(t *testing.T) {
	used := regexp.MustCompile(`"(screens\.[A-Za-z.]+)"`)
	err := fs.WalkDir(screens, "screens", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		data, _ := screens.ReadFile(path)
		for _, m := range used.FindAllStringSubmatch(string(data), -1) {
			if m[1] == "screens.notice" {
				continue // the key Notice keeps a message under in the session's storage, not a text
			}
			if _, ok := screenStrings[m[1]]; !ok {
				t.Errorf("%s names %s, which has no text in screenStrings", path, m[1])
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
