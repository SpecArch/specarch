// Package capability is what the specarch program says about itself: its
// commands as its specification defines them, the plug-ins it finds, the
// formats it reads and writes, and the order of work. help, capabilities
// and guide print it, so the text a person reads, the listing a program
// reads and the agent's guide hold the same facts.
package capability

import (
	"fmt"
	"io/fs"
	"path"
	"sort"
	"strconv"
	"strings"

	"go.yaml.in/yaml/v3"
)

// Command is one command as the specification defines it under
// commands.<name>.
type Command struct {
	// Name is the command's name after specarch, such as validate or
	// idioms diff.
	Name           string
	Summary        string
	Description    string
	Arguments      []Argument
	Options        []Option
	Reads          []Path
	Writes         []Path
	StandardOutput string
	StandardError  string
	ExitCodes      []ExitCode
}

// Argument is a positional argument, in the order the command takes them.
type Argument struct {
	Name        string
	Description string
	Required    bool
	Repeatable  bool
	Schema      map[string]any
}

// Option is a flag, written --<name>.
type Option struct {
	Name        string
	Description string
	Required    bool
	Schema      map[string]any
}

// Path is a file or folder a command reads or writes.
type Path struct {
	Path        string `json:"path"`
	Description string `json:"description"`
}

// ExitCode is one exit status, what it means and what to do about it.
type ExitCode struct {
	Status  int    `json:"status"`
	Meaning string `json:"meaning"`
	Remedy  string `json:"remedy,omitempty"`
}

// Boolean is an option given alone, with no value.
func (o Option) Boolean() bool { return o.Schema["type"] == "boolean" }

// Repeatable is an option given once per value.
func (o Option) Repeatable() bool { return o.Schema["type"] == "array" }

// Words are the command's name split at spaces: idioms diff is two words.
func (c Command) Words() []string { return strings.Fields(c.Name) }

// Option finds an option by name.
func (c Command) Option(name string) (Option, bool) {
	for _, o := range c.Options {
		if o.Name == name {
			return o, true
		}
	}
	return Option{}, false
}

// WritesFiles says whether the command writes any file.
func (c Command) WritesFiles() bool { return len(c.Writes) > 0 }

// Usage is the command's synopsis: the positional arguments that are not
// repeated, then the options, then the repeated ones, as the usage text
// writes them.
func (c Command) Usage() string { return strings.Join(c.UsageWords(), " ") }

// UsageWords are the synopsis's words, an option with its value one word,
// so that a wrapped synopsis never splits one.
func (c Command) UsageWords() []string {
	words := []string{"specarch", c.Name}
	for _, a := range c.Arguments {
		if !a.Repeatable {
			words = append(words, "<"+a.Name+">")
		}
	}
	for _, o := range c.Options {
		w := "--" + o.Name
		if !o.Boolean() {
			w += " <" + o.Name + ">"
		}
		if !o.Required {
			w = "[" + w + "]"
		}
		if o.Repeatable() {
			w += "..."
		}
		words = append(words, w)
	}
	for _, a := range c.Arguments {
		if a.Repeatable {
			w := "<" + a.Name + ">..."
			if !a.Required {
				w = "[" + w + "]"
			}
			words = append(words, w)
		}
	}
	return words
}

// LoadCommands reads every commands fragment of a folder of the
// specification, such as the embedded spec/design/commands, in name order.
func LoadCommands(fsys fs.FS, dir string) ([]Command, error) {
	files, err := fs.Glob(fsys, path.Join(dir, "*.yaml"))
	if err != nil {
		return nil, err
	}
	var out []Command
	for _, f := range files {
		data, err := fs.ReadFile(fsys, f)
		if err != nil {
			return nil, err
		}
		var doc yaml.Node
		if err := yaml.Unmarshal(data, &doc); err != nil {
			return nil, fmt.Errorf("%s: %v", f, err)
		}
		commands := child(root(&doc), "commands")
		if commands == nil {
			return nil, fmt.Errorf("%s holds no commands key", f)
		}
		for i := 0; i+1 < len(commands.Content); i += 2 {
			c, err := decodeCommand(commands.Content[i].Value, commands.Content[i+1])
			if err != nil {
				return nil, fmt.Errorf("%s: commands.%s: %v", f, commands.Content[i].Value, err)
			}
			out = append(out, c)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

func decodeCommand(name string, n *yaml.Node) (Command, error) {
	c := Command{
		Name:           name,
		Summary:        str(child(n, "summary")),
		Description:    strings.TrimSpace(str(child(n, "description"))),
		StandardOutput: strings.TrimSpace(str(child(n, "standardOutput"))),
		StandardError:  strings.TrimSpace(str(child(n, "standardError"))),
	}
	if a := child(n, "arguments"); a != nil {
		for _, item := range a.Content {
			arg := Argument{
				Name:        str(child(item, "name")),
				Description: str(child(item, "description")),
				Required:    str(child(item, "required")) == "true",
				Repeatable:  str(child(item, "repeatable")) == "true",
			}
			if err := decodeSchema(child(item, "schema"), &arg.Schema); err != nil {
				return c, err
			}
			c.Arguments = append(c.Arguments, arg)
		}
	}
	if o := child(n, "options"); o != nil {
		for i := 0; i+1 < len(o.Content); i += 2 {
			opt := Option{Name: o.Content[i].Value, Description: str(child(o.Content[i+1], "description")), Required: str(child(o.Content[i+1], "required")) == "true"}
			if err := decodeSchema(child(o.Content[i+1], "schema"), &opt.Schema); err != nil {
				return c, err
			}
			c.Options = append(c.Options, opt)
		}
	}
	for _, key := range []string{"reads", "writes"} {
		var paths []Path
		if p := child(n, key); p != nil {
			if err := p.Decode(&paths); err != nil {
				return c, err
			}
		}
		if key == "reads" {
			c.Reads = paths
		} else {
			c.Writes = paths
		}
	}
	remedies := map[string]string{}
	if r := child(n, "remedies"); r != nil {
		if err := r.Decode(&remedies); err != nil {
			return c, err
		}
	}
	if e := child(n, "exitCodes"); e != nil {
		for i := 0; i+1 < len(e.Content); i += 2 {
			status, err := strconv.Atoi(e.Content[i].Value)
			if err != nil {
				return c, fmt.Errorf("exit code %q is not a number", e.Content[i].Value)
			}
			c.ExitCodes = append(c.ExitCodes, ExitCode{Status: status, Meaning: str(e.Content[i+1]), Remedy: remedies[e.Content[i].Value]})
		}
	}
	sort.Slice(c.ExitCodes, func(i, j int) bool { return c.ExitCodes[i].Status < c.ExitCodes[j].Status })
	return c, nil
}

func decodeSchema(n *yaml.Node, into *map[string]any) error {
	if n == nil {
		*into = map[string]any{"type": "string"}
		return nil
	}
	return n.Decode(into)
}

func root(doc *yaml.Node) *yaml.Node {
	if doc.Kind == yaml.DocumentNode && len(doc.Content) > 0 {
		return doc.Content[0]
	}
	return doc
}

func child(n *yaml.Node, key string) *yaml.Node {
	if n == nil || n.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(n.Content); i += 2 {
		if n.Content[i].Value == key {
			return n.Content[i+1]
		}
	}
	return nil
}

func str(n *yaml.Node) string {
	if n == nil {
		return ""
	}
	return n.Value
}
