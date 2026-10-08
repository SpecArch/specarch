// Package diff compares two versions of a specification element by
// element and classifies each difference by the version step it needs,
// as docs/maintenance.md (The diff verb) describes.
package diff

import (
	"encoding/json"
	"fmt"
	"reflect"
	"slices"
	"sort"
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/SpecArch/specarch/internal/semver"
	"github.com/SpecArch/specarch/internal/source"
)

// Change is one element that differs between the two versions.
type Change struct {
	Pointer string // the element, such as /entities/Loan
	What    string // added, changed or removed
	Impact  int    // semver.Patch, semver.Minor or semver.Major
	Reason  string // for a changed element of the public interface, the change that decided its impact
}

// String is the line the diff verb prints.
func (c Change) String() string {
	line := fmt.Sprintf("%s %s #%s", semver.StepNames[c.Impact], c.What, c.Pointer)
	if c.Reason != "" {
		line += ": " + c.Reason
	}
	return line
}

// notCompared are the root keys that are not elements: the version always
// changes, and the stages are layout.
var notCompared = map[string]bool{"specarch": true, "info": true, "stages": true}

// singleSections are the sections that are one object.
var singleSections = map[string]bool{"release": true, "rollback": true, "signoff": true}

var methods = []string{"get", "post", "put", "patch", "delete"}

// publicSections hold elements that are all on the public interface.
var publicSections = map[string]bool{"commands": true, "channels": true, "configuration": true}

// descriptive keys explain an element and never change how a client
// calls it.
var descriptive = map[string]bool{
	"description": true, "summary": true, "title": true, "why": true, "cites": true, "examples": true,
	"satisfies": true, "verifies": true, "origin": true, "decidedIn": true,
}

func isDescriptive(key string) bool { return descriptive[key] || strings.HasPrefix(key, "x-") }

// elements lists the elements of a merged specification by pointer.
func elements(root *yaml.Node) map[string]*yaml.Node {
	out := map[string]*yaml.Node{}
	for _, sec := range source.Pairs(root) {
		key := sec.Key.Value
		switch {
		case notCompared[key]:
		case key == "paths":
			for _, p := range source.Pairs(sec.Value) {
				rest := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
				for _, kv := range source.Pairs(p.Value) {
					if isMethod(kv.Key.Value) {
						out[source.Pointer("paths", p.Key.Value, kv.Key.Value)] = kv.Value
					} else {
						rest.Content = append(rest.Content, kv.Key, kv.Value)
					}
				}
				if len(rest.Content) > 0 {
					out[source.Pointer("paths", p.Key.Value)] = rest
				}
			}
		case singleSections[key] || source.Deref(sec.Value).Kind != yaml.MappingNode:
			out[source.Pointer(key)] = sec.Value
		default:
			for _, e := range source.Pairs(sec.Value) {
				out[source.Pointer(key, e.Key.Value)] = e.Value
			}
		}
	}
	return out
}

func isMethod(k string) bool { return slices.Contains(methods, k) }

// public lists the elements on the public interface: every operation,
// command, channel and setting, every page (whose route alone is public),
// and every entity and enum one of them reaches through $ref.
func public(els map[string]*yaml.Node) map[string]bool {
	out := map[string]bool{}
	var queue []string
	for ptr := range els {
		tokens := tokensOf(ptr)
		if tokens[0] == "paths" || publicSections[tokens[0]] && len(tokens) == 2 {
			out[ptr] = true
			queue = append(queue, ptr)
		}
		if tokens[0] == "pages" && len(tokens) == 2 {
			out[ptr] = true
		}
	}
	for len(queue) > 0 {
		ptr := queue[0]
		queue = queue[1:]
		walkRefs(els[ptr], func(ref string) {
			target := strings.TrimPrefix(ref, "#")
			if els[target] != nil && !out[target] {
				out[target] = true
				queue = append(queue, target)
			}
		})
	}
	return out
}

func walkRefs(n *yaml.Node, fn func(string)) {
	n = source.Deref(n)
	if n == nil {
		return
	}
	switch n.Kind {
	case yaml.MappingNode:
		for _, p := range source.Pairs(n) {
			if p.Key.Value == "$ref" && source.IsScalar(p.Value) {
				if v := p.Value.Value; strings.HasPrefix(v, "#/entities/") || strings.HasPrefix(v, "#/enums/") {
					fn(v)
				}
				continue
			}
			walkRefs(p.Value, fn)
		}
	case yaml.SequenceNode:
		for _, item := range source.Items(n) {
			walkRefs(item, fn)
		}
	}
}

func tokensOf(ptr string) []string {
	var out []string
	for _, t := range strings.Split(strings.TrimPrefix(ptr, "/"), "/") {
		out = append(out, source.UnescapeToken(t))
	}
	return out
}

// Compare lists the elements that differ between two merged
// specifications, in pointer order.
func Compare(oldRoot, newRoot *yaml.Node) []Change {
	oldEls, newEls := elements(oldRoot), elements(newRoot)
	oldPublic, newPublic := public(oldEls), public(newEls)
	var ptrs []string
	for p := range oldEls {
		ptrs = append(ptrs, p)
	}
	for p := range newEls {
		if oldEls[p] == nil {
			ptrs = append(ptrs, p)
		}
	}
	sort.Strings(ptrs)
	var out []Change
	for _, p := range ptrs {
		a, b := oldEls[p], newEls[p]
		isPublic := oldPublic[p] || newPublic[p]
		switch {
		case a == nil:
			out = append(out, Change{Pointer: p, What: "added", Impact: pick(isPublic, semver.Minor)})
		case b == nil:
			out = append(out, Change{Pointer: p, What: "removed", Impact: pick(isPublic, semver.Major)})
		default:
			av, bv := source.ValueOf(a), source.ValueOf(b)
			if reflect.DeepEqual(av, bv) {
				continue
			}
			c := Change{Pointer: p, What: "changed", Impact: semver.Patch}
			if isPublic {
				k := &classifier{best: semver.Patch}
				if tokensOf(p)[0] == "pages" {
					am, _ := av.(map[string]any)
					bm, _ := bv.(map[string]any)
					k.walk(am["route"], bm["route"], []string{"route"})
				} else {
					k.walk(av, bv, nil)
				}
				c.Impact = k.best
				if k.best > semver.Patch {
					c.Reason = k.reason
				}
			}
			out = append(out, c)
		}
	}
	return out
}

func pick(isPublic bool, impact int) int {
	if isPublic {
		return impact
	}
	return semver.Patch
}

// classifier finds the largest impact among the changes inside one
// element, and the first change that has it.
type classifier struct {
	best   int
	reason string
}

func (k *classifier) record(impact int, path []string, format string, args ...any) {
	if impact > k.best {
		k.best = impact
		k.reason = strings.TrimPrefix(strings.Join(path, "/")+" "+fmt.Sprintf(format, args...), " ")
	}
}

func (k *classifier) walk(a, b any, path []string) {
	if reflect.DeepEqual(a, b) {
		return
	}
	key := ""
	if len(path) > 0 {
		key = path[len(path)-1]
	}
	if isDescriptive(key) {
		return // patch, which every changed element already has
	}
	am, aIsMap := a.(map[string]any)
	bm, bIsMap := b.(map[string]any)
	al, aIsList := a.([]any)
	bl, bIsList := b.([]any)
	switch {
	case a == nil && b != nil && len(path) > 0:
		k.addedKey(key, b, path)
	case b == nil && a != nil && len(path) > 0:
		k.record(semver.Major, path, "is removed")
	case aIsMap && bIsMap:
		for _, name := range sortedKeys(am, bm) {
			av, inA := am[name]
			bv, inB := bm[name]
			sub := append(append([]string{}, path...), name)
			switch {
			case !inB:
				if !isDescriptive(name) {
					k.record(semver.Major, sub, "is removed")
				}
			case !inA:
				if !isDescriptive(name) {
					k.addedKey(name, bv, sub)
				}
			default:
				k.walk(av, bv, sub)
			}
		}
	case aIsList && bIsList && (key == "enum" || key == "required"):
		lost, gained := setDiff(al, bl), setDiff(bl, al)
		path = path[:len(path)-1] // the reason says what the list's owner allows or needs
		if key == "enum" {
			for _, v := range lost {
				k.record(semver.Major, path, "loses the value %s", display(v))
			}
			for _, v := range gained {
				k.record(semver.Minor, path, "gains the value %s", display(v))
			}
			return
		}
		for _, v := range gained {
			k.record(semver.Major, path, "now requires %s", display(v))
		}
		for _, v := range lost {
			k.record(semver.Minor, path, "no longer requires %s", display(v))
		}
	case aIsList && bIsList && named(al) && named(bl):
		an, bn := byName(al), byName(bl)
		for _, name := range sortedKeys(an, bn) {
			av, inA := an[name]
			bv, inB := bn[name]
			sub := append(append([]string{}, path...), name)
			switch {
			case !inB:
				k.record(semver.Major, path, "loses %s", name)
			case !inA:
				if bv["required"] == true {
					k.record(semver.Major, path, "gains %s, which is required", name)
				} else {
					k.record(semver.Minor, path, "gains %s", name)
				}
			default:
				k.walk(av, bv, sub)
			}
		}
	case !aIsMap && !bIsMap && !aIsList && !bIsList:
		k.record(semver.Major, path, "changes from %s to %s", display(a), display(b))
	default:
		k.record(semver.Major, path, "changes")
	}
}

// addedKey records a key the new version adds: a requirement is major,
// anything else minor.
func (k *classifier) addedKey(name string, v any, path []string) {
	if name == "required" && v != false {
		k.record(semver.Major, path, "is added")
		return
	}
	k.record(semver.Minor, path, "is added")
}

func sortedKeys[V any](a, b map[string]V) []string {
	seen := map[string]bool{}
	var out []string
	for _, m := range []map[string]V{a, b} {
		for key := range m {
			if !seen[key] {
				seen[key] = true
				out = append(out, key)
			}
		}
	}
	sort.Strings(out)
	return out
}

// setDiff lists the values of a that b does not hold, in a's order.
func setDiff(a, b []any) []any {
	var out []any
	for _, x := range a {
		found := false
		for _, y := range b {
			if reflect.DeepEqual(x, y) {
				found = true
				break
			}
		}
		if !found {
			out = append(out, x)
		}
	}
	return out
}

// named reports whether every item is a mapping with a text name.
func named(l []any) bool {
	if len(l) == 0 {
		return false
	}
	for _, item := range l {
		m, ok := item.(map[string]any)
		if !ok {
			return false
		}
		if _, ok := m["name"].(string); !ok {
			return false
		}
	}
	return true
}

func byName(l []any) map[string]map[string]any {
	out := map[string]map[string]any{}
	for _, item := range l {
		m := item.(map[string]any)
		out[m["name"].(string)] = m
	}
	return out
}

func display(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case nil:
		return "null"
	case json.Number:
		return x.String()
	case bool:
		return fmt.Sprint(x)
	}
	b, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprint(v)
	}
	return string(b)
}
