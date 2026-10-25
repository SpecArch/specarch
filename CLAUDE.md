# CLAUDE.md

Rules for agents here. The umbrella `MyOwnApps/CLAUDE.md` and
`~/.claude/CLAUDE.md` also apply; this file holds what is SpecArch's own.

## Public repository

- Generic material only: no employer's, client's or product's name, data,
  code or internal detail. Generalise or leave it out, and name what was
  left out in the item report and the history file.
- No AI traces, in full (the umbrella's public-repo rule): no
  `Co-Authored-By` naming the assistant, no "Generated with" footer, no
  model name or reference to the assistant or the session in files,
  commits or pull requests. No de-ai pass on anything here (umbrella,
  overriding the global de-ai rule).
- Handoffs: mailbox and rules in the umbrella `CLAUDE.md` (specarch
  handoffs). Read the mailbox at the start of each SpecArch item and check
  each handoff for leaks. Once a handoff is fully processed and its work
  pushed, delete the file and name it in the report and the history file.
  No other change in that folder.
- Commits and pushes follow `agentq policy specarch`. Contributions from
  outside are the owner's decision; never merge one.

## History

- Every item appends to `history/YYYY-MM-DD.md` (that day) in the same
  commit as its work: what changed and why, the owner's decisions, commit
  hashes, handoffs processed and deleted, what was left out on purpose,
  open questions. Summary first, newest last, plain words.
- Design files, schemas, docs and the README read as the current truth: no
  "previously", "changed from", "updated on", "new in". That belongs only
  in `history/` and git. Version numbers stay.

## Bugs

- In `BUG_OUTSTANDING.md` and `BUG_HISTORY.md`, one `BUG-NNN` entry each;
  not in GitHub issues (turned off). A new bug takes the next free number
  in `BUG_OUTSTANDING.md`.
- The fixing commit says `Fixes BUG-NNN` and moves the entry to
  `BUG_HISTORY.md` with that commit's hash, in the same commit.
- An outside pull request that claims to fix a listed bug is reported to
  the owner, never taken on that ground.

## How SpecArch is built

- **The goal** (`docs/principles.md`, Goal): a specification is layered
  human decisions, and SpecArch makes each one cheap to take, cheap to
  review and traceable to the decisions it rests on. Every item is judged
  by one question: does it make a human decision faster, more accurate or
  traceable?
- **A compiler for specifications** (`docs/principles.md`): it reads an
  incomplete or wrong definition, never refuses to read it, never silently
  drops it. Every problem goes into the one problems file and is marked at
  the entry in every output. A rule that refuses something is an error at
  that entry, never a definition left unread.
- **A specification is never complete** (`docs/principles.md`): people
  leave things out, and end users see the system only at the user
  acceptance test, where they ask for changes.
- Every design choice and decision record checks both: is an incomplete
  definition still readable, with every problem in the problems file and
  at the entry? Is a result fast to show early, and a change request on it
  cheap to take back into the specification (`docs/acceptance.md`)?
- The Low IQ Tax (`docs/principles.md`): full-word keys, one way to say
  one thing, no placeholders, ambiguity is an error, summary first.
- Design and implementation stay apart (`docs/conventions.md`): a
  specification's root `specarch.yaml` and stage folders hold the design;
  `implementation/<stack>/<name>.<stack>.specarch-implementation.yaml`
  holds one stack's choices. Nothing stack-specific goes into the design.
- **Spec first**: a change to the `specarch` command changes its design
  in `spec/` (and its implementation files) before or with the code, and
  this must pass:

      specarch validate spec examples

- Before adopting or deviating from a standard (CEL, JSON Schema, OpenAPI,
  SQL, a language's rules), find out why it is the way it is and write
  that reason next to the choice in the design decisions. Never relax its
  rule for convenience without saying what it protects against and why
  that does not apply.
- Types are concrete and width-aware (`docs/conventions.md`, Types).
- Two validator builds, Go (root) and Swift (`swift/`): a validator change
  goes into both, a new case into `spec/tests/` with its design test, and
  both print the same output. Documents and generators are Go only.
- Dependencies, beyond the umbrella's SBOM and licence rules: OSI licences
  only, pinned, scanned (syft, grype or osv-scanner, govulncheck), and the
  result recorded in the commit message and in `libraries` of
  `spec/implementation/go/specarch.go.specarch-implementation.yaml`.
