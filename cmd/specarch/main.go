// Command specarch checks SpecArch design and implementation files.
//
//	specarch validate <file or folder>...
//	specarch version
package main

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"

	"github.com/SpecArch/specarch/internal/validate"
)

// version is the program version, set at release time.
const version = "0.1.0"

const usage = `usage:
  specarch validate <file or folder>...   check SpecArch files
  specarch version                         print the program version

A folder is searched for *.specarch-design.yaml and
*.specarch-implementation.yaml files.
`

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprint(stderr, usage)
		return 2
	}
	switch args[0] {
	case "validate":
		return runValidate(args[1:], stdout, stderr)
	case "version":
		if len(args) > 1 {
			fmt.Fprintf(stderr, "specarch version takes no arguments\n\n%s", usage)
			return 2
		}
		return runVersion(stdout)
	case "help", "-h", "--help":
		fmt.Fprint(stdout, usage)
		return 0
	}
	fmt.Fprintf(stderr, "specarch has no command %q\n\n%s", args[0], usage)
	return 2
}

func runVersion(stdout io.Writer) int {
	fmt.Fprintf(stdout, "specarch %s\n", version)
	fmt.Fprintln(stdout, "design files: meta-model 0.1")
	fmt.Fprintln(stdout, "implementation files: meta-model 0.1")
	return 0
}

// runValidate implements the exitStatus algorithm of the design: 2 for a
// usage or read error, 1 when any diagnostic was printed, 0 otherwise.
func runValidate(args []string, stdout, stderr io.Writer) int {
	if len(args) > 0 && args[0] == "--" {
		args = args[1:]
	}
	if len(args) == 0 {
		fmt.Fprintf(stderr, "specarch validate needs at least one file or folder\n\n%s", usage)
		return 2
	}
	for _, a := range args {
		if len(a) > 1 && a[0] == '-' {
			fmt.Fprintf(stderr, "specarch validate has no option %s; to check a file whose name starts with -, write -- before it\n\n%s", a, usage)
			return 2
		}
	}
	files, ioError := collect(args, stderr)
	var all []validate.Diagnostic
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			fmt.Fprintf(stderr, "specarch: cannot read %s: %v\n", f, err)
			ioError = true
			continue
		}
		all = append(all, validate.Check(f, data, os.ReadFile)...)
	}
	validate.Sort(all)
	for _, d := range all {
		fmt.Fprintln(stdout, d.String())
	}
	errors := validate.Errors(all)
	fmt.Fprintf(stderr, "specarch: %s checked: %s, %s\n", plural(len(files), "file"), plural(errors, "error"), plural(len(all)-errors, "warning"))
	switch {
	case ioError:
		return 2
	case errors > 0:
		return 1
	}
	return 0
}

// collect expands folders into the SpecArch files under them, sorted, and
// keeps files as given.
func collect(args []string, stderr io.Writer) ([]string, bool) {
	var files []string
	ioError := false
	seen := map[string]bool{}
	addFile := func(p string) {
		if !seen[p] {
			seen[p] = true
			files = append(files, p)
		}
	}
	for _, a := range args {
		info, err := os.Stat(a)
		if err != nil {
			fmt.Fprintf(stderr, "specarch: cannot read %s: %v\n", a, err)
			ioError = true
			continue
		}
		if !info.IsDir() {
			addFile(a)
			continue
		}
		var found []string
		err = filepath.WalkDir(a, func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if !d.IsDir() && validate.KindOf(d.Name()) != validate.KindNone {
				found = append(found, p)
			}
			return nil
		})
		if err != nil {
			fmt.Fprintf(stderr, "specarch: cannot read %s: %v\n", a, err)
			ioError = true
		}
		if err == nil && len(found) == 0 {
			fmt.Fprintf(stderr, "specarch: %s holds no %s or %s file; name a folder that does\n", a, "*"+validate.DesignSuffix, "*"+validate.ImplementationSuffix)
			ioError = true
		}
		sort.Strings(found)
		for _, f := range found {
			addFile(f)
		}
	}
	return files, ioError
}

func plural(n int, word string) string {
	if n == 1 {
		return "1 " + word
	}
	return fmt.Sprintf("%d %ss", n, word)
}
