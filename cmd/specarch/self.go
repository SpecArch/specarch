package main

import (
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"runtime/debug"
	"sort"
	"strings"
	"time"

	"github.com/SpecArch/specarch"
	"github.com/SpecArch/specarch/internal/capability"
	"github.com/SpecArch/specarch/internal/generate"
	"github.com/SpecArch/specarch/internal/metamodel"
)

// pluginTimeout is how long a plug-in may take to answer --capabilities.
const pluginTimeout = 5 * time.Second

// ownCommands are the commands as the specification defines them, from
// the copy of spec/design/commands built into the program.
func ownCommands() ([]capability.Command, error) {
	return capability.LoadCommands(specarch.Commands, "spec/design/commands")
}

// helpTopics are the topics docs/help holds, by name, with their text.
func helpTopics() map[string]string {
	out := map[string]string{}
	files, _ := fs.Glob(specarch.Help, "docs/help/*.txt")
	for _, f := range files {
		data, err := fs.ReadFile(specarch.Help, f)
		if err == nil {
			out[strings.TrimSuffix(path.Base(f), ".txt")] = string(data)
		}
	}
	return out
}

// listing gathers what the program says about itself, with the plug-ins
// found on PATH now when withPlugins is true: each is run, so a text that
// does not show them does not ask them.
func listing(commands []capability.Command, withPlugins bool) capability.Listing {
	var found []capability.Found
	if withPlugins {
		found = capability.Gather(os.Getenv("PATH"), pluginTimeout)
	}
	l := capability.Listing{
		Format:          capability.Format,
		Program:         "specarch",
		Version:         specarch.Version,
		Build:           buildInfo(),
		Conventions:     capability.Conventions,
		Workflow:        capability.Workflow,
		ReadingProblems: capability.ReadingProblems,
		AskTheOwner:     capability.AskTheOwner,
		Limits:          capability.Limits,
		Plugins:         found,
		DocumentTargets: generate.DocumentTargets,
		ExtractSources:  extractSources,
		Outputs:         capability.Outputs,
		ErrorCodes:      capability.ErrorCodes,
	}
	if l.Plugins == nil {
		l.Plugins = []capability.Found{}
	}
	for _, f := range []metamodel.Format{metamodel.Design, metamodel.Implementation, metamodel.Record, metamodel.Idiom, metamodel.Estate, metamodel.Vocabulary} {
		reads := []string{"1.0.0 and any later 1.0 patch"}
		for i := len(f.Earlier) - 1; i >= 0; i-- {
			reads = append(reads, f.Earlier[i])
		}
		l.Formats = append(l.Formats, capability.FormatVersions{Key: f.Key, Name: f.Name, Write: metamodel.Current, Read: reads, EarlierWarn: !f.Silent})
	}
	var generateCommand capability.Command
	for _, c := range commands {
		l.Tools = append(l.Tools, capability.ToolOf(c))
		if c.Name == "generate" {
			generateCommand = c
		}
	}
	l.Tools = append(l.Tools, capability.GenerateTools(generateCommand, found)...)
	topics := helpTopics()
	for t := range topics {
		l.HelpTopics = append(l.HelpTopics, t)
	}
	for _, c := range commands {
		if _, ok := topics[c.Name]; !ok {
			l.HelpTopics = append(l.HelpTopics, c.Name)
		}
	}
	sort.Strings(l.HelpTopics)
	return l
}

// buildInfo is the module and revision the Go toolchain recorded, or nil
// for a build that recorded none.
func buildInfo() *capability.Build {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return nil
	}
	b := &capability.Build{Module: info.Main.Path, Version: info.Main.Version}
	for _, s := range info.Settings {
		switch s.Key {
		case "vcs.revision":
			b.Revision = s.Value
		case "vcs.modified":
			b.Modified = s.Value == "true"
		}
	}
	if b.Module == "" {
		return nil
	}
	return b
}

// runCapabilities prints what the program can do: as text, or with
// --json as the listing.
func runCapabilities(args []string, stdout, stderr io.Writer) int {
	asJSON := false
	for _, a := range args {
		if a == "--json" {
			asJSON = true
			continue
		}
		fmt.Fprintf(stderr, "specarch capabilities takes no argument but --json, given %s\n\n%s", a, usage)
		return 2
	}
	commands, err := ownCommands()
	if err != nil {
		fmt.Fprintf(stderr, "specarch capabilities: the command definitions built into this program cannot be read: %v\n", err)
		return 2
	}
	l := listing(commands, true)
	if !asJSON {
		fmt.Fprint(stdout, capability.Text(l, commands))
		return 0
	}
	out, err := json.MarshalIndent(l, "", "  ")
	if err != nil {
		fmt.Fprintf(stderr, "specarch capabilities: cannot encode the listing: %v\n", err)
		return 2
	}
	stdout.Write(append(out, '\n'))
	return 0
}

// runGuide prints the order of work, for a person, or with --agent for an
// agent.
func runGuide(args []string, stdout, stderr io.Writer) int {
	agent := false
	for _, a := range args {
		if a == "--agent" {
			agent = true
			continue
		}
		fmt.Fprintf(stderr, "specarch guide takes no argument but --agent, given %s\n\n%s", a, usage)
		return 2
	}
	commands, err := ownCommands()
	if err != nil {
		fmt.Fprintf(stderr, "specarch guide: the command definitions built into this program cannot be read: %v\n", err)
		return 2
	}
	l := listing(commands, agent)
	if agent {
		fmt.Fprint(stdout, capability.AgentGuide(l, commands))
	} else {
		fmt.Fprint(stdout, capability.Guide(l, commands))
	}
	return 0
}

// runHelp prints the usage and the topics, or one topic: a guide to an
// area, a command's reference after its guide when it has one, or the
// plug-ins found with their guide.
func runHelp(args []string, stdout, stderr io.Writer) int {
	topics := helpTopics()
	commands, err := ownCommands()
	if err != nil {
		fmt.Fprintf(stderr, "specarch help: the command definitions built into this program cannot be read: %v\n", err)
		return 2
	}
	if len(args) == 0 {
		fmt.Fprint(stdout, usage)
		fmt.Fprint(stdout, "\nHelp topics, as specarch help <topic>:\n")
		for _, t := range topicNames(topics, commands) {
			fmt.Fprintf(stdout, "  %s\n", t)
		}
		fmt.Fprint(stdout, "\nThe commands in the order of work: specarch guide. For an agent: specarch guide --agent.\nEverything as JSON, each command's inputs as JSON Schema: specarch capabilities --json.\n")
		return 0
	}
	topic := strings.Join(args, " ")
	var command *capability.Command
	for i := range commands {
		if commands[i].Name == topic || strings.ReplaceAll(commands[i].Name, " ", "-") == topic {
			command = &commands[i]
		}
	}
	text, ok := topics[topic]
	if !ok && command == nil {
		fmt.Fprintf(stderr, "specarch help has no topic %q; the topics are %s\n", topic, strings.Join(topicNames(topics, commands), ", "))
		return 2
	}
	fmt.Fprint(stdout, text)
	if topic == "plugins" {
		fmt.Fprint(stdout, "\nPlug-ins found on PATH now\n")
		l := listing(commands, true)
		fmt.Fprint(stdout, capability.PluginsText(l.Plugins))
	}
	if command != nil {
		if ok {
			fmt.Fprint(stdout, "\n")
		}
		fmt.Fprint(stdout, capability.CommandHelp(*command))
	}
	return 0
}

func topicNames(topics map[string]string, commands []capability.Command) []string {
	seen := map[string]bool{}
	var out []string
	for t := range topics {
		seen[t] = true
		out = append(out, t)
	}
	for _, c := range commands {
		if !seen[c.Name] {
			out = append(out, c.Name)
		}
	}
	sort.Strings(out)
	return out
}
