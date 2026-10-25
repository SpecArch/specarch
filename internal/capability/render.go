package capability

import (
	"fmt"
	"strings"
)

// width is the column text is wrapped at, for an 80-column terminal.
const width = 76

// Guide is the order of work for a person: the steps, then each command's
// synopsis and summary.
func Guide(l Listing, commands []Command) string {
	var b strings.Builder
	fmt.Fprintf(&b, "SpecArch %s: the order of work\n\n", l.Version)
	numbered(&b, l.Workflow)
	b.WriteString("\nCommands\n\n")
	for _, c := range commands {
		synopsis(&b, c, "  ", "    ")
		indent(&b, c.Summary+".", "      ")
	}
	b.WriteString("\nspecarch help <topic or command> explains each; specarch guide --agent is the\nsame order with what an agent must do and must ask.\n")
	return b.String()
}

// AgentGuide is the guide for an agent: AgentCore, then the plug-ins this
// install found.
func AgentGuide(l Listing, commands []Command) string {
	var b strings.Builder
	b.WriteString(AgentCore(l, commands))
	b.WriteString("\n## Plug-ins found on PATH\n\n")
	plugins(&b, l.Plugins)
	return b.String()
}

// AgentCore is everything the agent guide says that does not depend on the
// machine: AGENTS.md holds the same text.
func AgentCore(l Listing, commands []Command) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# SpecArch %s for agents\n\n", l.Version)
	indent(&b, "SpecArch is a compiler for specifications: it reads a specification written in YAML, lists every problem in it, and writes documents and code from it. `specarch capabilities --json` holds every fact below as JSON.", "")
	b.WriteString("\n## Formats\n\n")
	for _, f := range l.Formats {
		line := fmt.Sprintf("%s (version key %s): writes %s; reads %s", f.Name, f.Key, f.Write, strings.Join(f.Read, ", "))
		if f.EarlierWarn {
			line += ", an earlier one with a warning naming specarch upgrade"
		}
		bullet(&b, line+".")
	}
	b.WriteString("\n## Workflow\n\n")
	numbered(&b, l.Workflow)
	b.WriteString("\n## Reading problems\n\n")
	for _, s := range l.ReadingProblems {
		bullet(&b, s)
	}
	b.WriteString("\n## Ask the owner\n\n")
	for _, s := range l.AskTheOwner {
		bullet(&b, s)
	}
	b.WriteString("\n## Limits\n\n")
	for _, s := range l.Limits {
		bullet(&b, s)
	}
	b.WriteString("\n## Conventions\n\n")
	for _, s := range l.Conventions {
		bullet(&b, s)
	}
	b.WriteString("\nError codes of --json:\n\n")
	for _, e := range l.ErrorCodes {
		bullet(&b, e.Code+": "+e.Meaning)
	}
	b.WriteString("\n## Commands\n\n")
	for _, c := range commands {
		b.WriteString("- `" + c.Usage() + "`\n")
		indent(&b, c.Summary+".", "  ")
		for _, e := range c.ExitCodes {
			indent(&b, fmt.Sprintf("exit %d: %s", e.Status, e.Meaning), "  ")
		}
	}
	return b.String()
}

// Text is the listing for a person reading a terminal: what capabilities
// prints without --json.
func Text(l Listing, commands []Command) string {
	var b strings.Builder
	// The build is left to --json and go version -m, so the text is the
	// same for every build of one version.
	fmt.Fprintf(&b, "specarch %s\n", l.Version)
	b.WriteString("\nFormats\n")
	for _, f := range l.Formats {
		fmt.Fprintf(&b, "  %s: writes %s, reads %s\n", f.Name, f.Write, strings.Join(f.Read, ", "))
	}
	b.WriteString("\nCommands\n")
	for _, c := range commands {
		synopsis(&b, c, "  ", "    ")
		indent(&b, c.Summary+".", "      ")
	}
	b.WriteString("\n")
	wrapInto(&b, "Document targets: "+strings.Join(l.DocumentTargets, ", "), "", "  ")
	wrapInto(&b, "Extract sources: "+strings.Join(l.ExtractSources, ", "), "", "  ")
	b.WriteString("\nPlug-ins found on PATH\n")
	plugins(&b, l.Plugins)
	b.WriteString("\nLimits\n")
	for _, s := range l.Limits {
		bullet(&b, s)
	}
	b.WriteString("\nspecarch capabilities --json prints all of it, with each command's inputs as\nJSON Schema, its outputs and its exit codes.\n")
	return b.String()
}

// CommandHelp is one command's reference: synopsis, summary, arguments,
// options, exit codes with their remedies.
func CommandHelp(c Command) string {
	var b strings.Builder
	synopsis(&b, c, "", "    ")
	b.WriteString("\n")
	indent(&b, c.Summary+".", "")
	if len(c.Arguments) > 0 {
		b.WriteString("\nArguments\n")
		for _, a := range c.Arguments {
			b.WriteString("  <" + a.Name + ">\n")
			indent(&b, a.Description, "      ")
		}
	}
	if len(c.Options) > 0 {
		b.WriteString("\nOptions\n")
		for _, o := range c.Options {
			name := "--" + o.Name
			if !o.Boolean() {
				name += " <" + o.Name + ">"
			}
			b.WriteString("  " + name + "\n")
			indent(&b, o.Description, "      ")
		}
	}
	b.WriteString("\nOutput\n")
	indent(&b, c.StandardOutput, "  ")
	if len(c.Writes) > 0 {
		b.WriteString("\nWrites\n")
		for _, w := range c.Writes {
			indent(&b, w.Path+": "+w.Description, "  ")
		}
	}
	b.WriteString("\nExit status\n")
	for _, e := range c.ExitCodes {
		indent(&b, fmt.Sprintf("%d: %s", e.Status, e.Meaning), "  ")
		if e.Remedy != "" {
			indent(&b, "Then: "+e.Remedy, "     ")
		}
	}
	b.WriteString("\nspecarch capabilities --json has the full description of " + c.Name + ".\n")
	return b.String()
}

// PluginsText is the plug-ins found, one entry each, as the text listing
// prints them.
func PluginsText(found []Found) string {
	var b strings.Builder
	plugins(&b, found)
	return b.String()
}

func plugins(b *strings.Builder, found []Found) {
	if len(found) == 0 {
		b.WriteString("  none: put the specarch-gen-<target> executables on PATH to generate code.\n")
		return
	}
	for _, f := range found {
		if f.Capabilities == nil {
			indent(b, f.Name()+" ("+f.Path+"): "+f.Error, "  ")
			continue
		}
		p := f.Capabilities
		head := p.Plugin + ": target " + p.Target
		if p.Stack != "" {
			head += ", stack " + p.Stack
		}
		indent(b, head+". "+p.Summary, "  ")
		if len(p.Generates) > 0 {
			indent(b, "Writes: "+strings.Join(p.Generates, "; ")+".", "      ")
		}
		if len(p.DoesNotGenerate) > 0 {
			indent(b, "Does not write: "+strings.Join(p.DoesNotGenerate, "; ")+".", "      ")
		}
	}
}

// synopsis writes a command's synopsis wrapped at width between its
// words, the first line after first and the others after rest.
func synopsis(b *strings.Builder, c Command, first, rest string) {
	line := first
	for i, w := range c.UsageWords() {
		if i > 0 && len(line)+1+len(w) > width {
			b.WriteString(line + "\n")
			line = rest + w
			continue
		}
		if i > 0 {
			line += " "
		}
		line += w
	}
	b.WriteString(line + "\n")
}

func numbered(b *strings.Builder, steps []string) {
	for i, s := range steps {
		prefix := fmt.Sprintf("%d. ", i+1)
		wrapInto(b, s, prefix, strings.Repeat(" ", len(prefix)))
	}
}

func bullet(b *strings.Builder, s string) { wrapInto(b, s, "- ", "  ") }

func indent(b *strings.Builder, s, prefix string) { wrapInto(b, s, prefix, prefix) }

// wrapInto writes s wrapped at width, its first line after first and the
// others after rest; a blank line in s is kept as a paragraph break.
func wrapInto(b *strings.Builder, s, first, rest string) {
	prefix := first
	for pi, para := range strings.Split(strings.TrimSpace(s), "\n\n") {
		if pi > 0 {
			b.WriteString("\n")
		}
		line := prefix
		empty := true
		for _, w := range strings.Fields(para) {
			if !empty && len(line)+1+len(w) > width {
				b.WriteString(line + "\n")
				line, empty = rest, true
			}
			if !empty {
				line += " "
			}
			line += w
			empty = false
		}
		b.WriteString(line + "\n")
		prefix = rest
	}
}
