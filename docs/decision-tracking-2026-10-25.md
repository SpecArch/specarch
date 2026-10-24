# Why a decision was taken: the reason behind every edit of a specification

An analyst or an architect who opens an element months later wants to know
why it is here, why it changed, who decided, on what evidence, what was
rejected, and what question or change request caused it. This document
says what SpecArch records of that today, what a reader can and cannot
answer, the smallest design that closes the gaps without turning every
keystroke into a form, how the record stays honest when an AI agent writes
the specification, what the editor would need, and a plan in phases. It
changes no code, schema or generator. Every claim names its source in
parentheses, or says it is an assumption.

In short:

- SpecArch already has one place for the reason behind the specification:
  the decision record (`decisions`, an architecture decision record after
  Nygard), with `why` for the reasoning, `cites` for the evidence,
  `decidedBy` for the role, `date`, `answers` for the question it settled,
  `supersededBy` for what replaced it, and `decidedIn` on the element it
  settled. A change request has a required `reason` and a decision with a
  required `why`. Nothing else is needed as a construct.
- The gaps are in what is required and what is linked. `why` is optional
  on a decision; `decidedBy` is required only when the decision answers a
  question; an element may name a decision only with `origin: decided`, so
  an element written spec-first cannot point at the decision behind it;
  nothing holds the alternatives rejected; a delete, a rename, a status
  change, a withdrawn release and a deprecated decision leave no reason
  anywhere but git; and nothing lists the elements with no reason behind
  them.
- The smallest closing design is additive to meta-model 0.3: `alternatives`
  and `proposedBy` on a decision, `governs` from a decision to the elements
  it covers, a warning for an accepted decision without `why`, a reason
  summary in `specarch gaps`, and the diff against the approval that
  `docs/acceptance.md` already plans, printing each changed element's reason
  beside it. Making `why` required on an accepted decision is the one
  breaking change, for meta-model 0.4 and program version 0.8.0, and an
  owner's decision.
- An agent never writes a reason as the owner's: its own conclusion is
  `origin: inferred` with `why`, or a decision with `status: proposed` and
  `proposedBy: agent`, with a question for a human to confirm. "Reason not
  recorded" is the absence of `why`, listed by `gaps`, never a sentence.

## 1. What SpecArch holds today

### 1.1 Where a reason can live

| Place | Holds | Required | Source |
|---|---|---|---|
| `why` on an element | the rationale in plain words; a document renders it as an Insight | optional, on 50 of the 86 object shapes of the design schema (every named element, a question, a decision; not a field, a parameter or a step, which trace through their holder) | `schema/specarch-design-0.3.schema.json` `$defs/why`; `docs/conventions.md`, Insight and Notes |
| `cites` on an element | the sources it rests on: `source`, `clause`, `says` | optional; required with `origin: stated` (`origin_citation`) | same schema; `docs/refinement.md`, Origin |
| `origin`, `decidedIn` | how the element is known: `stated`, `inferred` (needs `why`, `origin_reason`), `decided` (needs `decidedIn`, an accepted decision, `origin_decision`) | optional; `origin_missing` warns for every element without one when `info.tracksOrigin` is true | `docs/refinement.md`, Origin; `internal/validate/questions.go`, `checkOrigin` |
| `decisions`, one record per ID | `title`, `status` (proposed, accepted, deprecated, superseded), `date`, `context`, `decision`, `consequences`; `why`, `cites`, `satisfies`, `answers`, `opens`, `decidedBy`, `supersededBy` | the first six; `supersededBy` with status superseded; `decidedBy` with `answers` | design schema `$defs/decision`, `required` and `dependentRequired` |
| `questions` | `question`, `kind`, `priority`, `blocks`, `decidedBy`; `options`, `why`, `cites` | the first five | `schema/specarch-fragment-questions-0.3.schema.json` |
| a change record | `reason` (why it is asked for); `decision` with `by`, `date`, `outcome`, `why`; `affects` with `adds`, `changes`, `removes` | `reason` always; `decision.why` with the decision; `affects` once analysed | `schema/specarch-record-0.1.schema.json` `$defs/change`; `docs/maintenance.md`, Change requests |
| a defect record | `violates`, `test`, `duplicateOf`, `change` | no reason field | record schema `$defs/defect` |
| a release record | `version`, `status`, `date`, `includes`, `commit` | no reason field; the reasons are in the included changes | record schema `$defs/release` |
| an incident record | `summary`, `impact`, `noChange` (why no change follows) | `noChange` is the only reason-like field, optional | record schema `$defs/incident` |
| a commissioning record | `results` with a `note` per check, `signoff` by role and date | `note` optional | record schema `$defs/commissioning` |
| an approval record | `approvedBy`, `date`, `documents`, `digest` | no textual reason, by design: the reason is that the role read the documents and no must or should question was open | `docs/refinement.md`, Approval; ADR-019 |
| the problems file | every error, warning and open question with a stable id and its entry | always | `docs/principles.md`, rule 2; `docs/diagnostics.md` |
| git | the commit that changed the lines (`git log -L <first>,<last>:<file>`, `git blame`), the message, a tag per release (`v0.7.5`) | always, outside the specification | `docs/maintenance.md`: "that link lives in git, where history belongs; the validator does not read it" |
| `history/` of this repository | one prose file per day: what changed and why, the owner's decisions, commit hashes | for this repository's own work, by its `CLAUDE.md`; not a SpecArch construct | `CLAUDE.md`, History |

Two rules shape all of it. A specification describes the system as it is
now, and never points back at the records: no element carries the list of
the changes that touched it, since that would be a change log under
another name (`docs/maintenance.md`, The test that decides where a thing
lives; `docs/principles.md`, Documents). And a record or a decision names
people only by role, a stakeholder key, never a person
(`docs/maintenance.md`, Records; `docs/refinement.md`, The decision that
closes a question).

### 1.2 What the editor offers around them

The Spec panel of the owner's editor (`apps/myowncodeeditor/docs/specarch.md`)
shows every element as a card with its `why`, `cites`, origin and decision
as fields, and offers:

- **Answer…** on a question, which runs `specarch decide` and opens the
  decision's card (Spec panel, Answer).
- **Remark…** on any card or row, written as a question that blocks the
  element or as a change request with the remark as its `reason` (Review
  mode).
- **Review round** and **Compare since approval…**, the questions,
  decisions and change requests newer than the approval's last commit,
  and `specarch diff` from that commit (Review mode).
- **Compare release…** and **Suggest impact…** over `specarch diff` and
  the records (Release and environments; Reach and Suggest impact).
- **Rename…**, **Move to Its Own File**, **Merge into** and **Delete…**,
  which rewrite references and list the references a delete would break,
  and record no reason (`kit/specarch-panel-design.md`, 4.4).
- The change request's Approve and Reject with who, when and why, and the
  defect's and incident's steps (Defects and incidents on the record
  cards).

Not built, listed in its own plan: **Decide this…** on an element's card,
one sheet that writes the decision and sets `decidedIn` (item 10 of
`docs/specarch-sdlc-recommendations.md`), and **History** on a card, the
commits that touched the element's lines by `git log -L` with the release
and change request per commit (item 15 there; no `SpecHistory.swift`
exists).

### 1.3 For each kind of edit: is the reason recorded

| Edit | Reason recorded | Where | Required | Source |
|---|---|---|---|---|
| add, spec-first | only if the author writes `why` or `cites` | the element | no | design schema |
| add, from a source | yes: `origin` with `cites` (stated) or `why` (inferred) | the element | with `tracksOrigin`, by `origin_missing` and the three origin rules | `docs/refinement.md`, Origin |
| add or change that answers a question | yes: the decision's `context` (the question), `decision`, `consequences`, `decidedBy`, `cites` (the interview); `why` optional | the decision; `decidedIn` on the element | the record's six fields and `decidedBy`; `why` no | `docs/refinement.md`, The decision that closes a question; `specarch decide` |
| change of an element inside development, no question | no | git only | no | `docs/maintenance.md`: an answer to an open question of an unapproved specification is not a change request |
| change of an approved or released element | yes when a change request exists: its `reason` and `decision.why`; `diff`'s `covered` check refuses a changed element no record names, at release time | the change record | `reason` yes; the record itself only at release (`covered`), not at the edit | `docs/maintenance.md`, The diff verb |
| delete | as a change: `affects.removes` on a change record; otherwise nothing | the change record, or git | no, except through `covered` at release | same |
| rename | no; `diff` sees a removal and an addition (major) | git | no | `docs/maintenance.md`, the impact table; the editor's Rename… |
| move to another file | no; it is layout, no element changes | git | no | `kit/specarch-panel-design.md`, 4.4 |
| requirement status (proposed, accepted, rejected, retired) | no field for the reason of the step; `why` on the requirement is about the requirement, not the step | the requirement, if the author writes `why` | no | design schema `$defs/requirement` |
| decision status: proposed to accepted | no | git | no | design schema |
| decision superseded | the successor, `supersededBy`; the old one carries no why of its own | the old decision | `supersededBy` yes | design schema `then.required` |
| decision deprecated | no (no pointer, no why) | git | no | design schema |
| a field `deprecated: true` | no | git | no | design schema, a property's `deprecated` |
| change request approved or rejected | yes: `decision.by`, `date`, `outcome`, `why` | the change record | yes (`change_decision`) | record schema; `docs/maintenance.md` |
| change request withdrawn | no | git | no | record schema |
| defect triage (confirmed, not-a-defect, duplicate) | partly: `change` names the request a not-a-defect became, `duplicateOf` the other defect; no why | the defect record | no | record schema |
| release released | no reason; what it includes carries the reasons | the release record | no | record schema |
| release withdrawn | no | git | no | record schema |
| approval | the role, the date, the documents read, the digest; no text | the approval record | yes, written by `specarch approve` | `docs/refinement.md`, Approval |
| commissioning sign-off | `note` per check; the signing role and date | the commissioning record | `note` no | record schema |

Edits that leave no reason at all today: a change inside development that
no question drove, a delete that no change request covers, a rename, a
move, every status step of a requirement and of a decision but superseded,
a deprecated decision, a deprecated field, a withdrawn change request, a
withdrawn release, and a defect's triage. Git holds when and what for each
of them, and in this repository the commit message holds the why, by the
owner's rule that a commit says what changed and why (queue rules; this
repository's commit style, `git log`).

### 1.4 What this repository's own specification shows

Counted on 2026-10-25 in `spec/design/decisions/`: 176 decisions, 174
accepted, one proposed (ADR-083), one superseded (ADR-007, by ADR-010).
Five carry no `why` (ADR-001, ADR-002, ADR-006, ADR-007, ADR-009). None
carries `decidedBy`, and no element of the specification carries
`decidedIn`: the specification is written spec-first by the owner, so the
decider is the owner by default and nothing says so. Every commit's author
is the owner, whether the owner or an agent wrote it (queue rules: the
owner is the author of every commit), so `git blame` cannot tell them
apart either.

## 2. What a reader can answer now about an element

| Question | Answered by | What is missing |
|---|---|---|
| Why is it here? | `why` and `cites` on the element when written; `origin` and `decidedIn`, which leads to the decision's `context`, `decision` and `why` | nothing is required of a spec-first element; a decision's `why` is optional; an element written spec-first cannot name the decision behind it, since `decidedIn` without `origin: decided` is the error `origin_decision` (`internal/validate/questions.go`), and `origin` means "built from sources" |
| Why did it change? | a change request whose `affects` names it, found by reading `records/changes/` (the `changes` document; the editor's Reach), and the decision that answers it; `git log -L` for the commit | no link from the element or the decision to the change (ADR-083, section 8.1 of `docs/acceptance.md`, undecided); inside development, before a release, nothing is required at all; `diff` compares releases, not the approval (ADR-083 step 8) |
| Who decided? | `decidedBy` on a decision that answers a question; `decision.by` on a change; `approvedBy` on the approval; always a role | a decision that answers no question names nobody; a person is never named, by design (`docs/maintenance.md`, Records), and a reader who wants the person goes to git, where every commit is the owner's |
| On what evidence? | `cites` on the element and on the decision, each naming a declared source with its clause and what it says; an answer given in words is a source of kind `interview` with its date | optional everywhere except with `origin: stated`; a spec-first element cites nothing unless the author did |
| What alternatives were rejected? | nothing structural; some decisions write them into `why` or `context` as prose (ADR-155's `why`: "chosen over re-encoding") | no field; Nygard's form has none (`docs/refinement.md`), MADR's "considered options" is the usual addition (assumption: MADR 4.0, the Markdown Any Decision Records template) |
| What question caused it? | `answers` on the decision, the question's ID as text, since the question left the specification; `opens` for the half left open | the question's text survives only as the decision's `context`, which `specarch decide` writes from it; a hand-written decision may leave it out |
| What change request caused it? | the change record's `affects`, read backwards | nothing on the decision (ADR-083, 8.1) |
| What is missing? | `origin_missing` when `tracksOrigin` is true; `gaps` counts elements by origin | nothing lists elements without a `why`, decisions without a `why` or a `decidedBy`, or retired and deprecated things without a reason |

## 3. The smallest design that closes the gaps

Three principles bound it. One way to say one thing (`docs/principles.md`,
Low IQ Tax rule 4): the decision record is the reason's one home, so no
second "decision record element" is added, and a change request keeps its
own `reason` because a request is not a decision. No placeholders (rule
6): "reason not recorded" is the absence of `why`, which a report lists; it
is never a string in `why`. And the unit of a reason is a decision or a
change request, not an edit: a person who makes ten edits for one decision
writes one record, and a keystroke never opens a form. Each part below
says whether it is additive to meta-model 0.3 (a valid 0.3 file stays
valid: a new optional key, a new warning, a new line of output) or breaking
(a valid 0.3 file becomes invalid, which moves the meta-model to 0.4 by the
rule of ADR-052 and ADR-094, and the program to 0.8.0, the minor step a
breaking change takes before 1.0.0 under `docs/maintenance.md`, Releases
and versions).

### 3.1 The decision record, extended

The item's wish list mapped onto what exists: statement is `decision`,
reason is `why`, evidence is `cites`, decider is `decidedBy`, date is
`date`, supersedes is the inverse of `supersededBy`, and the elements it
governs are the inverse of `decidedIn`. Three keys are added, all
optional:

| Key | Holds | Rule | Additive |
|---|---|---|---|
| `alternatives` | the options considered and not taken, each `option` (one line) and `why` (why it was rejected), two or more fields per entry, in the form of MADR's considered options | none beyond the schema; the techspec's decisions section and the decision's card list them | yes |
| `governs` | the elements this decision shapes, as `#/` pointers, for an element that is not `origin: decided`: a spec-first element, or one stated by a source and then shaped by a decision (a loan period the source gives, kept for a reason the decision records) | `decision_governs`: every pointer resolves (error, as `question_block`); the inverse of `decidedIn` is not repeated here (warning, `decision_governs_twice`), since one fact is said once | yes |
| `proposedBy` | who wrote a proposed decision: a stakeholder role, or `agent` (section 4) | `decision_proposer`: an accepted decision with `proposedBy: agent` carries `decidedBy`, the human role that confirmed it (error) | yes |

`supersedes` is not added: the link exists as `supersededBy` on the old
decision, which the status rule already requires, and writing it twice
would be two ways to say one thing. The techspec's decisions section
already shows it (`internal/generate/techspec.go`); `specarch model` lists
`satisfies`, `verifies` and `decidedIn` among its references and not
`supersededBy` (`internal/validate/model.go`, `anywhere`), so phase 1 adds
it there, with `governs`, for the editor to walk both ways
(`docs/diagrams.md`, G). `governs` is the one new direction, and it is needed because
`decidedIn` is tied to origin: relaxing `origin_decision` so that
`decidedIn` may stand without `origin` would change what `origin` means
(how the element is known, from sources) and would let one element name
one decision only; an element may be governed by several. That is an
owner's decision (section 3.6, question 3): `governs` on the decision, or
`decidedIn` as a list allowed without origin, which is breaking.

Two warnings, additive: `decision_reason`, an accepted decision without
`why`, since a decision whose reasoning is not written is the thing this
design is for (five of this repository's own would warn, and are written
in the same item); and `decision_decider`, an accepted decision without
`decidedBy`, reported only when the specification says `tracksOrigin:
true`, as `origin_missing` is, so that a spec-first specification like this
one does not get 174 warnings for a decider everyone knows. Making `why`
required on an accepted decision, or `decidedBy` on every accepted
decision, is breaking: meta-model 0.4, 0.8.0, and the owner's call
(question 1).

### 3.2 A reason on a change request and on a delete

A change request's `reason` and its `decision.why` are already required
(record schema). What is missing is the tie between an edit and a record,
and it exists in one place: `specarch diff`'s `covered` check, which
refuses a changed or removed element that no change request's `affects`
or defect's `violates` of the release names (`docs/maintenance.md`, The
diff verb). So a delete of a released element already needs a change
request, with its reason, before the next release. Two gaps stay:

- Between an approval and a release, inside one pre-release version, no
  check runs: `diff` has no approved tree to compare with, since the
  approval record keeps a digest and no commit, and its `version` check
  fails within one pre-release. ADR-083 step 8 (`docs/acceptance.md`,
  section 7) closes it: the approval record gains `commit`, and `diff`
  skips `version` when both sides carry the same `info.version` while
  `covered` still runs. Additive (a new optional key on a record, a new
  case in `diff`).
- An element that is retired or deprecated rather than removed: a
  requirement with `status: retired`, a decision with `status:
  deprecated`, a field with `deprecated: true`. A warning,
  `retired_reason`, for a retired requirement or a deprecated decision
  with neither `why` nor a governing decision; a deprecated field
  inherits its entity's. Additive as a warning; an error would be
  breaking (question 5).

### 3.3 The rule "an element changed since approval has a change record with a reason"

It cannot be a `validate` rule: the validator sees one version at a time,
the specification never points back at the records, and the approval holds
no tree (`docs/maintenance.md`, Releases and versions; `docs/refinement.md`,
Approval). It is `specarch diff <approved> <working>` with `covered`, run
by the editor's Compare since approval… and by the agent before it
finishes an item, once ADR-083 step 8 gives `diff` the approved commit. Two
additions to `diff`'s output make it the reader's view: after each changed
element, the reason beside it, the covering change request's `reason` and
its `decision.why`, or the accepted decision newer than the old side whose
`governs` or an element's `decidedIn` names it; and for an element with
neither, `reason: none recorded`, as a line, not as a value in any file.
Additive: `diff` gains lines; its golden cases are rewritten.

### 3.4 The view of what has no reason

`specarch gaps` is the readiness report, summary first, and already counts
elements by origin (`docs/refinement.md`, The readiness report). It gains
one section, Reasons, left out when there is nothing to say:

- elements with neither `why`, `cites`, `decidedIn` nor a decision whose
  `governs` names them, by section, named up to the first three per
  section with the count;
- accepted decisions without `why`, and without `decidedBy` when
  `tracksOrigin` is true;
- proposed decisions by `proposedBy`, so that an agent's proposals are
  listed with the questions that wait on a human;
- retired requirements and deprecated decisions without a reason.

The same figures go into the `questions` document, which is the one handed
to the stakeholders who decide (`docs/refinement.md`). The editor computes
the same list from `specarch model`'s references, filtered by `through:
decidedIn` and `governs`, without a verb of its own. Additive: new output.

### 3.5 Additive and breaking, in one table

| Part | Meta-model 0.3 | Breaking (0.4, program 0.8.0) | Owner's decision |
|---|---|---|---|
| `alternatives` on a decision | additive | | no |
| `governs` on a decision, `decision_governs` | additive | the alternative, `decidedIn` as a list without origin | yes, question 3 |
| `proposedBy` on a decision, `decision_proposer` | additive | | yes, question 4 |
| `decision_reason` warning | additive | `why` required on an accepted decision | yes, question 1 |
| `decision_decider` warning with `tracksOrigin` | additive | `decidedBy` required on every accepted decision | yes, question 1 |
| `retired_reason` warning | additive | as an error | yes, question 5 |
| approval `commit`, `diff` inside one pre-release (ADR-083 step 8) | additive | | already open under ADR-083 |
| the reason beside each element in `diff` | additive | | no |
| Reasons section in `gaps` and the `questions` document | additive | | no |
| a decision naming the change request it settles (ADR-083, 8.1) | additive either way | | yes, open since 2026-10-09 |

### 3.6 For the owner to decide

1. Required or warned: `why` on an accepted decision, and `decidedBy` on
   one that answers no question. Recommended: the warnings now, the
   errors with meta-model 0.4, after this repository's own decisions and
   the examples carry both.
2. ADR-083, section 8.1: whether a decision names the change request it
   settles under `answers`, the one link from the specification into the
   records, or the change's `decision` names the decision record. Needed
   for "what change request caused it" to be answerable from the
   element. Recommended: the first, as `docs/acceptance.md` designs it,
   since a reader starts at the element.
3. `governs` on the decision, or `decidedIn` allowed without origin and as
   a list. Recommended: `governs`, additive, and `origin` keeps one
   meaning.
4. `proposedBy: agent` as a value beside the stakeholder roles, or a
   declared stakeholder for the agent. Recommended: the value, since a
   stakeholder is a party the system is for, and the rule in section 4
   needs to tell an agent's proposal from a role's without reading the
   stakeholders.
5. A retired requirement or a deprecated decision without a reason: a
   warning or an error.
6. Which of the editor items of section 5 to queue.

## 4. When an AI agent writes the specification

The rules, each resting on one that exists:

- **An agent never writes a reason as the owner's.** `decidedBy` names
  the role that decided, and only a decision the role gave, in words or
  in a document, carries it; the answer is declared as a source of kind
  `interview` or `document` with its date, and the decision cites it with
  the answer as what it says (`docs/refinement.md`, The decision that
  closes a question; `specarch decide --by --source`). An agent writes
  `decidedBy` only through that path, never from its own reading.
- **An agent's own conclusion is marked as its own.** Where the agent
  concluded a fact from code or from a source's silence, the element is
  `origin: inferred` with a `why` that says from what, and a `should`
  question asks the owner to confirm it, as `docs/from-sources.md`
  (section 2, "only the code has it") already prescribes. Where the agent
  proposes a design choice, it writes a decision with `status: proposed`
  and `proposedBy: agent`, its reasoning under `why`, the options it saw
  under `alternatives`, and a question of kind `decision` that blocks
  `#/decisions/<ID>`, decided by the role that owns it, with the
  decision's option and the alternatives as `options`. The element the
  proposal would shape stays as it is, or is written with the question
  blocking it (ADR-017: the question is the one licence for an
  incomplete element). A proposed decision governs nothing: an element
  rests only on an accepted decision (`origin_decision`), and `governs`
  follows the same rule (`decision_governs`).
- **A human confirms.** The owner answers the question; the agent then
  sets `status: accepted` and `decidedBy` to the answering role, removes
  the question, and the decision cites the answer as its source, as every
  answered question does. `specarch decide` writes a new decision today;
  a form that accepts an existing proposed one, `specarch decide <Q>
  --accept <ADR>`, is a later step (section 6, phase 3), and until then
  the agent makes the two edits by hand and validates. `decision_proposer`
  refuses an accepted decision with `proposedBy: agent` and no
  `decidedBy`, so an agent cannot accept its own proposal by mistake.
- **An unknown reason is an absence, never a sentence.** When the reason
  for an existing element is not known, `why` is left out. When the
  absence matters, a public element, a delete, a retirement, the agent
  writes a question of kind `material` or `decision`, priority `could`
  unless something waits, asking the role for the reason, and the
  Reasons section of `gaps` lists the element until then. A `why` of
  "unknown", "reason not recorded" or "to be confirmed" is a placeholder
  (`docs/principles.md`, rule 6) and the agent never writes one; the
  validator could refuse such text, but a list of forbidden words is
  itself a guess, so the rule stays a rule for agents and reviewers
  (assumption: no validator rule).
- **Where the agent's own reason belongs.** Why the agent did what it did
  in an item goes into the commit message and, in this repository, the
  day's `history/` file, as the queue rules and `CLAUDE.md` require. The
  specification's `why` is about the system, and in a specification that
  tracks origin it is the stakeholders' account, not the agent's diary.
- **The editor's rule holds on the other side**: the Spec panel never
  answers for the owner and never runs an agent from a card
  (`apps/myowncodeeditor/CLAUDE.md`, SpecArch and the Spec panel).

These rules go into `docs/from-sources.md`, section 5, the instructions
pasted into a specified repository, as two more lines.

## 5. What the editor would need

Items for the owner to approve, none built here, each naming what in
SpecArch it waits on. They belong to `apps/myowncodeeditor` and are
written here only so the owner sees the whole.

| Item | What | Waits on |
|---|---|---|
| E1 Why panel on every card | one disclosure on each element's card: `why`, `cites` as Notes, origin, the decision from `decidedIn` or from any decision's `governs` (title, decision, why, decidedBy, date, alternatives, a link to its card), the change requests whose `affects` name the element with each `reason` and `decision.why` (as Reach reads them), the question that blocks it; when all are empty, the line "No reason recorded" with Decide this… and Ask why… (a `could` question to a role) | nothing for what exists; `governs` and `alternatives` for those |
| E2 Decide this… | sdlc item 10 as planned: one sheet that writes the decision and sets `decidedIn` with `origin: decided`, or `governs` on the decision for a spec-first element | `governs` for the second form |
| E3 History on the card | sdlc item 15 as planned: `git log -L` over the element's lines, each commit with its message, the release that includes it and the change request or defect that names it; At <ref>… beside it | nothing |
| E4 A prompt at save | on ⌘S of a card whose edit changed an element that an approval or a released release covers: Decide this…, Change request…, or Save without a reason; the third records nothing and the Review round lists the element under "changed without a reason"; never per field, never on an unreleased element | the approval's `commit` (ADR-083 step 8) to know what is covered; until then the editor's own last-commit reading |
| E5 Delete…, Retire…, Deprecate… | the same three choices on Delete…; Retire… on a requirement and Deprecate… on a decision write the status and ask for `why`, dismissable | `retired_reason` to show the warning after a dismissal |
| E6 Reasons beside the diff | Compare since approval… and the Review round show, per changed element, the reason `diff` prints, and a filter "without a reason" | section 3.3 |
| E7 Agent proposals | a proposed decision with `proposedBy: agent` carries a badge "proposed by an agent, not confirmed"; Accept… asks the role and writes `status: accepted`, `decidedBy` and the question's removal; an agent is never offered the button | `proposedBy`, `decision_proposer` |
| E8 Filter: without a reason | Filters > Without a reason, the Reasons section of `gaps` computed from the model, each row opening its card | nothing; `specarch model` already lists `decidedIn`; `governs` once it exists |

## 6. A plan in phases

The smallest first step is phase 1 alone: it needs no owner decision, adds
no required key, and gives the reader the two things most asked for, the
reasoning on every decision and the list of what has none.

1. **Phase 1, additive, 0.7.x**: `alternatives` on a decision; the
   warning `decision_reason`; the Reasons section in `gaps` and the
   `questions` document (elements and decisions without a reason, by
   section); `why` written for the five decisions of this repository that
   lack one. Spec first: the keyword in `spec/`, the ADR for this design
   written with it, both validator builds, a conformance case in
   `spec/tests/`, `docs/conventions.md` and `docs/refinement.md`. Then E1
   and E8 in the editor.
2. **Phase 2, additive**: ADR-083 step 8 (the approval's `commit`, `diff`
   inside one pre-release) and the reason beside each element in `diff`'s
   output; the agent runs it before finishing an item on an approved
   specification. Then E6, E4.
3. **Phase 3, additive, after question 4**: `proposedBy` and
   `decision_proposer`; `specarch decide <Q> --accept <ADR>`; the two
   lines in `docs/from-sources.md`'s agent instructions. Then E7.
4. **Phase 4, additive, after questions 3 and 5**: `governs` and
   `decision_governs`; `decision_decider` under `tracksOrigin`;
   `retired_reason`. Then E2, E5, E3.
5. **Phase 5, breaking, meta-model 0.4 and 0.8.0, after question 1**: `why`
   required on an accepted decision, `decidedBy` on every accepted
   decision, `retired_reason` as an error, once every specification in
   this repository and the examples carry them; a 0.3 file keeps
   validating with the `meta_model_version` warning, as a 0.2 file does
   (ADR-055).

Left out on purpose: a reason per field, parameter or step (they trace
through their holder, as `why` does today); a comment or reviewer-remark
record (S8 of the editor's plan: questions and change requests are the
construct); a person's name anywhere (roles only); and any reading of git
by the validator (`docs/maintenance.md`).
