# From a result back to the specification: acceptance and change requests

A specification is never complete (`docs/principles.md`). People leave
things out, some of what they write is wrong in a way nobody sees until
the system is in front of them, and the people who will use the system
come to the user acceptance test, not to the requirements meetings. What
they bring is a change request about something they saw. This document
designs the loop that takes such a remark back into the specification and
out again in the next build. The decision is ADR-083, which is proposed
until the owner settles the points in its last section; nothing here is
built yet except the principle and the marked draft, and the steps are
listed at the end.

In short:

- Show a result early: the documents are written for an incomplete or
  invalid specification, and a marked draft of the code, the UI included,
  is generated and run before approval, labelled as a draft: the owner's
  option B for ADR-066, `generate --unapproved` (ADR-086).
- Trace the result to the specification: every generated screen, field,
  action, endpoint, message and test carries the JSON pointer of the
  element it came from, and the problem id when it is marked. A build for
  acceptance shows the pointer to the tester; production does not.
- A remark is a change request record, with who asked, when, against
  which build, where they saw it, what they saw and what they expected,
  and why the result was not what was wanted: the specification left it
  out, said it wrongly, or was right and the need changed. A remark that
  is the system not doing what the specification says is a defect.
- The remark enters the specification as a decision (a decision record
  that names the change request, and elements with origin decided), as a
  question, or not at all (a rejection). Before anything changes, what it
  reaches is listed: elements, tests, generated outputs, documents and the
  approval it voids.
- The next build carries the change, its regenerated output and tests,
  and the release notes list the change requests it answers, with why
  each was asked for.

Everything here reuses what exists: the change and defect records and
their statuses (`docs/maintenance.md`), origin, questions, decisions and
approval (`docs/refinement.md`), `specarch diff`, the problems file and its
marks (`docs/diagnostics.md`), and the `changes` and `releases` documents.
No second mechanism is added beside any of them.

## The loop

    specification  ->  draft or approved build  ->  acceptance  ->  change request  ->  specification
    (incomplete)       generate, every element      testers name    record: who, when,    change, decision
                       traced, problems marked      the element     build, seen, expected  or question

One round is: generate, build, put the build in front of the people who
will use it, take their remarks as change requests, analyse each against
the specification, change the specification, and generate again. The
round is repeated until a build is accepted. Approval (ADR-019) is where a
round ends with code that may be released; it is not where showing
results begins.

The standards behind it: ISO/IEC/IEEE 12207:2017 puts validation, the
check that the system does what its users need in its intended
environment, beside verification, the check against the specification
(6.4.11 and 6.4.9). ISO/IEC/IEEE 29148:2018 says that resolving what a
set of requirements leaves open is iterative (5.2.6). ISO/IEC/IEEE 14764:2022 names the kinds of change the
change request already uses. A trace that only goes from the
specification to the code serves the author; the trace back from a screen
to the element serves the person who is looking at the screen, and that
is the person who finds what was left out.

## 1. Show a result early

- Documents. Every document is written for an incomplete or invalid
  specification, with the open questions and problems marked where they
  are (`docs/diagnostics.md`, step 6). They are the first result, and the
  one a reviewer reads.
- Code. The approval gate stays for the code that may be released
  (ADR-019). Beside it, the marked draft lets `generate --unapproved` write a draft while questions are
  open or the approval is missing, with every gap marked at the entry
  (ADR-066, ADR-086). A draft of the server and the UI is what a tester
  runs. A draft says it is one in every file, and a build made from it
  will say so on every screen and in every answer (section 2, step 5), so
  it is never taken for the approved output.
- Build identity. Every generated file outside the specification (code,
  SQL, OpenAPI, tests, UI) names the specification's `info.version` and
  its digest, the digest the approval record keeps (ADR-019), taken
  without the problem mark lines. A build therefore says exactly which
  specification it came from, approved or not, and a change request can
  name it. The fragments that extract, merge and derive write are part of
  the specification, so they name no digest: it would be a digest of
  themselves. A checked-in generated file changes with every edit of the
  specification, so `generate --check` asks for it to be generated again
  after each one.

## 2. Trace a result to the specification

The id of an element is its JSON pointer into the merged specification,
the one the records, the problems and `diff` already use:
`#/pages/loans`, `#/paths/~1loans/post`, `#/entities/Loan/properties/dueDate`,
`#/tests/borrow-limit`. No second id scheme is added. A marked element
also carries the id of each problem at it (`docs/diagnostics.md`, 2.3), the
same id its mark shows.

Each generated form carries the pointer where that form keeps data a
reader does not see by default:

| Output | Trace |
|---|---|
| UI (web) | `data-specarch-element="<pointer>"` on the element drawn for a page, section, field, column or action, and `data-specarch-problems="<id> <id>"` when it is marked |
| UI (native) | the accessibility identifier, which UI tests already read, set to the pointer |
| Server code | a comment `// specarch: <pointer>` above each handler and type; each log line of a request names the operation's pointer |
| API answer | in an environment that shows the trace, the header `Specarch-Element: <pointer>` on every answer, and the problem type's pointer on a refusal |
| OpenAPI | `x-specarch-element` on each operation and schema whose name alone does not give the pointer |
| Messages | the header `specarch-element` on each message, in an environment that shows the trace |
| SQL | `-- specarch: <pointer>` above each table and column |
| Tests | the test's pointer in its name or a comment, and in every failure message |
| Documents | the element's heading |

The header name carries no `X-` prefix (RFC 6648, section 3).

A pointer names the inside of the system, so it is shown only where a
tester needs it. An environment of the deployment stage says
`trace: true` when its builds show the trace: the UI draws a small
overlay that shows the pointer of what is under the pointer or the focus,
the build's version and digest, and offers to copy a remark as the start
of a change request record (section 3). The API sends the header, and the
web UI carries its data attributes. Without `trace: true` nothing reaches
a user: the web UI is built without the attributes, since its HTML goes
to every browser, the API sends no header, and only comments in the
server code and the SQL keep the pointer. An environment that
promotes to none (the last one, production) with `trace: true` is a
warning at that key, since it shows the inside of the system to its
users.

## 3. A remark is a change request

A remark from acceptance is a change request record
(`records/changes/CR-12.yaml`), or a defect record when the system does
not do what the specification says. Both records keep their statuses and
rules (`docs/maintenance.md`). They gain what a remark on a build needs:

| Field | On | Holds |
|---|---|---|
| `phase` | change, defect | `development`, `acceptance` or `production`: acceptance is a remark made at the user acceptance test, on a build that is not released |
| `against` | change, defect | the build the remark is about: `version` (the specification's `info.version`, a pre-release one included), `digest` (the specification's digest the build names), `environment` (a key of the deployment stage) and `build` (the commit or build id, as in a commissioning record). Required in the acceptance phase. A defect keeps its own `environment` and has no second one under `against` |
| `seenAt` | change, defect | the pointers the tester copied from the trace: where it showed, before anyone has analysed what changes |
| `observed`, `expected` | change, defect | what the tester saw, and what they expected, in their words. Required in the acceptance phase |
| `cause` | change | why the result was not what was wanted: `omitted` (the specification did not say it), `wrong` (the specification said it, and it was not what was meant) or `changed` (the specification said what was meant, and the need changed). Required once the change is analysed |
| `questions` | change | the questions the analysis wrote, by ID. The IDs are text, as in a decision's `answers`, since an answered question leaves the specification; only a change still open (proposed or analysed) has them checked to be questions of the specification |

`raisedBy` on a change and `reportedBy` on a defect stay a role, a stakeholder key of the specification, never a
person's name. `observed` and `expected` are the tester's words and hold
no customer data: a record is read by more people than a ticket system.

### Spec wrong, spec silent, need changed, system wrong

Four answers to "why is this not what I wanted" are all normal, and the
records keep them apart:

| What happened | Record | Measured against |
|---|---|---|
| The system does not do what the specification says | defect | the specification |
| The specification did not say it | change, `cause: omitted` | what the users need |
| The specification said it, and it was not what was meant | change, `cause: wrong` | what the users need |
| The specification was right, and the need changed | change, `cause: changed` | what the users need now |

A defect triaged `not-a-defect` names the change it became, and that
change's `cause` says which of the other three it was. The `changes`
document counts each release's changes by cause; a release with many
omitted and wrong says the requirements work left gaps, and a release
with many changed says only that the need moved.

### Analysis: a decision, a question or a rejection

The analyst reads the remark against the specification and does one of
three things:

1. **A change the deciding role can settle now.** The role that owns the
   requirement it affects (`docs/maintenance.md`) approves it with a
   decision record in the specification, which names the change
   request under `answers`, as a decision names a question it answers,
   with `decidedBy` and `cites`. The elements it changes carry
   `origin: decided` and `decidedIn`. `affects` lists them, and `impact`
   is set by the version rule.
2. **A change that needs someone else's decision or material.** The
   analysis writes an open question in the specification, priority by
   what waits, and the change names it under `questions`. When the
   question is answered, the decision that answers it names both.
3. **No change.** The decision on the record is rejected, with its why.

A decision names the change request it settles, once, and the
specification gains nothing else from the record: no element carries the
list of the changes that touched it, which `docs/maintenance.md` forbids
as a change log. A decision is the specification's own account of why
an element is as it is; the request it settles is part of that account,
as the question is. Origin keeps its three values: the element is known
because a stakeholder decided it, and the record says that the decision
started at acceptance.

## 4. What it reaches, before it changes

When a change is analysed, the `changes` document shows, under each open
change, what its `affects` and `seenAt` reach in the specification as it
is now:

- the elements under each pointer;
- the tests that verify a requirement it names or whose subject is under
  one of its pointers;
- the pages, operations, commands and channels that read an entity or a
  setting under one of its pointers;
- the code targets whose `reads` take a section it touches, and the
  documents that show an element it touches;
- the approval it voids: any change to a file voids the current approval
  (ADR-019), and the line says which version and which documents the
  approver read.

Approval stays whole: the digest is over every file, because approval is
of exactly what was read. What makes a second approval cheap is that the
approver reads only what changed: `specarch diff` between the approved
specification and the working tree lists every changed element with the
change request or defect that covers it, and its `covered` check refuses
a changed element no record names. A planned release record for the
version being tested holds the change requests of the round, so the check
runs before the release.

`diff` cannot do that today. The approval record keeps a digest and no
commit, so nothing says which tree was approved; and within one
pre-release (`1.5.0-dev` approved, `1.5.0-dev` edited) the `version`
check fails, since the new version is not after the old one. So the
approval record gains the `commit` it was made at, as a release record
has one, and `diff` skips the `version` check when both sides have the
same `info.version`, the case of a round of acceptance inside one
pre-release, while `covered` still runs (step 8).

## 5. Regenerate and show again

After the specification changes, the round closes the usual way: validate,
`document`, `derive` for the new cases, `generate` (draft or approved),
and a new build whose version and digest differ. The change moves to
implemented when the specification and the code carry it. The tester
checks it in the next build; a remark that the change did not fix is a
new change request, not a reopened one, because the first one's decision
stands as made, and the new one's `reason` names the first.

The `releases` document lists, for each version, the change requests it
answers, grouped as added, changed, removed and fixed, each with its
phase and cause, so the notes of a release that closed a round of
acceptance say what the users asked for and why.

## 6. Gaps in the documents that exist

Checked against the principle, the existing design has these gaps; each is
a step below.

- `docs/refinement.md`: the path ends at code, and nothing comes back
  from a result. Its only reviewer reads documents, which the people who
  will use the system do not. "A change to an approved specification is a
  change request" names no acceptance phase and no build.
- `docs/from-sources.md`: the procedure ends at approval; it has no step
  for remarks on the first build, and its agent instructions do not say
  how to record one.
- ADR-017 and ADR-019: a question is the only way to mark something
  unknown, and nothing marks what is known to be unwanted (an open change
  request) at the element. The approval is whole, which is right, but
  nothing yet shows the approver the difference from the approved
  version without checking out the old one by hand.
- `docs/diagnostics.md`: a mark names a problem, but an element without a
  problem carries no id, so a tester cannot name it. An open change
  request is not shown at its element in the documents.
- `docs/maintenance.md`: the change and defect records have no acceptance
  phase, no build, no observed and expected, no cause, and no link to the
  question an analysis wrote; the `changes` document shows no reach.
- `docs/generators.md`: no generator writes a trace, and no generated file
  names the specification's digest.

## 7. Steps

1. **The principle and this design** (built): `docs/principles.md`, the
   check in `CLAUDE.md`, this document, ADR-083 (proposed) and SA-57.
2. **The records for acceptance**: `phase: acceptance`, `against`,
   `seenAt`, `observed`, `expected`, `cause` and `questions` in the record
   schema and both validator builds, with their references checked under
   `record_ref` (a change's `questions` only while it is open), a rule
   that an acceptance record says what was observed, expected and against
   which build, and one that an analysed change has a `cause`; an example
   change from acceptance in the library lending example, with a test
   that verifies SA-57; conformance cases.
3. **A decision answers a change request**: `answers` may name a change
   request, in both builds; an accepted decision that answers a change
   whose decision is not approved is an error. If the owner chooses the
   alternative of section 8.1, the change's `decision` names the decision
   record instead, checked to exist, and `docs/maintenance.md` says so;
   otherwise its sentence that the specification never points back at
   records names this one exception.
4. **The trace in generated output** (Go only): the pointer in every form
   of section 2's table and the version and digest in every generated
   header, after the marked draft of ADR-066 (diagnostics step 7).
5. **The trace shown in an acceptance build**: `trace` on an environment,
   the overlay in the web UI generators, the answer header in the
   go-dxlib server, and the warning for a last environment that shows it.
6. **Reach and marks of open changes**: the reach of section 4 in the
   `changes` document, and an open change request shown at its element in
   every document, beside the open questions.
7. **Release notes and the procedure**: phase and cause in the `releases`
   document and the count by cause; the return arrow in
   `docs/refinement.md`'s path; an acceptance round in
   `docs/from-sources.md` and its agent instructions.
8. **diff against the approval**: the approval record's `commit`, and
   `diff` skipping the `version` check when both sides have the same
   `info.version`, so the approver of a second round reads only what
   changed.

## 8. For the owner to decide

ADR-083 stays proposed until these are settled:

1. A decision names the change request it settles under `answers`, a link
   from the specification to a record, which `docs/maintenance.md`
   otherwise does not make (section 3). The alternative keeps every link
   in the record: the change's `decision` names the decision record, and
   the specification stays silent about records.
2. The trace is shown only in an environment with `trace: true`, and the
   last environment showing it is a warning. The alternative shows it in
   every build except that of the last environment, the one with no
   `promotesTo`, without a key; that takes the trace away from a project
   whose last environment is not its production, or that tests in it.
3. The causes `omitted`, `wrong` and `changed`, required once a change is
   analysed.
4. Approval stays whole, and the cost of approving again is kept down by
   `diff`, not by approving part of a specification.
