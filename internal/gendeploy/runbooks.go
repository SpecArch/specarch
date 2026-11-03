package gendeploy

import (
	"fmt"
	"sort"
	"strings"

	"github.com/SpecArch/specarch/internal/mark"
)

// runbookFiles writes the release, the rollback and each migration of the
// deployment stage as runbooks, once at the output's root: a shell
// script that prints each step with its check and goes on only when the
// person running it answers yes, so the steps run in the order the stage
// gives them and none is skipped; --yes answers every step for a run
// whose checks are made elsewhere.
func (g *gen) runbookFiles() {
	if rel, ok := obj(g.spec["release"]); ok {
		g.file("release.sh", g.runbook("release.sh", "/release", "the release", list(rel["steps"]), nil, "", false))
	}
	if rb, ok := obj(g.spec["rollback"]); ok {
		g.file("rollback.sh", g.runbook("rollback.sh", "/rollback", "the rollback", list(rb["steps"]), nil, "", false))
	}
	migrations := obj0(g.spec["migrations"])
	keys := make([]string, 0, len(migrations))
	for k := range migrations {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		m := obj0(migrations[k])
		back, hasBack := m["rollback"]
		var backSteps []any
		if hasBack {
			backSteps = list(back)
		}
		g.file("migrations/"+k+".sh", g.runbook(k+".sh", "/migrations/"+mark.Escape(k), "the migration "+k, list(m["steps"]), backSteps, text(m["description"]), true))
	}
}

// runbook writes one procedure as a script. A migration takes --rollback:
// with rollback steps it runs those instead, and a migration without them
// refuses it with its description, which says why it cannot be reversed.
func (g *gen) runbook(file, pointer, what string, steps, rollback []any, description string, migration bool) string {
	var b strings.Builder
	b.WriteString("#!/bin/sh\n")
	b.WriteString(g.header(nil, what+" of the deployment stage as a runbook"))
	b.WriteString(g.marks(pointer))
	b.WriteString("# Each step is printed with its action and its check, and the script goes on only when the person running it answers yes; --yes answers every step, for a run whose checks are made elsewhere.\n")
	if description != "" {
		b.WriteString("# " + oneLine(description) + "\n")
	}
	usage := "usage: " + file + " [--yes]"
	if rollback != nil {
		usage += " [--rollback]"
	}
	b.WriteString("set -u\n")
	b.WriteString("yes=false\n")
	if rollback != nil {
		b.WriteString("back=false\n")
	}
	b.WriteString("for a in \"$@\"; do\n  case \"$a\" in\n    --yes) yes=true ;;\n")
	if rollback != nil {
		b.WriteString("    --rollback) back=true ;;\n")
	} else if migration {
		b.WriteString("    --rollback) echo " + shQuote(file+": "+what+" has no rollback steps: "+oneLine(description)) + " >&2; exit 2 ;;\n")
	}
	b.WriteString("    *) echo " + shQuote(usage) + " >&2; exit 2 ;;\n  esac\ndone\n")
	b.WriteString("step() {\n")
	b.WriteString("  printf '\\nStep %s of %s: %s\\n  %s\\n  Check: %s\\n' \"$1\" \"$2\" \"$3\" \"$4\" \"$5\"\n")
	b.WriteString("  if $yes; then return 0; fi\n")
	b.WriteString("  printf 'Done, and the check holds? [yes/no] '\n")
	b.WriteString("  read -r answer\n")
	b.WriteString("  case \"$answer\" in\n    yes | y) ;;\n    *) echo " + shQuote(file+": stopped at step") + " \"$1, $3: answer yes once the check holds, and run it again\" >&2; exit 1 ;;\n  esac\n}\n")
	writeSteps := func(steps []any, at, indent string) {
		for i, s := range steps {
			st := obj0(s)
			b.WriteString(fmt.Sprintf("%s# specarch: #%s/%d\n", indent, at, i))
			b.WriteString(fmt.Sprintf("%sstep %d %d %s %s %s\n", indent, i+1, len(steps), shQuote(oneLine(text(st["name"]))), shQuote(oneLine(text(st["action"]))), shQuote(oneLine(text(st["check"])))))
		}
	}
	if rollback != nil {
		b.WriteString("if $back; then\n")
		b.WriteString("  echo " + shQuote("Reversing "+what) + "\n")
		writeSteps(rollback, pointer+"/rollback", "  ")
		b.WriteString("  echo " + shQuote(file+": every rollback step done") + "\n  exit 0\nfi\n")
	}
	b.WriteString("echo " + shQuote(strings.ToUpper(what[:1])+what[1:]) + "\n")
	writeSteps(steps, pointer+"/steps", "")
	b.WriteString("echo " + shQuote(file+": every step done") + "\n")
	return b.String()
}
