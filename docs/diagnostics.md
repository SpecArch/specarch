# Problems: one list, and a mark at every entry

SpecArch reports every problem of a specification the way a compiler
reports the problems of a program: one list, sorted by file and line, that
an editor can jump through, and a mark at the entry in every file SpecArch
writes. This document designs that list, the problem record behind it, its
machine form, and the marks, and splits the work into steps. It applies the
first principle of `docs/principles.md`; the decision is ADR-065.

In short:

- A problem is an error, a warning or an open question. Each has a stable
  id, a severity, a rule, a one-line message that says how to fix it, the
  file, line and column of the entry, its JSON pointer, and notes that point
  at the sources it came from and at the entries it blocks.
- `specarch document problems` writes two files from the same problems:
  `problems.txt`, the list a person opens, one compiler line per problem;
  and `problems.sarif`, the machine form, in SARIF 2.1.0. It writes them for
  an invalid specification too: that is when they are needed most.
- Every file SpecArch writes marks the entry a problem touches with the
  problem's line, in that file's own comment form, and `document problems`
  marks the fragments an author writes as well. The next run writes the
  marks again from the problems it finds, so a fixed problem loses its mark.

## 1. What exists, and how it is unified

Most of the pieces exist. They are joined, not replaced.

| What exists | What it becomes |
|---|---|
| validate's line | The problem line, `file:line:column: severity: /pointer: rule: message [id]` (section 3), printed by validate in both builds and by generate for a plug-in's diagnostics. |
| validate's errors and warnings | Problems of severity error and warning, with the same rule and message. |
| A diagnostic's message, "what is wrong and how to fix it" | Stays the one place that says how to fix it (section 2.4). |
| Open questions (`questions:`), listed by `gaps` and the questions document | Problems of severity question. Their id is the question's id (`Q-4`). The questions document stays the document for the people who decide. |
| The diagnostics an open question covers (a key it leaves out) | Notes of that question, at the entry that leaves the key out, not problems of their own: the question already says why the key is missing. |
| The Open question paragraph at an element in every document, and the `(open in Q-1)` text where a key is shown | The mark of a question problem in a Markdown document. Errors and warnings get the same paragraph (step 6). |
| The Draft notice at the top of a document | The file-level summary of the marks below it. |
| `gaps`'s place of the element that leaves a key out | The question's note at that place. |
| extract's "could not hold" lines, which go only to standard output | Could questions in the tree it writes, citing the source line, so they are problems like any other (step 5). |
| The fragment schemas' hints in the editor | Stay as they are: they help while typing, before any run. |
| The machine form the agent queue needs | `problems.sarif`, which holds every question with its fields (section 4). |

## 2. The problem record

A problem has these parts. The design entity is `Problem`.

### 2.1 Severity

`error`, `warning` or `question`. An error makes the specification invalid,
a warning leaves it valid, and a question is an open question of any
priority (must, should or could). The priority is in the message, as the
questions document writes it: `(must, decision)`.

### 2.2 Place: file, line, column, pointer

- The file is the fragment the author edits, named relative to the folder
  the problems file is in, so the line works from there and does not depend
  on the folder a run starts in.
- Lines and columns start at 1, as GCC, Clang, the Go tools and SARIF count
  them. LSP counts from 0; a reader for an LSP client subtracts one. A
  column counts Unicode characters, as the YAML parsers of both builds do.
- In an expression, the column is the one inside the expression, which
  the expression parser knows, counted from the expression's first
  character (after the quote of a quoted scalar, from the block's
  indentation in a block scalar). Otherwise it is that of the node the
  pointer names when the problem is on that node's line (its value, or else
  its key), and otherwise that of the first character on the line that is
  not a space.
- A problem about the whole file (`/`) is at line 1, column 1, where the
  file starts.
- A question is at its own entry in the questions file, where it is
  answered.

### 2.3 Id

A problem's id names what is wrong, not where it is printed, so that it
stays the same while lines move above it:

- a question: its own id, `Q-4`;
- an error or a warning: `<rule>@<file>#<pointer>`, with the file relative
  to the specification's folder, such as
  `relation_target@design/design.yaml#/entities/Loan/relations/member/target`.
  When one rule reports more than one problem at one pointer, each gets a
  tag after the rule, eight hex digits of the FNV-1a hash of its message:
  `test_case_missing.585bd96d@design/design.yaml#/paths/~1members/post`. A number in the order the problems
  come would shift the others' ids when one is fixed; the tag does not.
  Problems that share the message as well are numbered `-2`, `-3` and on.

The id is long, but every part of it is a word the reader already knows,
and it leads to the entry without the problems file. A hash of the whole
record would be short and say nothing (principles, rule 1); the tag is a
hash only where the readable part cannot tell two problems apart. The id is computed from the record
alone, so both validator builds give the same id. It is what SARIF calls a
partial fingerprint, which exists for the same reason: to know a result
again in the next run although its line moved.

### 2.4 Message, and how to fix it

The message is the diagnostic's message: one plain sentence that says what
is wrong and how to fix it. It is not split into a message and a fix. Every
rule already writes it that way, in more than three hundred places and in
two builds, and a separate fix field would be a second place to say one
thing. A question's message is its question, who decides it, and how it is
answered: "Answer with a decision record that names Q-4 under answers."

### 2.5 Notes

A note is a place that helps with the problem, printed after it:

- `source`: each citation of the element the problem is at, or of the
  question. When the source's `url` is a folder beside the
  specification and the clause is `path:line`, the note is at that file and
  line; otherwise it is at the citation in the specification. Its text
  names the source, the clause and what it says.
- `blocks`: for a question, each entry it blocks, at the entry, or at the
  element that leaves the blocked key out, with the keys missing there. A
  stage or a section has no entry; it is named in the message instead.

## 3. The problems file

`specarch document problems` writes `problems.txt` into the folder the
`problems` target owns, as every document does. It is written for an
invalid specification too, and the command then exits 1 after writing it.
The folder comes from `--out` or from the implementation files; an
implementation file with an entry that does not parse still names it
(section 6, step 8). Every other document
is written for an invalid specification too, with its errors marked
(section 5.1), and `gaps` lists the errors with the questions.

The file starts with two lines that are not problem lines: what it is, and
the counts. Then one line per problem, sorted by file, line, column,
pointer, rule and id, each followed by its notes:

```
Problems of Lending Desk, version 0.1.0: 0 errors, 22 warnings, 7 open questions (4 must, 3 should, 0 could).
Generated by specarch document problems from ../spec/specarch.yaml. Do not edit this file: fix the specification and generate again.

../spec/implementation/questions.yaml:3:3: question: /questions/Q-4: open_question: (should, decision) Renewing a loan is in the manual but not in the service. Build it as the manual describes, or take it out of the manual? It blocks implementation. Decided by desk-manager. Answer with a decision record that names Q-4 under answers. [Q-4]
../spec/implementation/questions.yaml:12:9: note: source desk-manual, clause 4.1: A member may renew a loan once.
../sources/code/lending/routes.go:14:1: note: source desk-code, clause lending/routes.go:14: Routes serves no renew route.
```

The manual's clause 4.1 is not a line of a file, so its note is at the
citation; the code's clause is, so its note is at that line of the code.

The line is the GNU form `file:line:column: severity: text`, which GCC,
Clang, the Go tools, Emacs, Vim and most CI log readers parse. After the
severity it keeps validate's order: pointer, rule, message; the id closes
the line in brackets. A specification with no problem gets the two lines
only, the first saying "no problems".

### 3.1 What the editor reads

The editor (`myowncodeeditor`, `Sources/Core/SpecArch.swift`) parses the
problem line, from validate and from the problems file. It:

1. accepts an optional column: when the place ends in `:<line>:<column>`,
   it takes both, and jumps to the column;
2. accepts the severities `question` and `note`, showing a note under the
   problem before it;
3. takes the id in brackets at the end of the message of a line with a
   column, and shows it;
4. offers `problems.txt` of the open specification as its Problems list
   when the file is there, beside what validate prints on save.

A line without a column still reads, with no column and no id.

## 4. The machine form: SARIF 2.1.0

`problems.sarif` holds the same problems as one SARIF 2.1.0 log. SARIF is
the OASIS standard for the results of static analysis. GCC and Clang emit
it, and GitHub code scanning, Azure DevOps and the SARIF viewers read it.
SpecArch uses it rather than a JSON form of its own, because a reader that
knows SARIF already knows this one, and the standard already answers the
questions an own format would have to: how to say which rule, how sure the
tool is, where in a file, how to know a result again in the next run.

Why SARIF is the way it is, and how SpecArch keeps its rules:

- It was made so that results from many tools can be merged, compared
  between runs and managed in one place. So a result says its rule
  (`ruleId`), and a `partialFingerprints` entry lets the next run recognise
  it although its line moved. SpecArch writes the id of section 2.3 there,
  as `specarchId/v1`.
- It separates whether a result is a failure (`kind`) from how bad a
  failure is (`level`), because some tools can prove a problem and some can
  only say they could not decide. An error is `kind: fail, level: error`, a
  warning `kind: fail, level: warning`. An open question is
  `kind: open, level: none`: the standard's `open` means that the tool
  lacked the information to decide, and that someone must supply it, which
  is what a question is; and a result whose kind is not `fail` has level
  `none`.
- Columns were ambiguous between tools, so a run says how it counts them.
  The standard's default is UTF-16 code units; SpecArch counts Unicode
  characters, so it writes `columnKind: unicodeCodePoints` rather than
  leaving the default to mislead.
- A `fixes` entry in SARIF is an exact edit of the file, which a tool can
  apply. SpecArch's message says what to do, not an edit, so it writes no
  `fixes`; the message is the result's `message.text`.
- Locations are relative to the base id `PROBLEMSDIR`, the folder holding
  the log, which the reader supplies, as SARIF allows. An absolute path
  would differ between machines and break byte-identical runs.

The rest of the mapping: the pointer is a logical location
(`fullyQualifiedName`, kind `element`); notes are `relatedLocations`; a
question's priority, kind, decider, options and blocked entries are in the
result's `properties`. `tool.driver.rules` lists the rules the log uses.
That is everything the agent queue needs of a question, so the queue reads
this log and has no JSON form of SpecArch's own; the outputs table stays in
the questions document.

## 5. Marks at the entry

Every file SpecArch writes marks the entry a problem touches with the
problem's line, from the severity on, in that file's comment form, on the
line above the entry with the entry's indentation. A mark is written from
the problems of the run that writes the file, in the order of the problems
file, so two runs write the same bytes; a file SpecArch writes again loses
the marks of problems that are fixed.

| Output | Mark |
|---|---|
| Specification fragments (YAML) | `# specarch-problem: error: relation_target: ... [id]` above the entry |
| Markdown documents | the Open question paragraph for a question (as now), and a Problem paragraph for an error or a warning, at the element |
| BPMN 2.0 XML | `<!-- specarch-problem: ... -->` before the element; `--` in the text is written `- -`, which XML forbids in a comment |
| Go, Swift, Dart, TypeScript, JavaScript, tests, UI | `// specarch-problem: ...` above the declaration, or under the header of a file that is the entry (a page) |
| HTML and CSS of the UI | `<!-- specarch-problem: ... -->` and `/* specarch-problem: ... */` under the header, with `--` written `- -` and `*/` written `* /` |
| SQL | `-- specarch-problem: ...` above the statement or column |
| OpenAPI | `x-specarch-marks`, a list of `{id, severity, rule, message}` on the object; JSON keeps no comment, OpenAPI allows `x-` keys on its objects, and `x-specarch-problems` already names the problem catalogue |

In a YAML fragment:

- The mark goes above the line where the entry starts in block style: the
  key of its pair, or the `-` of its item. When the entry sits inside a
  flow collection (`{ ... }`) or a block scalar, the mark goes above the
  nearest block-style entry that holds it, so a mark never lands inside a
  scalar's text.
- A line that starts with `# specarch-problem:` after its indentation is
  SpecArch's own. A run removes every such line from a file it marks and
  writes the current marks; it never touches any other line, so the file's
  own formatting and comments stay. The edit is on the text, not by writing
  the YAML out again.
- A problem about the file as a whole, or one whose pointer leads to no
  entry of the file (a file that does not parse), is marked above the
  file's first line that is not blank or a comment, so a schema line at
  the top stays first.
- Only the problem's own entry is marked. A note, at a source or at an
  entry a question blocks, is not: a question is marked at its entry in
  the questions file, where it is answered.
- A mark moves every line below it down. So the problems file gives the
  lines of the marked files: the lines in it are the lines on disk after
  the run. A mark is a comment, which no validator reads, so the marked
  files have the same problems.
- `document problems` marks every YAML file in the specification's folder
  that the specification read: the root file, its fragments, its
  implementation files, and a fragment that does not parse. A file the
  specification does not read, such as an input of a conformance case,
  is never touched. With `--check` it writes nothing and fails when a
  file's marks are not the ones it would write, as for any output.
- `extract` and `merge` mark the tree they write, and `derive` the draft
  tests it writes; only `document problems` edits a file an author wrote.
- A mark is not part of what is approved: the approval digest leaves the
  mark lines out, so writing or removing marks never voids an approval.

### 5.1 In a Markdown document

A document marks an error or a warning with a Problem paragraph, the
problem's line from the severity on without the pointer, which the place
of the paragraph and the id already give:

```
**Problem on orders:** error: relation_target: Ordr is not an entity of the specification; did you mean Order? [relation_target@specarch.yaml#/entities/Customer/relations/orders/target]
```

- The paragraph stands at the deepest element the document shows that
  holds the problem's pointer: where its Origin line and Open question
  paragraphs stand, after a table for a row, under the heading for an
  element with one. A pointer is deeper than the element a document
  shows (`/entities/Customer/relations/orders/target` under
  `/entities/Customer`), so the document is made once to learn which
  elements it shows, and again with each paragraph at its element.
- Under the summary, with the Draft notice, a Problems notice says that
  the specification is invalid while it has errors, and how many errors
  and warnings concern the document: those in the sections it reads, and
  any at an element it shows. After it comes the Problem paragraph of each
  one the document shows no element for (an `info` key, a fragment as a
  whole, or an implementation file in the technical specification), so
  none is left out. A valid specification gets the notice only where a
  warning concerns the document.
- The open questions document lists the errors after its summary as
  problem lines; in its table a document is a draft while an error is in
  what it reads, and code generation waits on every error. This is what
  `gaps` prints. Its questions are those of what could be read.
- The command prints the errors, writes every document, and exits 1;
  with `--check` it compares and exits 1 as well.

`generate` writes the marks of every code target from the problems the
plug-in request carries (`docs/generators.md`, Plug-ins), by one rule
(`internal/mark`, ADR-086):

- A warning is marked at the deepest entry the file shows that holds its
  pointer, an open question at each entry it blocks. A question that
  blocks a whole section the file shows is marked under the header.
- A problem at an entry the file leaves out, such as one another
  stakeholder owns or a page the target does not write, is not marked in
  it; the problems file lists it.
- The target's own problems, which specarch gives their ids and hands the
  plug-in on a second run, are marked the same way, and one that no entry
  holds, such as one about the target's settings, is marked where the
  output as a whole is: under the header of the file that stands for it.
- Where each target puts them: SQL above the `CREATE TABLE` of an entity
  and above the first column of a field, and in a later migration above
  the first statement that changes the entity's table; OpenAPI on the
  schema, the property and the operation; the go-dxlib file above the
  table's entry, the operation's request type and the job's task; a test
  file above the test or worked example; the web UI under the header of
  each file of a page, with the marks of the screens as a whole in
  `events.js` of the plain JavaScript UI and `application.ts` of the
  TypeScript one; BPMN before the element, in the
  BPMN file and in its SVG, whose marks for an operation or a role stand
  before the start event or the task that names them.
- A migration is written once, so it keeps the marks of the run that wrote
  it; the next migration carries the marks of what it changes.
- A draft says so under the header of every file, with the notice the
  request carries: the questions that block what the target reads and the
  approval's state. OpenAPI also carries it as `x-specarch-draft` on
  `info`, and the SVG of a workflow shows it on the picture.

A source SpecArch reads, such as a BPMN file given
to `extract workflows`, is never marked: a problem cites it in a note
instead.

## 6. Steps

1. **The problems file.** The `Problem` record, `specarch document
   problems` writing `problems.txt` and `problems.sarif` for a valid or an
   invalid specification, the ids, the derived column, the source and
   blocks notes; conformance cases; both examples carry a problems file.
   validate's output is unchanged.
2. **The editor reads it** (an item for `myowncodeeditor`): section 3.1.
3. **validate prints the new line**, after step 2: column and id in both
   builds, with the column inside an expression from its parser; every
   validate case is recorded again.
4. **Marks in specification fragments**: in every tree extract, merge and
   derive write, and in hand-written fragments when `document problems`
   runs, on every run (ADR-066).
5. **extract's "could not hold" lines become could questions** in the tree,
   each citing the source line.
6. **Documents are written for an invalid specification**, marked, with
   status 1: a Problem paragraph at the element for an error or a warning,
   and `gaps` lists the errors with the questions instead of refusing.
7. **Marks in generated code, SQL, OpenAPI, tests and UI**, and a marked
   draft (ADR-066): while a must or should question blocks what a target
   reads, or the version has no approval, generate may write a draft with
   every gap marked at its entry, labelled a draft in every file and never
   taken for the approved output.
8. **A fragment that does not parse keeps what can be read** (ADR-080):
   the file is read entry by entry by its indentation; an entry that
   parses is kept, a broken one is held by its name with no value, or
   left out when it is a key at the top of the file, and each is one
   yaml_syntax error at its line and pointer with nothing else reported
   of it. One broken entry stops causing errors in every file that
   refers to it, and an implementation file with a broken entry still
   names its output folders.
