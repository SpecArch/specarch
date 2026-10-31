package main

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/SpecArch/specarch/internal/extract"
	"github.com/SpecArch/specarch/internal/metamodel"
)

// runInit writes the smallest specification that validates into a folder
// that holds none (ADR-210): the schema line, the meta-model version and
// the info the format requires, with the profile when one is given.
func runInit(args []string, stdout, stderr io.Writer) int {
	title, profile, dryRun := "", "", false
	var rest []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		name, value := "", ""
		switch {
		case a == "--":
			rest = append(rest, args[i+1:]...)
			i = len(args)
			continue
		case a == "--dry-run":
			dryRun = true
			continue
		case a == "--title" || a == "--profile":
			if i+1 >= len(args) {
				fmt.Fprintf(stderr, "specarch init: %s needs a value\n\n%s", a, usage)
				return 2
			}
			name, value = a, args[i+1]
			i++
		case strings.HasPrefix(a, "--title=") || strings.HasPrefix(a, "--profile="):
			name, value, _ = strings.Cut(a, "=")
		case strings.HasPrefix(a, "-") && len(a) > 1:
			fmt.Fprintf(stderr, "specarch init has no option %s; its options are --title, --profile and --dry-run\n\n%s", a, usage)
			return 2
		default:
			rest = append(rest, a)
			continue
		}
		switch name {
		case "--title":
			if strings.TrimSpace(value) == "" {
				fmt.Fprintf(stderr, "specarch init: --title is empty; give the specification's title, or leave --title out to take the folder's name\n\n%s", usage)
				return 2
			}
			title = value
		case "--profile":
			if value != "lean" && value != "full" {
				fmt.Fprintf(stderr, "specarch init: --profile is %q; the profiles are lean and full\n\n%s", value, usage)
				return 2
			}
			profile = value
		}
	}
	if len(rest) != 1 {
		fmt.Fprintf(stderr, "specarch init needs one folder, the one the specification goes in\n\n%s", usage)
		return 2
	}
	folder := rest[0]
	path := filepath.Join(folder, "specarch.yaml")
	if _, err := os.Stat(path); err == nil {
		fmt.Fprintf(stderr, "specarch init: %s already exists; init never overwrites a specification, so give a folder without specarch.yaml, or edit that file\n", path)
		return 2
	} else if !errors.Is(err, fs.ErrNotExist) {
		fmt.Fprintf(stderr, "specarch init: cannot read %s to see whether a specification is there: %v\n", path, err)
		return 2
	}
	if title == "" {
		abs, err := filepath.Abs(folder)
		if err != nil {
			fmt.Fprintf(stderr, "specarch init: cannot read the name of the folder %s to take it as the title: %v; give the title with --title\n", folder, err)
			return 2
		}
		title = filepath.Base(abs)
	}
	text := initText(title, profile)
	if dryRun {
		fmt.Fprintf(stdout, "would write %s\n%s", path, text)
		fmt.Fprintf(stderr, "specarch init: nothing written (--dry-run)\n")
		return 0
	}
	if err := os.MkdirAll(folder, 0o755); err != nil {
		fmt.Fprintf(stderr, "specarch init: cannot write the folder %s: %v\n", folder, err)
		return 2
	}
	// O_EXCL: a file made between the check above and now is not overwritten.
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		fmt.Fprintf(stderr, "specarch init: cannot write %s: %v\n", path, err)
		return 2
	}
	if _, err := f.WriteString(text); err != nil {
		f.Close()
		fmt.Fprintf(stderr, "specarch init: cannot write %s: %v\n", path, err)
		return 2
	}
	if err := f.Close(); err != nil {
		fmt.Fprintf(stderr, "specarch init: cannot write %s: %v\n", path, err)
		return 2
	}
	fmt.Fprintln(stdout, path)
	fmt.Fprintf(stderr, "specarch init: next, run specarch validate %s after each edit; specarch help start shows a first entity and operation that validate\n", folder)
	return 0
}

// initText is the smallest specification that validates, with the title
// written as a YAML scalar and the profile when one is given.
func initText(title, profile string) string {
	var b strings.Builder
	b.WriteString(extract.SchemaLine)
	fmt.Fprintf(&b, "specarch: %q\n\n", metamodel.Current)
	b.WriteString("info:\n")
	fmt.Fprintf(&b, "  title: %s\n", yamlScalar(title))
	b.WriteString("  version: 0.1.0\n")
	if profile != "" {
		fmt.Fprintf(&b, "  profile: %s\n", profile)
	}
	return b.String()
}

// yamlScalar writes a string as one YAML scalar on one line: plain when
// YAML reads it back as the same string, quoted otherwise.
func yamlScalar(s string) string {
	var n yaml.Node
	n.SetString(s)
	out, err := yaml.Marshal(&n)
	if err != nil || strings.Count(strings.TrimRight(string(out), "\n"), "\n") > 0 {
		return fmt.Sprintf("%q", s)
	}
	return strings.TrimRight(string(out), "\n")
}
