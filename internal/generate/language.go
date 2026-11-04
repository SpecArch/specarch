package generate

import (
	"embed"
	"fmt"
	"io/fs"
	"path"
	"regexp"
	"sort"
	"strings"
	"sync"

	"go.yaml.in/yaml/v3"
)

// The languages a document is written in (ADR-241). English is the text
// of the code itself; every other language is a catalog under
// languages/<tag>/, one file per source file of this package, that gives
// each English text its translation or keeps it as it is.

// DefaultLanguage is the language a document is written in when none is
// named.
const DefaultLanguage = "en"

//go:embed languages
var catalogs embed.FS

// Catalog is one language's texts: the translation of each English text,
// and the English texts kept as they are in that language, such as a
// Mermaid keyword or a term practitioners say in English. Dynamic marks
// the translations of texts the code makes as it runs, such as a section
// title made from its key, which no literal in the code names.
type Catalog struct {
	Language     string
	Translations map[string]string
	Same         map[string]bool
	Dynamic      map[string]bool
}

type catalogFile struct {
	Translations map[string]string `yaml:"translations"`
	Dynamic      map[string]string `yaml:"dynamic"`
	Same         []string          `yaml:"same"`
}

// Languages is the tags a document can be written in, English first.
func Languages() []string {
	out := []string{DefaultLanguage}
	entries, _ := fs.ReadDir(catalogs, "languages")
	var tags []string
	for _, e := range entries {
		if e.IsDir() {
			tags = append(tags, e.Name())
		}
	}
	sort.Strings(tags)
	return append(out, tags...)
}

// LoadCatalog reads the catalog of a language. A text given two
// different translations in two files, or both translated and kept, is
// refused, since it would read one way or the other by the order of the
// files.
func LoadCatalog(tag string) (*Catalog, error) {
	files, err := fs.Glob(catalogs, path.Join("languages", tag, "*.yaml"))
	if err != nil || len(files) == 0 {
		return nil, fmt.Errorf("no catalog for the language %q; the documents are written in %s", tag, strings.Join(Languages(), ", "))
	}
	c := &Catalog{Language: tag, Translations: map[string]string{}, Same: map[string]bool{}, Dynamic: map[string]bool{}}
	from := map[string]string{}
	var problems []string
	for _, f := range files {
		data, err := catalogs.ReadFile(f)
		if err != nil {
			return nil, err
		}
		var cf catalogFile
		if err := yaml.Unmarshal(data, &cf); err != nil {
			return nil, fmt.Errorf("%s does not parse: %v", f, err)
		}
		for k := range cf.Dynamic {
			if _, ok := cf.Translations[k]; ok {
				problems = append(problems, fmt.Sprintf("%q is under both translations and dynamic in %s", k, f))
			}
			c.Dynamic[k] = true
		}
		for k, v := range merge(cf.Translations, cf.Dynamic) {
			if old, ok := c.Translations[k]; ok && old != v {
				problems = append(problems, fmt.Sprintf("%q is translated as %q in %s and as %q in %s", k, old, from[k], v, f))
			}
			c.Translations[k], from[k] = v, f
		}
		for _, k := range cf.Same {
			c.Same[k] = true
		}
	}
	for k := range c.Same {
		if _, ok := c.Translations[k]; ok {
			problems = append(problems, fmt.Sprintf("%q is both translated and kept as it is", k))
		}
	}
	if len(problems) > 0 {
		sort.Strings(problems)
		return nil, fmt.Errorf("the catalog of %s is inconsistent: %s", tag, strings.Join(problems, "; "))
	}
	return c, nil
}

// merge is the entries of two maps.
func merge(a, b map[string]string) map[string]string {
	out := map[string]string{}
	for k, v := range a {
		out[k] = v
	}
	for k, v := range b {
		out[k] = v
	}
	return out
}

var (
	languageMu sync.Mutex
	active     *Catalog
	missed     map[string]bool
)

// UseLanguage writes every document made until the returned function is
// called in the language tag. Documents are made one language at a time:
// a second call waits until the first is released.
func UseLanguage(tag string) (release func(), err error) {
	var c *Catalog
	if tag != "" && tag != DefaultLanguage {
		var err error
		if c, err = LoadCatalog(tag); err != nil {
			return nil, err
		}
	}
	languageMu.Lock()
	active, missed = c, map[string]bool{}
	return func() {
		active = nil
		languageMu.Unlock()
	}, nil
}

// Missed is the English texts the documents made in the last language
// other than English asked for and its catalog does not hold, sorted;
// the test of each language requires none.
func Missed() []string {
	var out []string
	for k := range missed {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// tr is a text of a document in the language in use: the text itself in
// English, or in a language whose catalog keeps it, else its translation.
// A text with no letter but in its verbs, such as a table rule, is the
// same in every language.
func tr(s string) string {
	if active == nil {
		return s
	}
	if t, ok := active.Translations[s]; ok {
		return t
	}
	if !active.Same[s] && HasWords(s) {
		missed[s] = true
	}
	return s
}

// Verb is a fmt verb, with an explicit argument index or not.
var Verb = regexp.MustCompile(`%(\[\d+\])?[-+# 0]*\d*(\.\d+)?[a-zA-Z%]`)

// HasWords says whether a text holds a letter outside its fmt verbs, so
// that a catalog has to give it.
func HasWords(s string) bool {
	s = Verb.ReplaceAllString(s, "")
	return strings.IndexFunc(s, func(r rune) bool { return r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z' }) >= 0
}
