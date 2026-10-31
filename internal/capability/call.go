package capability

import (
	"fmt"
	"sort"
	"strings"
)

// InputError is an input of a tool call a command line cannot carry: one
// the tool does not take, a required one left out, a value of the wrong
// JSON type, or an argument that starts with -. Its text names the input
// and what to give instead.
type InputError struct{ Text string }

func (e *InputError) Error() string { return e.Text }

// CommandLine is the arguments, after specarch, that run a tool with the
// inputs of a call (ADR-214): the tool's command words, then the command's
// arguments that are not repeated, each option as --<name> <value> (a
// boolean only when true, a list once per value), then the repeated
// arguments, the order of the command's synopsis. generate, document and
// extract read their target before any option, so the order matters. An
// argument the tool fixes, as generate-<target> fixes the target, is not
// an input. Only what a command line cannot carry is checked; the rest of
// the schema, a pattern or an enum, is the command's to check, so that its
// usage error names it with its remedy.
func CommandLine(t Tool, c Command, input map[string]any) ([]string, error) {
	props, _ := t.InputSchema["properties"].(map[string]any)
	var names []string
	for name := range props {
		names = append(names, name)
	}
	sort.Strings(names)
	var unknown []string
	for name := range input {
		if _, ok := props[name]; !ok {
			unknown = append(unknown, name)
		}
	}
	if len(unknown) > 0 {
		sort.Strings(unknown)
		return nil, &InputError{fmt.Sprintf("%s takes no input %s; its inputs are %s", t.Name, quoted(unknown), inputList(names))}
	}
	required, _ := t.InputSchema["required"].([]string)
	for _, name := range required {
		if _, ok := input[name]; !ok {
			return nil, &InputError{fmt.Sprintf("%s needs the input %q: %s", t.Name, name, describe(props[name]))}
		}
	}
	positional := map[string]bool{}
	for _, p := range t.Positional {
		positional[p] = true
	}
	words := append([]string{}, t.Command[1:]...)
	argument := func(a Argument) ([]string, error) {
		v, ok := input[a.Name]
		if !ok || !positional[a.Name] {
			return nil, nil
		}
		var values []string
		if a.Repeatable {
			list, err := stringList(t.Name, a.Name, v)
			if err != nil {
				return nil, err
			}
			values = list
		} else {
			s, ok := v.(string)
			if !ok {
				return nil, wrongType(t.Name, a.Name, "a string", v)
			}
			values = []string{s}
		}
		for _, s := range values {
			if strings.HasPrefix(s, "-") {
				return nil, &InputError{fmt.Sprintf("the input %q of %s is %q, which starts with -, so the command would read it as an option; give a path that starts with ./ instead", a.Name, t.Name, s)}
			}
		}
		return values, nil
	}
	for _, a := range c.Arguments {
		if a.Repeatable {
			continue
		}
		values, err := argument(a)
		if err != nil {
			return nil, err
		}
		words = append(words, values...)
	}
	for _, o := range c.Options {
		v, ok := input[o.Name]
		if !ok {
			continue
		}
		switch {
		case o.Boolean():
			b, ok := v.(bool)
			if !ok {
				return nil, wrongType(t.Name, o.Name, "true or false", v)
			}
			if b {
				words = append(words, "--"+o.Name)
			}
		case o.Repeatable():
			list, err := stringList(t.Name, o.Name, v)
			if err != nil {
				return nil, err
			}
			for _, s := range list {
				words = append(words, "--"+o.Name, s)
			}
		default:
			s, ok := v.(string)
			if !ok {
				return nil, wrongType(t.Name, o.Name, "a string", v)
			}
			words = append(words, "--"+o.Name, s)
		}
	}
	for _, a := range c.Arguments {
		if !a.Repeatable {
			continue
		}
		values, err := argument(a)
		if err != nil {
			return nil, err
		}
		words = append(words, values...)
	}
	return words, nil
}

func stringList(tool, name string, v any) ([]string, error) {
	items, ok := v.([]any)
	if !ok {
		return nil, wrongType(tool, name, "a list of strings", v)
	}
	out := make([]string, 0, len(items))
	for _, item := range items {
		s, ok := item.(string)
		if !ok {
			return nil, wrongType(tool, name, "a list of strings", v)
		}
		out = append(out, s)
	}
	return out, nil
}

func wrongType(tool, name, want string, v any) error {
	return &InputError{fmt.Sprintf("the input %q of %s is %s, and was given %s", name, tool, want, jsonKind(v))}
}

// jsonKind names the JSON type of a value encoding/json decoded.
func jsonKind(v any) string {
	switch v.(type) {
	case nil:
		return "null"
	case bool:
		return "a boolean"
	case float64:
		return "a number"
	case string:
		return "a string"
	case []any:
		return "a list"
	case map[string]any:
		return "an object"
	}
	return fmt.Sprintf("a %T", v)
}

func describe(p any) string {
	if s, ok := p.(map[string]any); ok {
		if d, ok := s["description"].(string); ok && d != "" {
			return d
		}
	}
	return "see the tool's inputSchema"
}

func quoted(names []string) string {
	q := make([]string, len(names))
	for i, n := range names {
		q[i] = fmt.Sprintf("%q", n)
	}
	return strings.Join(q, ", ")
}

func inputList(names []string) string {
	if len(names) == 0 {
		return "none"
	}
	return strings.Join(names, ", ")
}
