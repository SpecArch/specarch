package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/SpecArch/specarch/internal/approval"
	"github.com/SpecArch/specarch/internal/disk"
	"github.com/SpecArch/specarch/internal/generate"
	"github.com/SpecArch/specarch/internal/source"
	"github.com/SpecArch/specarch/internal/validate"
)

// runApprove records that a stakeholder read the documents of a
// specification and approves it for code generation. It refuses a
// specification with errors, one with an open must or should question, a
// stakeholder the specification does not declare, and documents on disk
// that are not what the specification generates now.
func runApprove(args []string, stdout, stderr io.Writer) int {
	by, date := "", ""
	w := disk.Real
	var paths []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "--":
			paths = append(paths, args[i+1:]...)
			i = len(args)
		case a == "--by" || a == "--date":
			if i+1 >= len(args) {
				fmt.Fprintf(stderr, "specarch approve: %s needs a value\n\n%s", a, usage)
				return 2
			}
			if a == "--by" {
				by = args[i+1]
			} else {
				date = args[i+1]
			}
			i++
		case a == "--dry-run":
			w = disk.DryRun(stdout)
		case strings.HasPrefix(a, "--by="):
			by = strings.TrimPrefix(a, "--by=")
		case strings.HasPrefix(a, "--date="):
			date = strings.TrimPrefix(a, "--date=")
		case strings.HasPrefix(a, "-") && len(a) > 1:
			fmt.Fprintf(stderr, "specarch approve has no option %s; its options are --by, --date and --dry-run\n\n%s", a, usage)
			return 2
		default:
			paths = append(paths, a)
		}
	}
	if by == "" {
		fmt.Fprintf(stderr, "specarch approve needs --by <stakeholder>, the role that read the documents and approves\n\n%s", usage)
		return 2
	}
	if len(paths) == 0 {
		fmt.Fprintf(stderr, "specarch approve needs at least one folder\n\n%s", usage)
		return 2
	}
	if date == "" {
		date = time.Now().Format("2006-01-02")
	} else if _, err := time.Parse("2006-01-02", date); err != nil {
		fmt.Fprintf(stderr, "specarch approve: the date is written as YYYY-MM-DD, not %q\n", date)
		return 2
	}
	specs, status := loadSpecs(paths, "approve", stdout, stderr)
	if status != 0 {
		return status
	}
	for _, l := range specs {
		if status := approve(l, by, date, w, stdout, stderr); status != 0 {
			return status
		}
	}
	return 0
}

func approve(l loaded, by, date string, w *disk.Writer, stdout, stderr io.Writer) int {
	root := l.spec.Root
	must, should, _ := validate.Questions(root)
	if must+should > 0 {
		verb := "hold"
		if must+should == 1 {
			verb = "holds"
		}
		fmt.Fprintf(stderr, "specarch approve: %s has %s that %s things up; answer them first, as specarch gaps lists them\n", l.spec.Dir, plural(must+should, "open question"), verb)
		return 1
	}
	if source.Child(source.Child(root, "stakeholders"), by) == nil {
		fmt.Fprintf(stderr, "specarch approve: %s is not a stakeholder of %s; approve as one of its stakeholders, by key\n", by, l.spec.Dir)
		return 1
	}
	var documents []string
	stale := 0
	for _, target := range generate.DocumentTargets {
		if !generate.BuiltDocuments[target] || generate.RefinementDocuments[target] {
			continue
		}
		folder := configuredFolder(l, target)
		if folder == "" {
			continue
		}
		// What cannot be drawn for the site was said when document wrote it.
		impls, st := l.documentInputs(target, folder, io.Discard)
		text, _ := generate.Document(target, root, relSlash(folder, l.spec.RootFile), impls, st)
		p := filepath.Join(folder, generate.DocumentName(target))
		old, err := os.ReadFile(p)
		switch {
		case os.IsNotExist(err):
			fmt.Fprintf(stdout, "%s: missing; run specarch document %s and read it\n", p, target)
			stale++
		case err != nil:
			fmt.Fprintf(stderr, "specarch: cannot read %s: %v\n", p, err)
			return 2
		case !bytes.Equal(old, []byte(text)):
			fmt.Fprintf(stdout, "%s: differs from what the specification says now; run specarch document %s and read it again\n", p, target)
			stale++
		}
		documents = append(documents, target)
	}
	if len(documents) == 0 {
		fmt.Fprintf(stderr, "specarch approve: no document is configured for %s; add a document target with its output folder to an implementation file, run specarch document, and read it\n", l.spec.Dir)
		return 1
	}
	if stale > 0 {
		fmt.Fprintf(stderr, "specarch approve: %s of %s not what the specification says now, so nothing was approved\n", plural(stale, "document"), plural(len(documents), "document"))
		return 1
	}
	digest, err := approval.Digest(l.spec.Dir)
	if err != nil {
		fmt.Fprintf(stderr, "specarch approve: cannot read the files of %s: %v\n", l.spec.Dir, err)
		return 2
	}
	version := source.Str(source.Child(source.Child(root, "info"), "version"))
	rec := approval.Record{Version: version, ApprovedBy: by, Date: date, Documents: documents, Digest: digest}
	if err := approval.Write(w, l.spec.Dir, rec); err != nil {
		fmt.Fprintf(stderr, "specarch approve: cannot write the record: %v\n", err)
		return 2
	}
	if w.Dry() {
		fmt.Fprintf(stderr, "specarch approve: version %s of %s can be approved by %s; nothing written (--dry-run)\n", version, l.spec.Dir, by)
		return 0
	}
	fmt.Fprintf(stderr, "specarch approve: version %s of %s approved by %s; %s written beside it\n", version, l.spec.Dir, by, approval.Name(version))
	return 0
}

// configuredFolder is the output folder the implementation files name for
// a document target, or "" when none does.
func configuredFolder(l loaded, target string) string {
	for _, i := range l.impls {
		if o := source.Str(source.Child(source.Child(source.Child(i.Node, "targets"), target), "output")); o != "" {
			return filepath.Clean(filepath.Join(filepath.Dir(i.Path), filepath.FromSlash(o)))
		}
	}
	return ""
}
