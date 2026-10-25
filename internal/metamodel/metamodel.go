// Package metamodel reads the version line of a SpecArch file: which rules
// the file is written against, and so which schema checks it
// (docs/meta-model-1.0.0.md). A version is read by its major and minor;
// the patch never changes what is valid or what a file means.
package metamodel

import (
	"maps"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/SpecArch/specarch/internal/source"
)

// Current is the newest version of every SpecArch format, the one
// specarch upgrade writes.
const Current = "1.0.0"

// Format is one kind of SpecArch file and the versions of it a program
// reads.
type Format struct {
	// Key is the key of the version line, such as specarch.
	Key string
	// Name names the format in a message.
	Name string
	// Silent is a format whose earlier versions are read with no
	// warning, since nothing reports a warning for it: a vocabulary,
	// which extract and merge read and which is refused or taken whole.
	Silent bool
	// Earlier are the versions before 1.0.0 read beside it, each with a
	// warning, oldest first.
	Earlier []string
}

// The formats, each by the key of its version line. The specification's is
// the meta-model; the others are as old as 0.1, the same in shape as 1.0.0.
var (
	Design         = Format{Key: "specarch", Name: "meta-model", Earlier: []string{"0.1", "0.2", "0.3"}}
	Implementation = Format{Key: "specarchImplementation", Name: "implementation file", Earlier: []string{"0.1"}}
	Record         = Format{Key: "specarchRecord", Name: "record", Earlier: []string{"0.1"}}
	Idiom          = Format{Key: "specarchIdiom", Name: "idiom", Earlier: []string{"0.1"}}
	Estate         = Format{Key: "specarchEstate", Name: "estate", Earlier: []string{"0.1"}}
	Vocabulary     = Format{Key: "specarchVocabulary", Name: "vocabulary", Earlier: []string{"0.1"}, Silent: true}
)

// current matches every patch of the current minor: one schema per major
// and minor, as OpenAPI publishes one per minor.
var current = regexp.MustCompile(`^1\.0\.(0|[1-9][0-9]*)$`)

// numeric is a version written in numbers alone, in up to three parts.
var numeric = regexp.MustCompile(`^(0|[1-9][0-9]*)(\.(0|[1-9][0-9]*)){0,2}$`)

// Class is what a version line says to this program.
type Class int

const (
	// Known is the current minor, any patch.
	Known Class = iota
	// Earlier is a version before 1.0.0 still read, with a warning.
	Earlier
	// Later is a three-part version after the current minor, which a
	// later program reads.
	Later
	// Short is a version of 1 or more written in fewer than three parts.
	Short
	// Unknown is anything else: no version this or any program reads.
	Unknown
)

// Classify says what a version line's value is.
func (f Format) Classify(v string) Class {
	if slices.Contains(f.Earlier, v) {
		return Earlier
	}
	if current.MatchString(v) {
		return Known
	}
	if !numeric.MatchString(v) {
		return Unknown
	}
	parts := strings.Split(v, ".")
	major, _ := strconv.Atoi(parts[0])
	if major < 1 {
		return Unknown
	}
	if len(parts) < 3 {
		return Short
	}
	return Later
}

// Reads is the version a file is read as: its own when this program reads
// it, otherwise the current one, so that every other problem in the file
// is still listed under the newest rules.
func (f Format) Reads(v string) string {
	switch f.Classify(v) {
	case Known, Earlier:
		return v
	}
	return Current
}

// IsEarlier reports whether a file whose line says v is read as a version
// before 1.0.0, which the earlier schema checks.
func (f Format) IsEarlier(v string) bool {
	return f.Classify(v) == Earlier
}

// AtLeast reports whether version v, read as its major and minor, is
// major.minor or later. A version that is not one is never at least
// anything.
func AtLeast(v string, major, minor int) bool {
	parts := strings.Split(v, ".")
	if len(parts) < 2 {
		return false
	}
	ma, err1 := strconv.Atoi(parts[0])
	mi, err2 := strconv.Atoi(parts[1])
	if err1 != nil || err2 != nil {
		return false
	}
	return ma > major || ma == major && mi >= minor
}

// Reading names the versions a format is read in, for a message: "1.0.0
// and any later 1.0 patch, and 0.3, 0.2 and 0.1 with a warning".
func (f Format) Reading() string {
	s := Current + " and any later 1.0 patch"
	if len(f.Earlier) == 0 {
		return s
	}
	earlier := make([]string, 0, len(f.Earlier))
	for i := len(f.Earlier) - 1; i >= 0; i-- {
		earlier = append(earlier, f.Earlier[i])
	}
	s += ", and " + joinAnd(earlier)
	if !f.Silent {
		s += " with a warning"
	}
	return s
}

func joinAnd(items []string) string {
	if len(items) == 1 {
		return items[0]
	}
	return strings.Join(items[:len(items)-1], ", ") + " and " + items[len(items)-1]
}

// Problem is the message for a version line this program does not read,
// or "" for one it reads. The file is then read as Current.
func (f Format) Problem(v string) string {
	tail := "; this program reads " + f.Reading() + ", and reads the file as " + Current
	switch f.Classify(v) {
	case Later:
		return strconv.Quote(v) + " is a later version than this program reads, which a later specarch reads" + tail
	case Short:
		return strconv.Quote(v) + " is written in fewer than three parts; write the version as major.minor.patch, such as " + strconv.Quote(Current) + tail
	case Unknown:
		return strconv.Quote(v) + " is not a version of this format" + tail
	}
	return ""
}

// The keys meta-model 1.0.0 renamed, each still read under its old
// spelling with a meta_model_deprecated warning until 2.0.0.
const (
	RowPoliciesKey       = "rowPolicies"
	RowPoliciesOldKey    = "rows"
	SourceDeliveryKey    = "sourceDelivery"
	SourceDeliveryOldKey = "source"
)

// RowPolicies is a role's row policies and the key the file gives them
// under: rowPolicies, or rows, its spelling before 1.0.0. When a role gives
// both, rowPolicies is read and the validator reports rows.
func RowPolicies(role *yaml.Node) (string, *yaml.Node) {
	return renamed(role, RowPoliciesKey, RowPoliciesOldKey)
}

// SourceDelivery is how the source of a release reaches the customer, under
// the handover's sourceDelivery or source, its spelling before 1.0.0.
func SourceDelivery(handover *yaml.Node) (string, *yaml.Node) {
	return renamed(handover, SourceDeliveryKey, SourceDeliveryOldKey)
}

func renamed(n *yaml.Node, key, old string) (string, *yaml.Node) {
	if v := source.Child(n, key); v != nil {
		return key, v
	}
	if v := source.Child(n, old); v != nil {
		return old, v
	}
	return key, nil
}

// Normalized gives a specification's value as a plug-in reads it: in the
// shape of the current version, every renamed key under its new name, so a
// plug-in reads one shape whichever version the file declares. The maps it
// changes are copies; v is left as it was.
func Normalized(v any) any {
	root, ok := v.(map[string]any)
	if !ok {
		return v
	}
	return NormalizedMap(root)
}

// NormalizedMap is Normalized for a specification's content as a map; nil
// stays nil.
func NormalizedMap(root map[string]any) map[string]any {
	if root == nil {
		return nil
	}
	out := root
	copied := false
	set := func(k string, x any) {
		if !copied {
			out = maps.Clone(root)
			copied = true
		}
		out[k] = x
	}
	if roles, ok := root["roles"].(map[string]any); ok {
		changed := false
		nr := make(map[string]any, len(roles))
		for name, r := range roles {
			nr[name] = r
			if role, ok := r.(map[string]any); ok {
				if x, ok := rename(role, RowPoliciesKey, RowPoliciesOldKey); ok {
					nr[name] = x
					changed = true
				}
			}
		}
		if changed {
			set("roles", nr)
		}
	}
	if h, ok := root["handover"].(map[string]any); ok {
		if x, ok := rename(h, SourceDeliveryKey, SourceDeliveryOldKey); ok {
			set("handover", x)
		}
	}
	return out
}

// rename gives a copy of m with old moved to key, or false when m has no
// old key. A map that gives both keeps key, and old is dropped.
func rename(m map[string]any, key, old string) (map[string]any, bool) {
	ov, ok := m[old]
	if !ok {
		return nil, false
	}
	out := make(map[string]any, len(m))
	for k, v := range m {
		if k != old {
			out[k] = v
		}
	}
	if _, both := m[key]; !both {
		out[key] = ov
	}
	return out, true
}

// FilePath gives a pointer into the current version's shape, as a plug-in
// writes it, as a pointer into the file: a renamed key under the spelling
// the file gives it, so that a problem a plug-in reports is placed at the
// file's own line.
func FilePath(root *yaml.Node, path string) string {
	tokens := strings.Split(path, "/")
	switch {
	case len(tokens) > 3 && tokens[1] == "roles" && tokens[3] == RowPoliciesKey:
		role := source.Child(source.Child(root, "roles"), source.UnescapeToken(tokens[2]))
		tokens[3], _ = RowPolicies(role)
	case len(tokens) > 2 && tokens[1] == "handover" && tokens[2] == SourceDeliveryKey:
		tokens[2], _ = SourceDelivery(source.Child(root, "handover"))
	default:
		return path
	}
	return strings.Join(tokens, "/")
}
