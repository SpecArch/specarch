package capability

import (
	"fmt"
	"strconv"
	"strings"
)

// SkillName is the name of the agent skill specarch skill writes, the name
// of its folder too, as the Agent Skills format asks.
const SkillName = "specarch"

// skillDescription says what the skill does and when an agent uses it; an
// agent reads it before the body, to choose the skill.
const skillDescription = "Write, check and generate from SpecArch specifications with the specarch command: validate a specification and read its problems, list the open questions and ask them one at a time, record decisions and approvals, write documents and code, and capture a system that exists from its code and documents. Use when a repository holds specarch.yaml, specarch-estate.yaml or *.specarch-implementation.yaml files, or when asked to write, review, check or generate from a specification with SpecArch."

// SkillFile is one file of the skill, by its path in the skill's folder.
type SkillFile struct {
	Path string
	Text string
}

// ReferencePath is the path of a command's reference in the skill's
// folder: a command of two words joined by a hyphen.
func ReferencePath(c Command) string {
	return "references/" + strings.ReplaceAll(c.Name, " ", "-") + ".md"
}

// Skill is the agent skill for SpecArch in the Agent Skills format
// (ADR-214): SKILL.md, whose body is AgentCore with the list of the
// references, then one reference per command. The plug-ins of the machine
// are left out, so one release writes one skill.
func Skill(l Listing, commands []Command) []SkillFile {
	var b strings.Builder
	b.WriteString("---\n")
	b.WriteString("name: " + SkillName + "\n")
	// Quoted: the description holds ": ", which YAML would read as a key.
	b.WriteString("description: " + strconv.Quote(skillDescription) + "\n")
	b.WriteString("---\n\n")
	fmt.Fprintf(&b, "<!-- Written by specarch skill %s from its command definitions; write it again with specarch skill --out <folder> after an upgrade. -->\n\n", l.Version)
	b.WriteString(AgentCore(l, commands))
	b.WriteString("\n## References\n\n")
	indent(&b, "Each command's reference, with its arguments, options, files and every exit status with its remedy; read one when the command is needed. The plug-ins this machine has are listed by `specarch capabilities --json`, and `specarch mcp` offers every command as a tool to a client of the Model Context Protocol.", "")
	b.WriteString("\n")
	for _, c := range commands {
		fmt.Fprintf(&b, "- [%s](%s): %s.\n", c.Name, ReferencePath(c), c.Summary)
	}
	files := []SkillFile{{Path: "SKILL.md", Text: b.String()}}
	for _, c := range commands {
		files = append(files, SkillFile{Path: ReferencePath(c), Text: CommandReference(c)})
	}
	return files
}

// CommandReference is one command's reference in Markdown: its synopsis,
// what it does, its arguments and options, what it reads, writes and
// prints, and each exit status with its meaning and remedy.
func CommandReference(c Command) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# specarch %s\n\n", c.Name)
	b.WriteString("```\n" + c.Usage() + "\n```\n\n")
	b.WriteString(c.Summary + ".\n")
	if c.Description != "" {
		b.WriteString("\n" + c.Description + "\n")
	}
	if len(c.Arguments) > 0 {
		b.WriteString("\n## Arguments\n\n")
		for _, a := range c.Arguments {
			var marks []string
			if a.Required {
				marks = append(marks, "required")
			}
			if a.Repeatable {
				marks = append(marks, "one or more")
			}
			if values := enumValues(a.Schema); values != "" {
				marks = append(marks, "one of "+values)
			}
			line := "- `<" + a.Name + ">`"
			if len(marks) > 0 {
				line += " (" + strings.Join(marks, "; ") + ")"
			}
			b.WriteString(line + ": " + a.Description + "\n")
		}
	}
	if len(c.Options) > 0 {
		b.WriteString("\n## Options\n\n")
		for _, o := range c.Options {
			name := "--" + o.Name
			if !o.Boolean() {
				name += " <" + o.Name + ">"
			}
			var marks []string
			if o.Required {
				marks = append(marks, "required")
			}
			if o.Repeatable() {
				marks = append(marks, "given once per value")
			}
			if values := enumValues(o.Schema); values != "" {
				marks = append(marks, "one of "+values)
			}
			line := "- `" + name + "`"
			if len(marks) > 0 {
				line += " (" + strings.Join(marks, "; ") + ")"
			}
			b.WriteString(line + ": " + o.Description + "\n")
		}
	}
	paths := func(title string, ps []Path) {
		if len(ps) == 0 {
			return
		}
		b.WriteString("\n## " + title + "\n\n")
		for _, p := range ps {
			b.WriteString("- `" + p.Path + "`: " + p.Description + "\n")
		}
	}
	paths("Reads", c.Reads)
	paths("Writes", c.Writes)
	if c.StandardOutput != "" {
		b.WriteString("\n## Standard output\n\n" + c.StandardOutput + "\n")
	}
	if c.StandardError != "" {
		b.WriteString("\n## Standard error\n\n" + c.StandardError + "\n")
	}
	b.WriteString("\n## Exit status\n\n")
	for _, e := range c.ExitCodes {
		line := fmt.Sprintf("- %d: %s.", e.Status, e.Meaning)
		if e.Remedy != "" {
			line += " Then: " + e.Remedy
		}
		b.WriteString(line + "\n")
	}
	return b.String()
}

// enumValues lists the values a schema allows, when it names them.
func enumValues(s map[string]any) string {
	values, ok := s["enum"].([]any)
	if !ok || len(values) == 0 {
		return ""
	}
	out := make([]string, len(values))
	for i, v := range values {
		out[i] = fmt.Sprintf("`%v`", v)
	}
	return strings.Join(out, ", ")
}
