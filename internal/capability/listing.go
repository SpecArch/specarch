package capability

import (
	"maps"
	"sort"
	"strings"
)

// Format is the version of the listing capabilities --json prints, under
// its key specarchCapabilities.
const Format = "1.0.0"

// Listing is everything the program says about itself, as capabilities
// --json prints it. Its tools are shaped as a tool listing of the Model
// Context Protocol is (name, title, description, inputSchema,
// annotations), so a thin wrapper can offer each command as a tool; the
// keys after those are this program's own.
type Listing struct {
	Format  string `json:"specarchCapabilities"`
	Program string `json:"program"`
	Version string `json:"version"`
	// Build is the module and revision the binary was built from, when
	// the Go toolchain recorded them.
	Build           *Build             `json:"build,omitempty"`
	Formats         []FormatVersions   `json:"formats"`
	Conventions     []string           `json:"conventions"`
	Workflow        []string           `json:"workflow"`
	ReadingProblems []string           `json:"readingProblems"`
	AskTheOwner     []string           `json:"askTheOwner"`
	Limits          []string           `json:"limits"`
	Tools           []Tool             `json:"tools"`
	Plugins         []Found            `json:"plugins"`
	DocumentTargets []string           `json:"documentTargets"`
	ExtractSources  []string           `json:"extractSources"`
	HelpTopics      []string           `json:"helpTopics"`
	Outputs         map[string]any     `json:"outputs"`
	ErrorCodes      []ErrorCodeMeaning `json:"errorCodes"`
}

// Build is what the Go toolchain recorded of the build.
type Build struct {
	Module   string `json:"module"`
	Version  string `json:"version"`
	Revision string `json:"revision,omitempty"`
	Modified bool   `json:"modified,omitempty"`
}

// FormatVersions is one kind of SpecArch file: the version the program
// writes and the versions it reads.
type FormatVersions struct {
	Key   string   `json:"key"`
	Name  string   `json:"name"`
	Write string   `json:"writes"`
	Read  []string `json:"reads"`
	// EarlierWarn is true when a file on an earlier version is read with
	// a warning naming specarch upgrade.
	EarlierWarn bool `json:"earlierWarn"`
}

// ErrorCodeMeaning is one stable code of the error object --json prints
// on standard error.
type ErrorCodeMeaning struct {
	Code    string `json:"code"`
	Meaning string `json:"meaning"`
}

// Tool is one operation: a command, or the generate command for one
// target, offered by the plug-ins found.
type Tool struct {
	Name        string         `json:"name"`
	Title       string         `json:"title"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"inputSchema"`
	Annotations Annotations    `json:"annotations"`
	// Command are the words before the inputs, such as specarch idioms
	// diff, and Positional the inputs given as arguments, in order; every
	// other input is the option of its name.
	Command        []string   `json:"command"`
	Positional     []string   `json:"positional"`
	Usage          string     `json:"usage"`
	Reads          []Path     `json:"reads"`
	Writes         []Path     `json:"writes"`
	StandardOutput string     `json:"standardOutput"`
	StandardError  string     `json:"standardError"`
	ExitCodes      []ExitCode `json:"exitCodes"`
	// Plugins are the plug-ins that write the target, for a generate
	// tool.
	Plugins []string `json:"plugins,omitempty"`
}

// Annotations are hints about what running the tool does.
type Annotations struct {
	ReadOnly    bool `json:"readOnlyHint"`
	Destructive bool `json:"destructiveHint"`
	Idempotent  bool `json:"idempotentHint"`
	OpenWorld   bool `json:"openWorldHint"`
}

// SchemaDialect is the JSON Schema dialect every inputSchema is written in.
const SchemaDialect = "https://json-schema.org/draft/2020-12/schema"

// ToolOf is a command as a tool.
func ToolOf(c Command) Tool {
	props := map[string]any{}
	var required, positional []string
	for _, a := range c.Arguments {
		s := copySchema(a.Schema)
		if a.Repeatable {
			arr := map[string]any{"type": "array", "items": s}
			if a.Required {
				arr["minItems"] = 1
			}
			s = arr
		}
		s["description"] = a.Description
		props[a.Name] = s
		positional = append(positional, a.Name)
		if a.Required {
			required = append(required, a.Name)
		}
	}
	for _, o := range c.Options {
		s := copySchema(o.Schema)
		s["description"] = o.Description
		props[o.Name] = s
		if o.Required {
			required = append(required, o.Name)
		}
	}
	schema := map[string]any{
		"$schema":              SchemaDialect,
		"type":                 "object",
		"properties":           props,
		"additionalProperties": false,
	}
	if len(required) > 0 {
		schema["required"] = required
	}
	description := c.Summary + "."
	if c.Description != "" {
		description += "\n\n" + c.Description
	}
	readOnly := !c.WritesFiles()
	return Tool{
		Name:           strings.ReplaceAll(c.Name, " ", "-"),
		Title:          c.Summary,
		Description:    description,
		InputSchema:    schema,
		Annotations:    Annotations{ReadOnly: readOnly, Destructive: !readOnly, Idempotent: readOnly},
		Command:        append([]string{"specarch"}, c.Words()...),
		Positional:     positional,
		Usage:          c.Usage(),
		Reads:          nonNil(c.Reads),
		Writes:         nonNil(c.Writes),
		StandardOutput: c.StandardOutput,
		StandardError:  c.StandardError,
		ExitCodes:      c.ExitCodes,
	}
}

// GenerateTools are the generate command once per target the plug-ins
// found write, with the target fixed: the operations the plug-ins add.
func GenerateTools(generate Command, found []Found) []Tool {
	byTarget := map[string][]Plugin{}
	for _, f := range found {
		if f.Capabilities != nil {
			byTarget[f.Capabilities.Target] = append(byTarget[f.Capabilities.Target], *f.Capabilities)
		}
	}
	targets := make([]string, 0, len(byTarget))
	for t := range byTarget {
		targets = append(targets, t)
	}
	sort.Strings(targets)
	var out []Tool
	for _, target := range targets {
		t := ToolOf(generate)
		t.Name = "generate-" + target
		t.Title = "Generate " + target
		props := t.InputSchema["properties"].(map[string]any)
		delete(props, "target")
		t.InputSchema["required"] = []string{"paths"}
		t.Command = []string{"specarch", "generate", target}
		t.Positional = []string{"paths"}
		t.Usage = strings.Replace(t.Usage, "<target>", target, 1)
		var b strings.Builder
		b.WriteString("Write the " + target + " target from an approved specification.")
		for _, p := range byTarget[target] {
			t.Plugins = append(t.Plugins, p.Plugin)
			b.WriteString("\n\n" + p.Plugin)
			if p.Stack != "" {
				b.WriteString(" (stack " + p.Stack + ")")
			}
			b.WriteString(": " + p.Summary)
			if len(p.Generates) > 0 {
				b.WriteString("\nWrites: " + strings.Join(p.Generates, "; ") + ".")
			}
			if len(p.DoesNotGenerate) > 0 {
				b.WriteString("\nDoes not write: " + strings.Join(p.DoesNotGenerate, "; ") + ".")
			}
		}
		t.Description = b.String()
		out = append(out, t)
	}
	return out
}

func copySchema(s map[string]any) map[string]any {
	out := make(map[string]any, len(s)+1)
	maps.Copy(out, s)
	return out
}

func nonNil(p []Path) []Path {
	if p == nil {
		return []Path{}
	}
	return p
}
