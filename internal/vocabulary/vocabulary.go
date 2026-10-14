// Package vocabulary reads a vocabulary: the words of one language that
// extract documents finds a commitment and reads a table of fields by,
// and that merge reads a number of days by (docs/vocabularies.md,
// ADR-105). SpecArch ships English and reads it by default; a project
// writes its own file for another language.
package vocabulary

import (
	"fmt"
	"io/fs"
	"os"
	"regexp"
	"sort"
	"strings"
	"time"

	"go.yaml.in/yaml/v3"

	"github.com/SpecArch/specarch/internal/expr"
	"github.com/SpecArch/specarch/vocabularies"
)

// Suffix ends the name of every vocabulary file.
const Suffix = ".specarch-vocabulary.yaml"

// Tag is the form of a language tag: BCP 47's language subtag and the
// subtags after it, letters and digits, each at most eight characters.
var Tag = regexp.MustCompile(`^[A-Za-z]{2,3}(-[A-Za-z0-9]{1,8})*$`)

// Header names a column of a table of fields, by the key its words are
// listed under.
const (
	Field       = "field"
	Type        = "type"
	Required    = "required"
	Sensitivity = "sensitivity"
	Description = "description"
	RowNumber   = "rowNumber"
)

// The closed sets a vocabulary maps its words to.
var (
	commitmentKeys  = []string{"must", "should"}
	headerKeys      = []string{Field, Type, Required, Sensitivity, Description, RowNumber}
	sensitivityKeys = []string{"public", "internal", "personal", "credential"}
	typeKeys        = []string{"string", "date", "dateTime", "boolean", "email"}
	topKeys         = []string{"specarchVocabulary", "language", "description", "commitments", "frequencies", "timeLimits", "pronouns",
		"linkingVerbs", "leadingWords", "numbers", "units", "timeSeparators", "headers", "yes", "sensitivities", "types"}
)

// Vocabulary is one language's words, each list lower case.
type Vocabulary struct {
	Language       string
	Path           string // the file it was read from, or vocabularies/<file> for a shipped one
	Commitments    map[string][]string
	Frequencies    []string
	TimeLimits     []string
	Pronouns       []string
	LinkingVerbs   []string
	LeadingWords   []string
	Numbers        map[string]int
	Units          map[string]time.Duration
	TimeSeparators []string
	Headers        map[string][]string
	Yes            []string
	Sensitivities  map[string][]string
	Types          map[string][]string
}

// Error is a vocabulary that cannot be used, at its file and line.
type Error struct{ Reason string }

func (e *Error) Error() string { return e.Reason }

// English is the vocabulary read when a document names no language.
func English() *Vocabulary {
	v, err := Shipped("en")
	if err != nil {
		panic(err)
	}
	return v
}

// Shipped is the vocabulary SpecArch ships for a language tag, or nil when
// it ships none.
func Shipped(tag string) (*Vocabulary, error) {
	paths, _ := fs.Glob(vocabularies.Shipped, "*"+Suffix)
	for _, p := range paths {
		if strings.EqualFold(strings.TrimSuffix(p, Suffix), tag) {
			data, err := vocabularies.Shipped.ReadFile(p)
			if err != nil {
				return nil, err
			}
			return Parse(data, "vocabularies/"+p)
		}
	}
	return nil, nil
}

// ShippedTags is the language tags SpecArch ships a vocabulary for.
func ShippedTags() []string {
	paths, _ := fs.Glob(vocabularies.Shipped, "*"+Suffix)
	var out []string
	for _, p := range paths {
		out = append(out, strings.TrimSuffix(p, Suffix))
	}
	sort.Strings(out)
	return out
}

// Read reads and checks a project's vocabulary file. An error of type
// *Error is the file's; any other is the file system's.
func Read(path string) (*Vocabulary, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return Parse(data, path)
}

// Format is the keys a vocabulary file holds, for the message that
// refuses a language with no vocabulary.
const Format = `specarchVocabulary: "0.1", language (the tag), description, commitments (must and should), frequencies, timeLimits, pronouns, linkingVerbs, leadingWords, numbers (word: value), units (word: ISO 8601 duration), timeSeparators, headers (field, type, required, sensitivity, description and rowNumber), yes, sensitivities (public, internal, personal and credential) and types (string, date, dateTime, boolean and email); docs/vocabularies.md has the English file as the example`

// Parse checks a vocabulary: every key present and no other, every word
// given once, and no word in two lists of one group, since that word
// would mean both.
func Parse(data []byte, path string) (*Vocabulary, error) {
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, &Error{fmt.Sprintf("%s does not parse as YAML: %v", path, err)}
	}
	if len(doc.Content) == 0 || doc.Content[0].Kind != yaml.MappingNode {
		return nil, &Error{fmt.Sprintf("%s is not a mapping; a vocabulary holds %s", path, Format)}
	}
	p := &parser{path: path}
	root := doc.Content[0]
	keys := p.mapping(root, "the file", topKeys)
	v := &Vocabulary{Path: path}
	if n := keys["specarchVocabulary"]; n != nil && n.Value != "0.1" {
		p.fail(n, "specarchVocabulary is %q; this build reads 0.1", n.Value)
	}
	if n := keys["language"]; n != nil {
		v.Language = n.Value
		if !Tag.MatchString(n.Value) {
			p.fail(n, "language %q is not a language tag; write it as BCP 47 does, such as en or pt-BR", n.Value)
		}
	}
	v.Commitments = p.group(keys["commitments"], "commitments", commitmentKeys)
	v.Frequencies = p.list(keys["frequencies"], "frequencies")
	v.TimeLimits = p.list(keys["timeLimits"], "timeLimits")
	v.Pronouns = p.list(keys["pronouns"], "pronouns")
	v.LinkingVerbs = p.list(keys["linkingVerbs"], "linkingVerbs")
	v.LeadingWords = p.list(keys["leadingWords"], "leadingWords")
	v.TimeSeparators = p.list(keys["timeSeparators"], "timeSeparators")
	v.Yes = p.list(keys["yes"], "yes")
	v.Headers = p.group(keys["headers"], "headers", headerKeys)
	if n := keys["headers"]; n != nil && len(v.Headers[Field]) == 0 {
		p.fail(n, "headers.field lists no word, so no table would be read as a table of fields")
	}
	v.Sensitivities = p.group(keys["sensitivities"], "sensitivities", sensitivityKeys)
	v.Types = p.group(keys["types"], "types", typeKeys)
	v.Numbers = map[string]int{}
	if n := keys["numbers"]; n != nil {
		for _, pr := range p.pairs(n, "numbers") {
			var i int
			if err := pr[1].Decode(&i); err != nil || i < 0 || pr[1].Tag != "!!int" {
				p.fail(pr[1], "numbers.%s is %q; write the whole number the word stands for", pr[0].Value, pr[1].Value)
				continue
			}
			v.Numbers[strings.ToLower(pr[0].Value)] = i
		}
	}
	v.Units = map[string]time.Duration{}
	if n := keys["units"]; n != nil {
		for _, pr := range p.pairs(n, "units") {
			d, ok := expr.ParseDuration(pr[1].Value)
			if !ok || d == 0 {
				p.fail(pr[1], "units.%s is %q; write the ISO 8601 duration the word stands for, such as P1D or PT1H", pr[0].Value, pr[1].Value)
				continue
			}
			v.Units[strings.ToLower(pr[0].Value)] = d
		}
	}
	if len(p.problems) > 0 {
		return nil, &Error{fmt.Sprintf("%s is not a vocabulary this build reads: %s", path, strings.Join(p.problems, "; "))}
	}
	return v, nil
}

type parser struct {
	path     string
	problems []string
}

func (p *parser) fail(n *yaml.Node, format string, args ...any) {
	p.problems = append(p.problems, fmt.Sprintf("line %d: %s", n.Line, fmt.Sprintf(format, args...)))
}

// mapping is a mapping's values by key: every one of keys must be there,
// and no other.
func (p *parser) mapping(n *yaml.Node, what string, keys []string) map[string]*yaml.Node {
	out := map[string]*yaml.Node{}
	if n == nil {
		return out
	}
	if n.Kind != yaml.MappingNode {
		p.fail(n, "%s is not a mapping; it holds %s", what, strings.Join(keys, ", "))
		return out
	}
	known := map[string]bool{}
	for _, k := range keys {
		known[k] = true
	}
	for i := 0; i+1 < len(n.Content); i += 2 {
		k := n.Content[i]
		if !known[k.Value] {
			p.fail(k, "%s holds %s, which a vocabulary does not have; it holds %s", what, k.Value, strings.Join(keys, ", "))
			continue
		}
		out[k.Value] = n.Content[i+1]
	}
	var missing []string
	for _, k := range keys {
		if out[k] == nil {
			missing = append(missing, k)
		}
	}
	if len(missing) > 0 {
		p.fail(n, "%s has no %s; write each, with an empty list where the language has no such word", what, strings.Join(missing, ", "))
	}
	return out
}

func (p *parser) pairs(n *yaml.Node, what string) [][2]*yaml.Node {
	if n.Kind != yaml.MappingNode {
		p.fail(n, "%s is not a mapping of a word to what it stands for", what)
		return nil
	}
	var out [][2]*yaml.Node
	seen := map[string]bool{}
	for i := 0; i+1 < len(n.Content); i += 2 {
		w := strings.ToLower(n.Content[i].Value)
		if seen[w] {
			p.fail(n.Content[i], "%s lists the word %q twice", what, w)
			continue
		}
		seen[w] = true
		out = append(out, [2]*yaml.Node{n.Content[i], n.Content[i+1]})
	}
	return out
}

// list is a list of words, lower case, each given once.
func (p *parser) list(n *yaml.Node, what string) []string {
	if n == nil {
		return nil
	}
	if n.Kind != yaml.SequenceNode {
		p.fail(n, "%s is not a list of words", what)
		return nil
	}
	out := []string{}
	seen := map[string]bool{}
	for _, item := range n.Content {
		w := strings.ToLower(strings.TrimSpace(item.Value))
		switch {
		case item.Kind != yaml.ScalarNode || w == "":
			p.fail(item, "%s holds an item that is not a word", what)
		case seen[w]:
			p.fail(item, "%s lists the word %q twice", what, w)
		default:
			seen[w] = true
			out = append(out, w)
		}
	}
	return out
}

// group is a mapping of lists under the keys of a closed set; a word in
// two of its lists is refused, at the second.
func (p *parser) group(n *yaml.Node, what string, keys []string) map[string][]string {
	out := map[string][]string{}
	if n == nil {
		return out
	}
	fields := p.mapping(n, what, keys)
	in := map[string]string{}
	for _, k := range keys {
		list := fields[k]
		if list == nil {
			continue
		}
		words := p.list(list, what+"."+k)
		for i, w := range words {
			if other, ok := in[w]; ok {
				p.fail(list.Content[i], "the word %q is under both %s.%s and %s.%s; one word can mean only one of them", w, what, other, what, k)
				continue
			}
			in[w] = k
		}
		out[k] = words
	}
	return out
}

// HeaderWord is the first word a vocabulary gives a column, as a table's
// header writes it, for the messages that name the columns read.
func (v *Vocabulary) HeaderWord(key string) string {
	ws := v.Headers[key]
	if len(ws) == 0 {
		return key
	}
	r := []rune(ws[0])
	return strings.ToUpper(string(r[:1])) + string(r[1:])
}

// CommitmentWords is every commitment word, of every priority.
func (v *Vocabulary) CommitmentWords() []string {
	var out []string
	for _, k := range commitmentKeys {
		out = append(out, v.Commitments[k]...)
	}
	return out
}

// Priority is the priority a commitment word gives: should for a should
// word, must for any other.
func (v *Vocabulary) Priority(word string) string {
	for _, w := range v.Commitments["should"] {
		if strings.EqualFold(w, word) {
			return "should"
		}
	}
	return "must"
}

// Key is the key a word is listed under in a group, or "".
func Key(group map[string][]string, word string) string {
	word = strings.ToLower(strings.TrimSpace(word))
	for k, ws := range group {
		for _, w := range ws {
			if w == word {
				return k
			}
		}
	}
	return ""
}

// Has says whether a list holds a word, in any case.
func Has(list []string, word string) bool {
	word = strings.ToLower(strings.TrimSpace(word))
	for _, w := range list {
		if w == word {
			return true
		}
	}
	return false
}

// NumberWords is the words of numbers.
func (v *Vocabulary) NumberWords() []string {
	var out []string
	for w := range v.Numbers {
		out = append(out, w)
	}
	return out
}

// Words is a pattern that finds any of the words standing alone: not
// inside a longer word, in any case. RE2's \b knows only ASCII letters,
// so the edges are written out.
func Words(words []string) string {
	if len(words) == 0 {
		return `[^\s\S]`
	}
	ws := append([]string(nil), words...)
	sort.Slice(ws, func(i, j int) bool {
		if len(ws[i]) != len(ws[j]) {
			return len(ws[i]) > len(ws[j])
		}
		return ws[i] < ws[j]
	})
	var quoted []string
	for _, w := range ws {
		quoted = append(quoted, regexp.QuoteMeta(w))
	}
	return `(` + strings.Join(quoted, "|") + `)`
}

// Before and After are the edges of a word: the start or end of the text,
// or a character that is not a letter, a digit or an underscore.
const (
	Before = `(?:^|[^\p{L}\p{N}_])`
	After  = `(?:$|[^\p{L}\p{N}_])`
)
