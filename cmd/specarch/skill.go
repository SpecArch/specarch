package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/SpecArch/specarch/internal/capability"
	"github.com/SpecArch/specarch/internal/disk"
)

// runSkill writes the agent skill for SpecArch, in the Agent Skills format,
// into <out>/specarch (ADR-214), or with --check compares it with the disk.
func runSkill(args []string, stdout, stderr io.Writer) int {
	out, check, dryRun := "", false, false
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "--check":
			check = true
		case a == "--dry-run":
			dryRun = true
		case a == "--out":
			if i+1 >= len(args) {
				fmt.Fprintf(stderr, "specarch skill: --out needs a folder, the one the skill's folder specarch goes in\n\n%s", usage)
				return 2
			}
			out = args[i+1]
			i++
		case strings.HasPrefix(a, "--out="):
			out = strings.TrimPrefix(a, "--out=")
		case strings.HasPrefix(a, "-") && len(a) > 1:
			fmt.Fprintf(stderr, "specarch skill has no option %s; its options are --out, --check and --dry-run\n\n%s", a, usage)
			return 2
		default:
			fmt.Fprintf(stderr, "specarch skill takes no argument, given %s; give the folder with --out\n\n%s", a, usage)
			return 2
		}
	}
	if out == "" {
		fmt.Fprintf(stderr, "specarch skill needs --out, the folder the skill's folder specarch goes in, such as .agents/skills\n\n%s", usage)
		return 2
	}
	if check && dryRun {
		fmt.Fprintf(stderr, "specarch skill: --check and --dry-run are not given together: --check compares the files on disk and exits 1 when one differs, --dry-run lists what it would write\n\n%s", usage)
		return 2
	}
	commands, err := ownCommands()
	if err != nil {
		fmt.Fprintf(stderr, "specarch skill: the command definitions built into this program cannot be read: %v\n", err)
		return 2
	}
	dir := filepath.Join(out, capability.SkillName)
	if msg := notTheSkill(dir); msg != "" {
		fmt.Fprintf(stderr, "specarch skill: %s\n", msg)
		return 2
	}
	files := capability.Skill(listing(commands, false), commands)
	held := map[string]bool{}
	for _, f := range files {
		held[f.Path] = true
	}
	stale, err := staleReferences(dir, held)
	if err != nil {
		fmt.Fprintf(stderr, "specarch skill: cannot read %s: %v\n", filepath.Join(dir, "references"), err)
		return 2
	}
	if check {
		differs := 0
		for _, f := range files {
			p := filepath.Join(dir, filepath.FromSlash(f.Path))
			old, err := os.ReadFile(p)
			switch {
			case errors.Is(err, fs.ErrNotExist):
				fmt.Fprintf(stdout, "%s: missing; run specarch skill --out %s without --check\n", p, out)
				differs++
			case err != nil:
				fmt.Fprintf(stderr, "specarch skill: cannot read %s: %v\n", p, err)
				return 2
			case !bytes.Equal(old, []byte(f.Text)):
				fmt.Fprintf(stdout, "%s: differs from what skill writes; run specarch skill --out %s without --check\n", p, out)
				differs++
			}
		}
		for _, p := range stale {
			fmt.Fprintf(stdout, "%s: not the skill's, a reference no command of this release has; run specarch skill --out %s without --check to remove it\n", p, out)
			differs++
		}
		fmt.Fprintf(stderr, "specarch skill --check: %s checked, %s differ\n", plural(len(files), "file"), plural(differs, "file"))
		if differs > 0 {
			return 1
		}
		return 0
	}
	w := disk.For(dryRun, stdout)
	written := 0
	for _, f := range files {
		p := filepath.Join(dir, filepath.FromSlash(f.Path))
		if old, err := os.ReadFile(p); err == nil && bytes.Equal(old, []byte(f.Text)) {
			continue
		}
		if err := w.WriteFile(p, []byte(f.Text), 0o644); err != nil {
			fmt.Fprintf(stderr, "specarch skill: cannot write %s: %v\n", p, err)
			return 2
		}
		if !dryRun {
			fmt.Fprintln(stdout, p)
		}
		written++
	}
	for _, p := range stale {
		if err := w.Remove(p); err != nil {
			fmt.Fprintf(stderr, "specarch skill: cannot write %s, a reference this release does not hold, to remove it: %v\n", p, err)
			return 2
		}
		if !dryRun {
			fmt.Fprintf(stdout, "removed %s\n", p)
		}
	}
	current := len(files) - written
	if dryRun {
		fmt.Fprintf(stderr, "specarch skill: %s to write, %s to remove, %s already current; nothing written (--dry-run)\n", plural(written, "file"), plural(len(stale), "file"), plural(current, "file"))
		return 0
	}
	fmt.Fprintf(stderr, "specarch skill: %s written, %s removed, %s already current, in %s\n", plural(written, "file"), plural(len(stale), "file"), plural(current, "file"), dir)
	return 0
}

// notTheSkill says why a folder is not one specarch skill wrote: it holds
// files but no SKILL.md whose front matter names the skill. Nothing when
// the folder is the skill's, empty or not there.
func notTheSkill(dir string) string {
	entries, err := os.ReadDir(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return ""
	}
	if err != nil {
		return fmt.Sprintf("cannot read %s, the folder the skill goes in: %v", dir, err)
	}
	if len(entries) == 0 {
		return ""
	}
	skill := filepath.Join(dir, "SKILL.md")
	data, err := os.ReadFile(skill)
	if err != nil {
		return fmt.Sprintf("%s holds files but no SKILL.md, so it is not a skill specarch skill wrote, and nothing is written there; give --out a folder whose specarch folder is the skill's or does not exist", dir)
	}
	if !namesSkill(string(data)) {
		return fmt.Sprintf("%s does not name the skill %s in its front matter, so %s is not a skill specarch skill wrote, and nothing is written there; give --out a folder whose specarch folder is the skill's or does not exist", skill, capability.SkillName, dir)
	}
	return ""
}

// namesSkill says whether a SKILL.md's front matter names the skill.
func namesSkill(text string) bool {
	rest, ok := strings.CutPrefix(text, "---\n")
	if !ok {
		return false
	}
	front, _, ok := strings.Cut(rest, "\n---\n")
	if !ok {
		return false
	}
	for _, line := range strings.Split(front, "\n") {
		if strings.TrimSpace(line) == "name: "+capability.SkillName {
			return true
		}
	}
	return false
}

// staleReferences are the references, .md files in references/, that the
// skill does not hold, by path, in name order.
func staleReferences(dir string, held map[string]bool) ([]string, error) {
	entries, err := os.ReadDir(filepath.Join(dir, "references"))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") || held["references/"+e.Name()] {
			continue
		}
		out = append(out, filepath.Join(dir, "references", e.Name()))
	}
	sort.Strings(out)
	return out, nil
}
