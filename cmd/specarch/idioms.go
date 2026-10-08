package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/SpecArch/specarch/internal/source"
	"github.com/SpecArch/specarch/internal/validate"
)

// runIdioms lists, per implementation file, every idiom that applies to it
// and how: shipped, overridden (with the parts the project replaces), the
// project's own, or excluded, with the reason. "idioms diff" prints a
// shipped idiom's parts beside each override's. It exits 0, and 2 on a
// usage error or a specification with errors.
func runIdioms(args []string, stdout, stderr io.Writer) int {
	if len(args) > 0 && args[0] == "diff" {
		return runIdiomsDiff(args[1:], stdout, stderr)
	}
	if len(args) == 0 {
		fmt.Fprintf(stderr, "specarch idioms needs at least one folder\n\n%s", usage)
		return 2
	}
	for _, a := range args {
		if len(a) > 1 && a[0] == '-' {
			fmt.Fprintf(stderr, "specarch idioms has no option %s\n\n%s", a, usage)
			return 2
		}
	}
	specs, status := loadSpecs(args, "idioms", stdout, stderr)
	if status != 0 {
		return 2
	}
	for _, l := range specs {
		for _, i := range l.impls {
			fmt.Fprintln(stdout, filepath.ToSlash(i.Path))
			uses := validate.IdiomUses(i.Path, i.Node, l.spec)
			if len(uses) == 0 {
				fmt.Fprintln(stdout, "  no idiom applies")
			}
			for _, u := range uses {
				line := fmt.Sprintf("  %s %s: ", u.Name, u.Version)
				switch u.As {
				case "overridden":
					line += fmt.Sprintf("overridden by %s, copied from %s, replacing %s", filepath.ToSlash(u.Override), u.From, strings.Join(u.Parts, ", "))
				case "project":
					line += "the project's own"
				default:
					line += u.As
				}
				if u.Why != "" {
					line += "; " + oneLine(u.Why)
				}
				fmt.Fprintln(stdout, line)
			}
		}
	}
	return 0
}

// runIdiomsDiff prints, for every implementation file that overrides the
// named idiom, each part it replaces, stack by stack: the shipped
// rendering, then the override's.
func runIdiomsDiff(args []string, stdout, stderr io.Writer) int {
	if len(args) < 2 {
		fmt.Fprintf(stderr, "specarch idioms diff needs an idiom's name and at least one folder\n\n%s", usage)
		return 2
	}
	name := args[0]
	shipped, ok := validate.ShippedIdioms()[name]
	if !ok {
		fmt.Fprintf(stderr, "specarch idioms diff: %s is not an idiom SpecArch ships; specarch idioms lists the ones that apply\n", name)
		return 2
	}
	specs, status := loadSpecs(args[1:], "idioms", stdout, stderr)
	if status != 0 {
		return 2
	}
	found := false
	for _, l := range specs {
		for _, i := range l.impls {
			for _, u := range validate.IdiomUses(i.Path, i.Node, l.spec) {
				if u.Name != name || u.As != "overridden" {
					continue
				}
				found = true
				data, err := os.ReadFile(u.Override)
				if err != nil {
					fmt.Fprintf(stderr, "specarch idioms diff: cannot read %s: %v\n", u.Override, err)
					return 2
				}
				o := source.Parse(data)
				fmt.Fprintf(stdout, "%s overrides %s %s; SpecArch ships %s.\n", filepath.ToSlash(u.Override), name, u.From, shipped.Version())
				for _, part := range u.Parts {
					shippedStacks := source.Child(source.Child(source.Child(shipped.Root, "parts"), part), "stack")
					for _, st := range source.Pairs(source.Child(source.Child(source.Child(o.Root, "parts"), part), "stack")) {
						fmt.Fprintf(stdout, "\npart %s, stack %s, shipped:\n%s", part, st.Key.Value, yamlText(source.Child(shippedStacks, st.Key.Value)))
						fmt.Fprintf(stdout, "\npart %s, stack %s, override:\n%s", part, st.Key.Value, yamlText(st.Value))
					}
				}
			}
		}
	}
	if !found {
		fmt.Fprintf(stdout, "No implementation file overrides %s.\n", name)
	}
	return 0
}

// yamlText is a node as YAML, indented two spaces, or a line saying there
// is none.
func yamlText(n *yaml.Node) string {
	if n == nil {
		return "  (none)\n"
	}
	var b bytes.Buffer
	enc := yaml.NewEncoder(&b)
	enc.SetIndent(2)
	if err := enc.Encode(n); err != nil {
		return "  (cannot print: " + err.Error() + ")\n"
	}
	var out strings.Builder
	for _, line := range strings.Split(strings.TrimRight(b.String(), "\n"), "\n") {
		out.WriteString("  " + line + "\n")
	}
	return out.String()
}

func oneLine(s string) string { return strings.Join(strings.Fields(s), " ") }
