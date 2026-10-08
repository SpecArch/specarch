# From documents and code to a specification

How an agent writes the specification of a system that already exists,
from its prose documents and its code, without inventing what they do not
say. The result is a partial specification: every element cites the
document section or the code line it came from, every disagreement
between them is a question, and `specarch gaps` shows what is still open
and which parts of the sources produced nothing. The owner reads that and
the generated documents, answers the questions, and the specification
grows towards complete.

`examples/lending-desk/` is this procedure run on a small public sample: a
desk manual (`sources/manual.md`), the Go service it describes
(`sources/code/`), the specification written from both (`spec/`) and the
documents made from it (`docs/`). The service and the manual disagree on
the loan period on purpose.

This is step 1 of bringing an existing system in, done by an agent by hand
with the toolchain checking each step. `specarch extract`, the verb that
would read the sources mechanically, is designed but not built and answers
with status 2. `docs/extraction.md` is the method behind it and
`docs/refinement.md` the design of origin, questions and approval.

## 1. Install and pin

The repository being specified pins one exact build of `specarch`, so
that every run of the agent and every reviewer sees the same rules.

With Go 1.26 or later, install by release tag:

    go install github.com/SpecArch/specarch/cmd/specarch@v0.3.0

or by commit, when what is needed is newer than the last tag:

    go install github.com/SpecArch/specarch/cmd/specarch@<full commit hash>

Go fetches the module through the module proxy and checks it against the
public checksum database, so the commit or tag names one exact content.
To see what is installed, and its checksum:

    go version -m "$(go env GOPATH)/bin/specarch"

The `mod` line names the module version (for a commit, a pseudo-version
ending in the first twelve characters of the hash) and its `h1:` checksum.
Write both in the repository's agent instructions; the agent runs the
command at the start of each session and stops if either differs.
`specarch version` prints the version of the source the program was built
from, which between two tags is the last tag's, so it is not the check.

The schemas are compiled into the program, so validation needs nothing
else. An editor that should check the YAML as it is typed points at the
same commit:

    # yaml-language-server: $schema=https://raw.githubusercontent.com/SpecArch/specarch/<commit or tag>/schema/specarch-design-0.1.schema.json

## 2. The folder

The specification lives in its own folder of the repository, such as
`spec/`, beside the documents and code it is read from. Its root file
declares the sources and says the specification tracks origin:

    specarch: "0.1"
    info: { title: <system>, version: 0.1.0, tracksOrigin: true }
    stages: [requirements, design, implementation]
    sources:
      <document-key>:
        kind: document
        title: <the document's own title>
        edition: <its version or date>
        url: <path to the file, from the root file>
        clauses:
          - { clause: "3.2", title: <heading> }
      <code-key>:
        kind: code
        title: <the service>
        edition: <the full commit hash that was read>
        url: <path to the code, from the root file>
        clauses:
          - { clause: internal/lending/routes.go, title: <what is there> }

One source per document file. One source for the code, at the commit the
agent read; reading a later commit is a new edition. `clauses` is the
outline: every section of the document that has a number or a heading,
and every file or folder of the code that holds behaviour (routes,
handlers, models, migrations, seeds, configuration). List the leaves of
the outline, not their parents: a parent listed beside its children
shows as producing nothing when everything cites a child.

A citation names its source, the clause, and what the source says there:

    cites:
      - { source: desk-manual, clause: "3.3", says: "The loan period is 21 days." }
      - { source: desk-code, clause: "lending/model.go:8", says: "LoanPeriod is 14 days." }

For a document the clause is the section number, or the heading when the
section has none. For code it is the path from the code source's root,
with `:line`, or with a space and `package.Function`. A citation falls
under the longest listed clause it equals or starts with, followed by a
dot, a colon, a slash or a space.

## 3. The procedure

### 3.1 Read the documents, section by section

For each section, in order:

1. Write every element the section states: stakeholders, needs,
   requirements, entities and their fields, permissions and roles,
   operations, configuration. Each gets `origin: stated` and a citation of
   that section, saying in plain words what the section says.
2. Where the section implies something without saying it, write it with
   `origin: inferred` and a `why` that says from what, or do not write it.
   Never fill a gap with a plausible value.
3. Where the section leaves open something an element needs, write the
   element as far as it is known and add a question (section 3.4).
4. Where two documents disagree, add a question that cites both.

A section that holds nothing the specification needs is still read; it
shows as producing nothing in the coverage, and that is the record that
it was read. If it should have produced something, cite it.

### 3.2 Read the code, surface by surface

The code says what the system does today. Read it by the surfaces of the
table in `docs/extraction.md`, from the place the running system reads
each fact, not from the file that looks like it: the routes the router
serves, the tables after every migration, the permissions where the
running check reads them (often seed scripts), the configuration the
program loads. For each element the code shows, compare it with what the
documents gave:

| Documents and code | What to write |
|---|---|
| agree | one element, `origin: stated`, citing the document section and the code line |
| disagree | the element as far as both agree, and a `must` question (`decision`) that cites both and says "the document says X, the code does Y"; the disputed key is left out, and the question blocks it |
| only the code has it | the element with `origin: inferred`, a `why` that starts "Undocumented, from code." and names the line, citing the code, and a `should` question asking the owner to confirm it |
| only the documents have it | the element as the documents state it, and a `should` question in `implementation/questions.yaml`, blocking `implementation`, that starts "Not built yet." and cites the section and the code where it would be |

Never choose between a document and the code. Whichever is right is the
owner's decision, recorded as a decision record when it is answered
(`docs/refinement.md`, The decision that closes a question).

### 3.3 Write the implementation file from the code

The as-built implementation file, `spec/implementation/go/<name>.go.specarch-implementation.yaml`,
holds what only the builders need: the language and its version, the
libraries with their licences, the package layout, and a mapping per
design element to the type, table or handler that builds it. Every
mapping carries `origin: stated` and cites the line it was read from:

    mappings:
      "#/paths/~1loans/post":
        target: lending.Server.LendBook
        origin: stated
        cites:
          - { source: desk-code, clause: "lending/routes.go:17", says: "The route." }

An element of the design that no mapping names is not built, and has the
question of section 3.2 saying so.

### 3.4 Questions

A question says what is unknown, who decides it, and what waits on it:

    questions:
      Q-1:
        question: Is the loan period 21 days, as the manual says, or 14 days, as the service does?
        kind: decision
        priority: must
        blocks: ["#/requirements/LEND-4/statement"]
        decidedBy: desk-manager
        options: ["21 days: the service changes", "14 days: the manual changes"]
        cites: [ ...both sources... ]

- `must`: what it blocks is not written, and nothing that reads it can be
  made. Use it for a disagreement and for anything a reader would
  otherwise have to guess.
- `should`: what it blocks is written as inferred or as not built, and the
  owner confirms it before it is built on.
- `could`: nothing waits on it.

A question sits in the stage folder of what it blocks. `decidedBy` names a
stakeholder by role, never a person. `docs/conventions.md`, Open
questions, has every rule.

### 3.5 Check after every step

    specarch validate spec

must report no errors. Warnings about missing tests are expected until the
tests stage is written (`specarch derive spec` drafts them); every other
warning is work. Then:

    specarch gaps spec

prints the open questions by stage, the elements by origin, the coverage
of each source and which outputs can be made. It exits 1 while a `must` or
`should` question is open, which is the normal state of a partial
specification. Then write the documents the owner reads:

    specarch document questions spec
    specarch document requirements spec
    specarch document techspec spec
    specarch document traceability spec

Each target is configured in the implementation file, under `targets`,
with the folder it writes to. `specarch document <target> --check spec`
exits 1 when a committed document is not what the specification makes
now; run it before each commit.

## 4. What the owner reads

- `questions.md`, the same text as `specarch gaps`: what to decide, with
  the options, and the coverage of each source. A document section or a
  code file that produced nothing is where to look first.
- `requirements.md` and `techspec.md`: the refined documents, each element
  with its Origin line and the open questions that concern it in place.
  A document that a question concerns starts with a Draft line.
- `traceability.md`: what each requirement is built by.

The owner answers in words. The agent records each answer as a decision
(`answers` names the question, `decidedBy` the role, the answer cited as a
source of kind `interview`), changes the elements to `origin: decided`
with `decidedIn`, removes the question and validates again. When no `must`
or `should` question is left, `specarch approve` records the owner's
approval, and code generation may start.

## 5. The agent's instructions

Paste this into the agent instructions of the repository being specified,
with the pinned version and checksum filled in:

    ## Specification

    The specification of this system is in spec/, written from the
    documents in <docs folder> and the code in <code folder> by the
    procedure in SpecArch's docs/from-sources.md.

    - Tool: specarch, pinned to <module version> (<h1 checksum>). Install
      with `go install github.com/SpecArch/specarch/cmd/specarch@<commit or tag>`;
      at the start of each session check `go version -m "$(go env GOPATH)/bin/specarch"`
      shows that version and checksum, and stop if it does not.
    - Every element carries origin and cites the document section or the
      code file and line it came from. Nothing is invented: what the
      sources do not say is a question, and a disagreement between a
      document and the code is a must question citing both.
    - Code only: origin inferred, why "Undocumented, from code. ...", and
      a should question. Documents only: a should question in
      spec/implementation/questions.yaml, "Not built yet. ...".
    - After every edit run `specarch validate spec` (no errors), then
      `specarch gaps spec`, and regenerate the documents with
      `specarch document <target> spec` for questions, requirements,
      techspec and traceability. End each work item with the gaps output.

## 6. Not built yet

- `specarch extract`: the agent reads the sources by hand.
- `specarch decide`: the agent writes the decision record by hand.
- `specarch gaps --json`: the dispatcher reads the text.
- A validator rule for a citation outside its source's clauses: `gaps`
  lists them, and the rule follows once a real project shows whether
  outlines stay complete.
- The document target `comparison`, which would set the old document
  beside the refined one.
