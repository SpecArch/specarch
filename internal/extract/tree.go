package extract

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"go.yaml.in/yaml/v3"
)

// The schema line of every root file a reader writes.
const schemaLine = "# yaml-language-server: $schema=https://raw.githubusercontent.com/SpecArch/specarch/v0.4.0/schema/specarch-design-0.1.schema.json\n"

// Node builders. A tree is built from yaml.Node values, never from Go
// maps, so its keys keep the order they were added in.

func mapping(pairs ...any) *yaml.Node {
	m := &yaml.Node{Kind: yaml.MappingNode}
	for i := 0; i+1 < len(pairs); i += 2 {
		set(m, pairs[i].(string), pairs[i+1])
	}
	return m
}

// set adds a key to a mapping; a nil value leaves it out.
func set(m *yaml.Node, key string, v any) {
	n := value(v)
	if n == nil {
		return
	}
	m.Content = append(m.Content, str(key), n)
}

func value(v any) *yaml.Node {
	switch v := v.(type) {
	case nil:
		return nil
	case *yaml.Node:
		return v
	case string:
		return str(v)
	case int:
		return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!int", Value: strconv.Itoa(v)}
	case int64:
		return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!int", Value: strconv.FormatInt(v, 10)}
	case bool:
		return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!bool", Value: strconv.FormatBool(v)}
	case []string:
		s := &yaml.Node{Kind: yaml.SequenceNode, Style: yaml.FlowStyle}
		for _, x := range v {
			s.Content = append(s.Content, str(x))
		}
		return s
	case []*yaml.Node:
		s := &yaml.Node{Kind: yaml.SequenceNode}
		s.Content = v
		return s
	}
	panic(fmt.Sprintf("extract: no YAML node for %T", v))
}

func str(s string) *yaml.Node {
	return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: s}
}

// literal is a number written as it was read, such as a default of 0.50.
func literal(tag, s string) *yaml.Node {
	return &yaml.Node{Kind: yaml.ScalarNode, Tag: tag, Value: s}
}

func flow(n *yaml.Node) *yaml.Node {
	n.Style = yaml.FlowStyle
	return n
}

// citation is one entry of cites, written on one line.
func citation(source, clause, says string) *yaml.Node {
	return flow(mapping("source", source, "clause", clause, "says", says))
}

// Tree is a specification tree being written: its files by their path from
// the tree's root.
type Tree struct {
	files map[string]*yaml.Node
}

func newTree() *Tree { return &Tree{files: map[string]*yaml.Node{}} }

func (t *Tree) put(path string, n *yaml.Node) { t.files[path] = n }

// Write writes the tree into out, creating the folder. It writes only its
// own files; it never removes one.
func (t *Tree) Write(out string) error {
	names := make([]string, 0, len(t.files))
	for name := range t.files {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		text, err := encode(t.files[name])
		if err != nil {
			return err
		}
		if name == "specarch.yaml" {
			text = append([]byte(schemaLine), text...)
		}
		p := filepath.Join(out, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(p, text, 0o644); err != nil {
			return err
		}
	}
	return nil
}

// Files is the tree's file names, sorted.
func (t *Tree) Files() []string {
	var names []string
	for name := range t.files {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func encode(n *yaml.Node) ([]byte, error) {
	var b bytes.Buffer
	enc := yaml.NewEncoder(&b)
	enc.SetIndent(2)
	if err := enc.Encode(n); err != nil {
		return nil, err
	}
	if err := enc.Close(); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

// The root of every tree a reader writes: tracking origin, with the one
// code source read.
func rootFile(title, description string, stages []string, sources *yaml.Node) *yaml.Node {
	info := mapping("title", title, "version", "0.1.0", "description", block(description), "tracksOrigin", true)
	root := mapping("specarch", "0.1", "info", info)
	if len(stages) > 0 {
		set(root, "stages", stages)
	}
	set(root, "sources", sources)
	return root
}

// block is a text written as a YAML literal block.
func block(s string) *yaml.Node {
	n := str(s)
	n.Style = yaml.LiteralStyle
	return n
}

// codeSource is the source entry of the repository read.
func codeSource(r *Read, out string, clauses []*yaml.Node) (*yaml.Node, error) {
	url, err := relativeURL(out, r.Repository.Root)
	if err != nil {
		return nil, err
	}
	return mapping(
		"kind", "code",
		"title", "The code repository",
		"edition", r.Commit,
		"url", url,
		"clauses", clauses,
	), nil
}

// relativeURL is the repository's folder as seen from the tree's root.
func relativeURL(out, repo string) (string, error) {
	abs, err := filepath.Abs(out)
	if err != nil {
		return "", err
	}
	// out may not exist yet: resolve the nearest folder that does.
	resolved, rest := abs, ""
	for {
		if r, err := filepath.EvalSymlinks(resolved); err == nil {
			resolved = filepath.Join(r, rest)
			break
		}
		parent := filepath.Dir(resolved)
		if parent == resolved {
			break
		}
		rest = filepath.Join(filepath.Base(resolved), rest)
		resolved = parent
	}
	rel, err := filepath.Rel(resolved, repo)
	if err != nil {
		return "", err
	}
	return filepath.ToSlash(rel), nil
}

// The stakeholder a tree with a question names: the reader cannot know who
// decides, so the reviewer renames it to the real role.
const owner = "system-owner"

func ownerStakeholder() *yaml.Node {
	return mapping(owner, mapping(
		"description", "Whoever owns the system that was read and answers for it; the reviewer replaces this with the real role.",
		"origin", "inferred",
		"why", "The surface names no one, and every question needs a stakeholder to decide it.",
	))
}

// kebab writes a name in kebab-case for a file name: loan_items and
// LoanItems both become loan-items.
func kebab(s string) string {
	var b strings.Builder
	for i, r := range s {
		switch {
		case r == '_' || r == ' ' || r == '.' || r == '-':
			if b.Len() > 0 && !strings.HasSuffix(b.String(), "-") {
				b.WriteByte('-')
			}
		case r >= 'A' && r <= 'Z':
			if i > 0 && b.Len() > 0 && !strings.HasSuffix(b.String(), "-") {
				b.WriteByte('-')
			}
			b.WriteRune(r + ('a' - 'A'))
		default:
			b.WriteRune(r)
		}
	}
	return strings.Trim(b.String(), "-")
}
