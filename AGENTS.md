# AGENTS.md

Instructions for agents working in this repository: SpecArch, a
specification language and its toolchain, the `specarch` command and the
`specarch-gen-*` plug-ins. The README says what SpecArch is;
`CONTRIBUTING.md` how a change lands.

## This repository

- Public, under Apache-2.0: generic material only, never an employer's,
  a client's or a product's name, data, code or internal detail.
- Spec first, here too: a change to the `specarch` command changes its
  design in `spec/` (and its implementation files) before or with the
  code. Each command is defined in `spec/design/commands/<command>.yaml`;
  the program builds that folder in, and `help`, `guide` and
  `capabilities` print from it, so a command or an option without its
  definition fails the tests.
- Two validator builds, Go (the root module) and Swift (`swift/`): a
  validator change goes into both, a new case into `spec/tests/` with its
  design test, and both print the same output. Documents, generators and
  the other commands are Go only.
- What changed and why goes into `history/YYYY-MM-DD.md`; bugs into
  `BUG_OUTSTANDING.md` and `BUG_HISTORY.md`.

Build and check, from the repository root:

    go build ./...
    go test ./...                              # includes the conformance suite in spec/tests
    go run ./cmd/specarch validate spec examples
    sh scripts/landing-check.sh                # every check a change must pass, Go and Swift
    sh scripts/regenerate.sh                   # rewrite every generated document and file

The section below is what `specarch guide --agent` prints up to the
plug-ins it finds; `go test ./cmd/specarch -run TestAgentsFile -update`
writes it again after a change to the program's facts or commands.

<!-- specarch guide --agent: begin -->
# SpecArch 1.2.0 for agents

SpecArch is a compiler for specifications: it reads a specification written
in YAML, lists every problem in it, and writes documents and code from it.
`specarch capabilities --json` holds every fact below as JSON.

## Formats

- meta-model (version key specarch): writes 1.0.0; reads 1.0.0 and any later
  1.0 patch, 0.3, 0.2, 0.1, an earlier one with a warning naming specarch
  upgrade.
- implementation file (version key specarchImplementation): writes 1.0.0;
  reads 1.0.0 and any later 1.0 patch, 0.1, an earlier one with a warning
  naming specarch upgrade.
- record (version key specarchRecord): writes 1.0.0; reads 1.0.0 and any
  later 1.0 patch, 0.1, an earlier one with a warning naming specarch
  upgrade.
- idiom (version key specarchIdiom): writes 1.0.0; reads 1.0.0 and any later
  1.0 patch, 0.1, an earlier one with a warning naming specarch upgrade.
- estate (version key specarchEstate): writes 1.0.0; reads 1.0.0 and any
  later 1.0 patch, 0.1, an earlier one with a warning naming specarch
  upgrade.
- vocabulary (version key specarchVocabulary): writes 1.0.0; reads 1.0.0 and
  any later 1.0 patch, 0.1.

## Workflow

1. Pin the program: record the module version and checksum `go version -m`
   prints for the installed specarch in the repository's agent instructions,
   and check them at the start of each session; `specarch capabilities
   --json` names the version, the formats and the plug-ins this install has.
2. Specification first: change the specification (specarch.yaml, its stage
   folders, the implementation files) before the code made from it, never a
   generated file; a generated file's first lines say which command made it.
3. After every edit run `specarch validate <folder>`: exit 0 is valid, 1
   lists the errors one per line, 2 is a usage or read error.
4. Run `specarch gaps <folder>` for the open questions and what each holds
   up; take each must or should question to the owner and record the answer
   with `specarch decide`.
5. Write what people read with `specarch document <target> --out <folder>
   <folder>` (techspec, requirements, problems, questions, manual and the
   others); once the owner has read them, they record the approval with
   `specarch approve`.
6. Write code with `specarch generate <target> <folder>`; it refuses an
   unapproved specification unless `--unapproved` makes the output a draft.
   `--dry-run` lists what it would write; `--check` fails when the files on
   disk differ, so run it before each commit and in CI.
7. For a system that exists already: `specarch extract <source>` once per
   surface (documents, database, router, code), `specarch merge` the trees,
   then validate and gaps; `specarch check` compares the specification with
   the code's own tables later.
8. When a release moves the formats: `specarch upgrade --dry-run <folder>`
   lists the changes and the steps left to the author, `specarch upgrade
   <folder>` makes them.

## Reading problems

- A problem line is file:line:column: severity: path: rule: message [id],
  the same in validate's output, in problems.txt and in --json as fields.
- The severity is error (the specification is invalid: generate and approve
  refuse it, document writes with the error marked and exits 1), warning
  (valid, but something is missing or doubtful), question (an open question;
  must and should questions block what they name) or note.
- The path is the YAML path of the entry in the merged specification; the
  rule names the check, the same word in every release; the id is stable
  across runs, so a problem can be tracked.
- problems.txt, which `specarch document problems` writes beside
  problems.sarif, starts with the counts, then lists every problem, one line
  each, with the file paths relative to it; each one is also marked above
  its entry in the YAML files, on a `# specarch-problem:` comment line, and
  in the documents written.
- Fix an error at the file and line it names, then validate again; do not
  silence a warning by deleting the entry it is at.

## Ask the owner

- Every open must or should question, and any choice between two sources
  that disagree (a document and the code): the agent records the question,
  never picks an answer; the owner answers and `specarch decide` records it.
- Approving a specification: `specarch approve --by <stakeholder>` records a
  person's approval, so only that person's word runs it.
- Generating a draft with --unapproved for anything that will be used, and
  overwriting a file built by hand.
- Any error the agent cannot settle from the specification and its sources,
  a refusal it does not understand, and anything a remedy does not cover.

## Limits

- No promise of complete generation: a plug-in writes what its target covers
  from the specification, marks each problem at its entry, and names what it
  does not write; the rest is written by hand.
- Hand-written islands: keep code written by hand in files a target does not
  write, since generate writes every file its plug-in answers (sqlc and
  scripts never write over a file they did not write); `generate --check`
  reads a file without the generated header as built by hand and says
  whether it matches the output but for the header; an element an
  implementation file marks ownedBy a stakeholder is generated nowhere.
- A specification is never complete: an incomplete one is still read, and
  every gap is a problem or a question rather than a refusal to read.
- The program never runs or deploys the code it generates; a plug-in is
  found on PATH, answers on standard output and never touches the disk, and
  specarch writes its files.
- Prompts: none. Every command runs without asking anything, so no --yes
  exists; a step that needs a person is a command the person runs (decide,
  approve).

## Conventions

- --json, on any command, given anywhere before -- but as the value of an
  option that takes one, prints one JSON object on standard output (key
  specarchOutput; model and capabilities print their own, specarchModel and
  specarchCapabilities, on exit 0) and, on exit 2, one error object on
  standard error (key specarchError, a stable code), and nothing else on
  either.
- Without --json, results go to standard output and counts and reasons to
  standard error, as plain text with no colour or terminal codes, whether or
  not the output is a terminal; NO_COLOR is honoured by having no colour.
- --dry-run, on every command that writes, lists each file it would write or
  remove on standard output and writes nothing.
- Exit status: 0 done, 1 the command ran and found what it reports (errors,
  differences, open questions), 2 a usage or read error or a failed plug-in;
  each command's own exit codes and remedies are in its tool entry.
- Arguments after -- are paths, even when they start with -.

Error codes of --json:

- usage: The command line is wrong: an unknown command or option, or a
  missing argument; the message says which.
- read: A path given or needed could not be read.
- write: A file could not be written.
- plugin: A generator plug-in was not found on PATH, failed, or answered
  something that is not specarch's JSON.
- failed: Any other failure of exit status 2; the message and detail say
  what happened.

## Commands

- `specarch approve --by <by> [--date <date>] [--dry-run] <paths>...`
  Record that the documents were read and the specification is approved.
  exit 0: the approval was recorded
  exit 1: refused; the specification has errors or open questions, the
  stakeholder is unknown, no document is configured, or a document is not
  current
  exit 2: usage error, or a file that could not be read or written
- `specarch capabilities [--json]`
  List what this program and the plug-ins on PATH can do; with --json as a
  tool listing a program reads.
  exit 0: printed
  exit 2: usage error
- `specarch check <specification> [--allowlist <allowlist>] [--since <since>] <dumps>...`
  Compare a specification with the dumps of the running code, for a CI gate.
  exit 0: every difference is one the allowlist names or a report, and every
  line of the allowlist names a difference and was there at --since
  exit 1: a difference the allowlist does not name, a line of the allowlist
  that names none, repeats one or was added since --since, or a
  specification validate reports an error in
  exit 2: usage error; a dump, the specification or the allowlist that could
  not be read; a dump of no format check reads, or one its reader refuses; a
  catalogue dump with no implementation file to compare it through, or with
  one whose tables cannot be written for PostgreSQL; or a revision git does
  not know
- `specarch decide <question> <folder> --by <by> --answer <answer> [--source <source>] [--date <date>] [--dry-run]`
  Answer an open question with a decision record and take the question out.
  exit 0: the record is written and the question taken out
  exit 1: refused; the question, the stakeholder or the source is unknown,
  the specification tracks origin and no source is given, design is not a
  stage, or the entry cannot be edited as text; or written, while the
  specification has errors
  exit 2: usage error, or a file that could not be read or written
- `specarch decision rests <decision> <folder>`
  Print the decisions that rest on a decision, nearest first.
  exit 0: the decisions are printed
  exit 1: refused, the decision is not one of the specification's; or
  printed, while the specification has errors
  exit 2: usage error, or a folder that could not be read
- `specarch decision why <decision> <folder>`
  Print the chain of decisions a decision rests on, earliest first.
  exit 0: the chain is printed
  exit 1: refused, the decision is not one of the specification's; or
  printed, while the specification has errors
  exit 2: usage error, or a folder that could not be read
- `specarch derive [--dry-run] <paths>...`
  Write a draft test for every derived case no test covers.
  exit 0: the tests were written, or there was nothing to write
  exit 1: a specification has errors other than the missing tests it writes,
  or a draft's name is taken by another draft or by a test of another
  subject
  exit 2: usage error, a path that could not be read or written, or a
  specification that keeps its tests in the root file
- `specarch diff <old> <new>`
  Compare two versions of a specification and check the release between
  them.
  exit 0: every check passes
  exit 1: a check fails, there is no release record for the new version, or
  a specification has errors
  exit 2: usage error, or a path that could not be read
- `specarch document <target> [--out <out>] [--check] [--audience <audience>] [--dry-run] <paths>...`
  Write a document from a specification.
  exit 0: written, or with `--check` the output is current
  exit 1: a specification has an error (after its files are written, or with
  `--check` compared), a marker is wrong, or with `--check` the output
  differs
  exit 2: usage error, a target this build does not offer, --audience with a
  value other than external, without --out or for the problems target, no
  output folder in any specification given (or, for the handover target, no
  handover in any), the implementation files of one naming different output
  folders, or a file that could not be read or written
- `specarch extract <source> --out <out> [--implementation <implementation>] [--facts <facts>] [--routes <routes>] [--language <language>] [--vocabulary <vocabulary>] [--given-outside] [--implementation-out <implementation-out>] [--table] [--source-key <source-key>] [--dry-run] <paths>...`
  Write a specification from existing code or documents.
  exit 0: the specification was written
  exit 1: the surface could not be read as the source expects: a dump that
  does not parse or is stale, a dump whose repository is no gitlink of the
  repository holding it, is not checked out, pins a commit its history does
  not hold or whose checkout under the path is not the pinned commit's, a
  code-facts dump another parser version made, whose files are not the
  tracked files of its folder the reader reads, or, for pages, whose folder
  does not hold the router's folder, a route table that lists a method and
  path pair twice, a permission table that lists a grant, a rule or a gate
  twice, holds a gate with no check, with both a setting and a kind of
  caller or neither, or skipped for a kind other than client, names a
  matcher the reader does not know, or holds a rule without one of its four
  keys, a vocabulary that does not parse, lacks a key, holds one it does not
  have, gives a number, a duration or a language tag in another form, or
  lists one word twice in a list or in two lists of one group, a route table
  given with --routes from another repository or from a commit on another
  line of history than the permission table's, a router root that gives one
  route twice or holds no page, a path with changes not committed, untracked
  files, a shallow clone, a path outside a git repository, a document that
  is not Markdown, a file that is not an OpenAPI 3.0 or 3.1 document, a file
  that is not BPMN 2.0 XML or holds no process, a workflow table that does
  not parse, is stale, lacks a key of the format or holds another, names a
  step kind other than approval, operation or other, lists a workflow twice
  or gives a workflow, a step or a form field no name, a gateway table that
  does not parse, is stale, lacks a key of the format or holds another,
  gives a row no value or a value twice, or names a gateway or a claim that
  is no name, or an implementation file that does not parse as YAML
  exit 2: usage error, a source this build does not offer, --implementation
  given to a source other than go, swift, javascript, dart or pages, --facts
  given to a source other than pages, --routes given to a source other than
  permissions, --language, --vocabulary or --given-outside given to a source
  other than documents, --table given to a source other than workflows,
  --implementation-out given to a source other than database, go or gateway,
  or naming a file that exists or whose name does not end in .yaml, a
  language that is not a BCP 47 tag, a language SpecArch ships no vocabulary
  for given without --vocabulary, a --language other than the vocabulary's
  own, more than one permission table, or a path that could not be read or
  written
- `specarch facts <language> <folder> <dump> [--dry-run]`
  Write the code-facts dump extract reads of a committed folder.
  exit 0: the dump was written
  exit 1: the folder could not be read as committed code: it is in no git
  repository, it has changes not committed or files not tracked, no commit
  has changed it, the dump file is in a repository that does not hold the
  code's, or the reader's toolchain failed to build or to run it
  exit 2: usage error, a language this build does not read, a folder that
  does not exist, a dump file whose folder does not exist or that could not
  be written, or a submodule folder with a quote or a backslash
- `specarch gaps <paths>...`
  List the open questions and what they hold up.
  exit 0: no must or should question is open and no specification has an
  error
  exit 1: at least one must or should question is open, or a specification
  or an estate has an error
  exit 2: usage error, or a path that could not be read
- `specarch generate <target> [--out <out>] [--check] [--estate <estate>]... [--unapproved] [--dry-run] <paths>...`
  Write code or data from a specification.
  exit 0: written, a draft included, or with `--check` the output is current
  exit 1: a specification has an error, an open question blocks what the
  target reads or the specification is not approved and `--unapproved` was
  not given, the plug-in reported an error (what it answered is written or
  checked), or with `--check` the output differs
  exit 2: usage error, no generator for the target (not built in and no
  plug-in on PATH), the plug-in failed or answered badly, no output folder
  in any specification given, or a file that could not be read or written
- `specarch guide [--agent]`
  Print the order of work, for a person or, with --agent, for an agent.
  exit 0: printed
  exit 2: usage error
- `specarch help [<topic>...]`
  Explain an area of SpecArch or one command, from the text built into the
  program.
  exit 0: printed
  exit 2: no such topic
- `specarch idioms <paths>...`
  List the idioms each implementation file uses, and how.
  exit 0: the idioms were listed
  exit 2: usage error, a path that could not be read, or a specification
  with errors
- `specarch idioms diff <idiom> <paths>...`
  Print a shipped idiom's parts beside each override's.
  exit 0: the overrides were printed, or there was none
  exit 2: usage error, an idiom this program does not ship, a path that
  could not be read, or a specification with errors
- `specarch merge --out <out> [--vocabulary <vocabulary>]... [--dry-run] <trees>...`
  Merge the partial specifications the readers wrote into one.
  exit 0: the merged specification was written
  exit 1: a tree could not be merged: it does not track origin, validate
  reports an error in it, it holds implementation files or records or a test
  that is not the draft derive writes for it, or it declares a source
  another tree declares differently, or whose paths changed after the commit
  a tree read them at; or a vocabulary that is not one this build reads
  exit 2: usage error, fewer than two trees, two vocabularies of one
  language, or a folder or file that could not be read or written
- `specarch model <paths>...`
  Print the specifications as one JSON document, for a program.
  exit 0: printed
  exit 2: usage error, a path could not be read, or a folder holds no
  specification, no implementation file and no estate
- `specarch try <algorithm> <folder> [--example <example>] [--inputs <inputs>] [--file <file>]`
  Evaluate an algorithm on its worked examples or on given inputs and print
  each step.
  exit 0: every evaluation is computed and every example evaluated holds
  exit 1: refused, the algorithm or the example is not the specification's,
  or the formula does not check; or an example fails, cannot be computed or
  cannot be read; or the specification has errors
  exit 2: usage error, or a folder or file that could not be read
- `specarch upgrade [--dry-run] [--to <to>] <paths>...`
  Move a specification and its files to the newest version of each format,
  or list what moving them changes.
  exit 0: every change the move names was made, or with --dry-run can be
  made, and no step is left to the author
  exit 1: a step is left to the author
  exit 2: usage error, a path that could not be read or written, or a
  version --to names that this program does not write
- `specarch validate <paths>...`
  Check specifications and implementation files.
  exit 0: every input is valid
  exit 1: at least one input has an error
  exit 2: usage error, a path could not be read, or a folder holds no
  specification, no implementation file and no estate
- `specarch version`
  Print the program version and the meta-model versions it reads.
  exit 0: printed
<!-- specarch guide --agent: end -->
