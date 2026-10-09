# CLAUDE.md

Standing rules for agents working in this repository.

## History

Every item appends to that day's file in `history/` (`history/YYYY-MM-DD.md`)
in the same commit as its work: what changed and why, the owner's decisions,
commit hashes, handoffs processed and deleted, what was left out on purpose,
and open questions. Summary first, newest last, plain words.

Design files, schemas, docs and the README carry no change log: no
"previously", "changed from", "updated on" or "new in". They always read as
the current truth; how they got there belongs only in `history/` and git.
Version numbers stay.

## No de-ai pass, and no AI traces

No de-ai pass is run on anything in this repository: not on the history,
the docs, the README, commit messages or reports. This overrides the
global instruction to invoke the de-ai skill on prose. The rule against
AI traces still holds in full: no `Co-Authored-By` line naming the
assistant, no "Generated with" footer, and no model name or reference to
the assistant or the session in files, commits or pull requests.

## Public repository

This repository is public. It holds generic material only. Never put an
employer's, client's or product's name, data, code or internal detail in it;
generalise or leave it out, and name what was left out in the item report and
the history file.

## Handoffs

Handoffs arrive as files in the handoff mailbox folder named in the umbrella
repository's `CLAUDE.md`. Read it at the start of each SpecArch item. Check a
handoff for leaks before it becomes work here. Once a handoff is fully
processed and its work is pushed, delete that file and name it in the report
and the history file. Make no other change in that folder.

## How SpecArch is built

- SpecArch is a compiler for specifications (`docs/principles.md`): it
  reads an incomplete or wrong definition, never refuses to read it and
  never silently drops it; every problem goes into the one problems file
  and is marked at the entry in every output. Every design choice and
  decision record checks it: does this keep the definition readable when
  it is incomplete, and does every problem surface in the problems file
  and at the entry? A rule that refuses something is an error at that
  entry, never a definition left unread.
- Follow `docs/principles.md` (the Low IQ Tax): full-word keys, one way to
  say one thing, no placeholders, ambiguity is an error, summary first.
- Design and implementation stay apart: `*.specarch-design.yaml` holds the
  design, `<name>.<stack>.specarch-implementation.yaml` holds one stack's
  choices. Nothing stack-specific goes into a design file.
- Spec first: a change to the `specarch` command changes
  `spec/specarch.specarch-design.yaml` (and its implementation file) before
  or with the code, and the validator must pass on `spec/` and `examples/`.
- Dependencies: OSI licences only, pinned, with an SBOM scan (syft, grype or
  osv-scanner, govulncheck) recorded in the commit message and in the Go
  implementation file's `libraries`.
- Before adopting a standard (CEL, JSON Schema, OpenAPI, SQL, a language's
  rules) or deviating from one, find out why it is the way it is, and write
  that reason next to the choice in the design decisions. Never relax a
  standard's rule for convenience without saying what it protects against
  and why that does not apply.
- Types are concrete and width-aware (`docs/conventions.md`, Types).
- The validator has two builds, Go (root) and Swift (`swift/`). A change to
  the validator is made in both, a new case goes into `spec/tests/` with
  its design test, and both must print the same output. The documents and
  generators are Go only.
- Contributions from outside are the owner's decision; never merge one.
- Commits and pushes follow the repository's switches (`agentq policy
  specarch`).
