# Intent and work packages: what a product requirements document still adds

A team that writes a full specification before development, golden and red
paths included, still writes a product requirements document for two
reasons: to say why the system is built and what counts as success, and to
split the work into an ordered sequence of pieces programmers or agents
take one at a time. This document says what SpecArch already holds of
both, what is missing, the smallest design that closes the gap, and a plan
in phases. It changes no code, schema or generator.

In short:

- Most of a product requirements document is already in a specification:
  stakeholders with their concerns, needs, requirements with priority,
  status, harm and release, assumptions, constraints, questions and
  decisions. Writing those again in a second document makes two sources
  that drift.
- Four things have no home: the **problem** the system solves, the
  **goals** that say what success is and how it is measured, what is
  **out of scope** on purpose, and the **quality goals** in order, which
  decide a trade-off. They are the business requirements of ISO/IEC/IEEE
  29148 (9.3) and section 1 of arc42, and they go in the requirements
  stage beside the stakeholders.
- Half of the work split exists: each requirement names its `release`, a
  release record can be planned, and `specarch document progress` draws a
  burn-up per release (CR-41). What is missing is a unit smaller than a
  release, its order, and a check that a unit is ready to start. That unit
  is a **work package**, a record beside the release records, naming
  requirements only; its design elements and tests follow from the
  satisfy and verify links.
- `specarch plan` proposes the packages and their order from the
  reference graph the diagrams already walk, and writes a brief per
  package that a programmer or an agent can work from alone.
- The users it is shaped for are the project manager, the system analyst,
  the QA engineer and DevOps: the plan round-trips with the tracker the
  project manager already uses, the analyst answers questions instead of
  learning keys, a package is ready and done by QA's entry and exit
  criteria, and DevOps sees what a package changes in operation before it
  lands.
- Every new rule is a warning in the problems file, never a refusal. A
  specification without any of it reads as it does today.

## 1. What a product requirements document holds, and where SpecArch has it

| Part of a product requirements document | In SpecArch today |
|---|---|
| Who it is for, what they need | `stakeholders` with `concerns`, `needs` |
| Features, user stories | `requirements` with `acceptance`, refining `needs` |
| Priority, release | `priority` (must, should, could), `release` |
| What is at stake | `harm` |
| Assumptions, constraints, open questions | `assumptions`, `constraints`, `questions` |
| Why a choice was made | `decisions` with `why`, `restsOn` |
| Acceptance | `tests`, `checks`, `monitors`, commissioning records |
| Problem statement | nothing |
| Goals and success measures | nothing at the level of the system; a monitor measures a requirement |
| Out of scope, non-goals | a need with status `rejected` covers "asked and declined" only |
| Trade-off order | nothing; `harm` is what a failure costs, `priority` is what a release may go without |
| Work split into ordered pieces | `release` per requirement and planned release records; no smaller unit, no order |

The rule "one way to say one thing" (`docs/principles.md`, the Low IQ
Tax) decides what is added: only the rows that say "nothing".

## 2. Intent, in the requirements stage

### 2.1 Problem

One per specification, not a map.

```yaml
problem:
  statement: Members cannot see which books are on the shelf, so a third of the visits to the library end without a loan.
  stakeholders: [member, librarian]
  today: 31 percent of visits counted at the desk in March end without a loan.
  cites: [{ source: desk-survey-2026-03, says: "31 percent of visits ended without a loan." }]
  why: The librarians asked for an online catalogue after the survey.
```

The decision it helps: whether the system is worth building, and whether
each goal below answers the problem. A reviewer reads one paragraph before
anything else.

### 2.2 Goals

An outcome the owner of the system wants, with how it is measured. A goal
is not a need: a need is one stakeholder's wish in their own words before
anyone shapes it (29148, 6.3); a goal is the owner's outcome for the whole
system, served by many needs. The measure follows Planguage (Tom Gilb,
*Competitive Engineering*, 2005): the scale counted, the value today and
the value wanted, because a goal without a number cannot be met or missed.

```yaml
goals:
  GOAL-1:
    statement: Fewer visits end without a loan.
    measure: Share of visits counted at the desk that end without a loan.
    baseline: 31 percent (March 2026)
    target: 15 percent
    by: 1.0.0
    priority: must
    status: accepted
    measuredBy: [visits-without-loan]
```

- `measure`, `baseline`, `target`: plain sentences; a number with its unit.
- `by`: a release version, as `release` on a requirement.
- `priority`, `status`: the same words as on a requirement; `status`
  adds `met` and `dropped`.
- `measuredBy`: monitors (`spec/operation`) that read the measure on the
  live system, when one does. A goal measured by hand leaves it out.

A need names the goals it serves: `goals: [GOAL-1]`. The chain becomes
goal, need, requirement, design element, test, and each link is a plain
list of IDs as today (`docs/traceability.md`).

The decision it helps: when two needs compete for a release, the one
serving a must goal wins; when a goal is met, the needs serving only it
can wait.

### 2.3 Out of scope

What the system will not do, decided before anyone asks, so a question or
a change request that asks for it is answered by pointing here.

```yaml
outOfScope:
  OUT-1:
    statement: Members cannot reserve a book online.
    why: Reservations need a hold shelf the library does not have.
    until: 2.0.0
    needs: [NEED-7]
```

- `until`: the release it may come back in; left out, it is not planned.
- `needs`: the needs it rules out, when they were asked. A need ruled out
  this way keeps its own status; the link says why.

A rejected need says "asked and declined"; an out-of-scope entry says "we
decided this boundary". The second is written before anyone asks, so it is
a different statement.

The decision it helps: triage of a change request is a lookup, not a
meeting, and an agent filling a gap does not build what was ruled out.

### 2.4 Quality goals

The three to five qualities that matter most, in order, after arc42
section 1.2 and the quality characteristics of ISO/IEC 25010. The order is
what decides a trade-off.

```yaml
qualityGoals:
  QG-1:
    rank: 1
    quality: functional-suitability
    statement: A loan shown as returned has been returned.
    why: A wrong state sends a member to an empty shelf, which is the problem itself.
  QG-2:
    rank: 2
    quality: interaction-capability
    statement: A member finds a book's shelf in under 30 seconds on a phone.
```

A decision names the quality goals it weighed under `weighs: [QG-1, QG-2]`,
so the techspec prints which side won and why.

The decision it helps: when latency and accuracy, or speed of delivery and
completeness, pull apart, the rank answers it before anyone argues.

### 2.5 Rules

All warnings, at the entry and in the problems file, on lean and full,
except `problem_missing` (below). Nothing refuses a specification for
missing intent.

| Rule | When |
|---|---|
| `problem_missing` | the specification has requirements and no `problem` |
| `goal_unmeasured` | an accepted goal without `measure` or `target` |
| `goal_unserved` | an accepted goal no accepted need names |
| `need_without_goal` | goals exist and an accepted need names none |
| `scope_conflict` | an accepted requirement refines a need an out-of-scope entry rules out |
| `quality_goal_rank` | two quality goals share a rank |

On the lean profile `problem_missing` is not a warning but the first
question `specarch gaps --next` asks: on a small tool a warning that shows
on every run gets ignored, and teaches people to ignore the others. On
full it is a warning.

`need_without_goal` fires only once the specification has any goal, so an
existing specification upgrades with no new warnings but `problem_missing`.
A reference to a missing goal, out-of-scope entry or quality goal is the
usual unresolved-reference error at the entry.

### 2.6 Outputs

- `specarch document intent` writes `intent.md`: problem, goals with
  measures, out of scope, quality goals, about one page. It is the first
  thing a reviewer and an agent read.
- The techspec opens with an Intent chapter in the shape of arc42 section 1.
- The traceability matrix gains a goal column.
- `specarch gaps --next` ranks a missing problem and an unmeasured goal
  first: every other answer rests on them. `specarch init` asks for the
  problem, the goals and what is out of scope as plain questions, so the
  analyst answers in sentences and the keys are written for them.
- `specarch document progress` adds, per goal, the baseline, the target,
  the latest value a monitor read, and the share of its requirements met.

## 3. Work packages, in the records

### 3.1 Why a record and not the specification

A work package describes the delivery, not the system: it changes when a
programmer is ill or an agent finishes early, while the system it builds
does not. Release records are already kept apart for the same reason
(`docs/maintenance.md`, Releases and versions). The word is the work
package of ISO/IEC/IEEE 12207 and of a work breakdown structure, because
"batch", "ticket" and "story" each mean something else in some tracker.
The documents say once that a work package is what a tracker may call an
epic or a batch, so a project manager maps it on first reading.

```yaml
specarchRecord: "1.0.0"
kind: workPackage
id: WP-3
release: 1.0.0
title: Lend and return a book
requirements: [LIB-4, LIB-5, LIB-9]
after: [WP-1, WP-2]
assignedTo: alex
status: planned
effort: 3 days
why: Lending needs the member and catalogue entities of WP-1 and WP-2.
```

- `requirements`: the only content. The design elements are those that
  satisfy them, the tests those that verify them. Naming elements too
  would be a second way to say the same thing.
- `after`: the packages that must be done first.
- `assignedTo`: a person or a team. A project manager plans with names,
  and a team name does not answer who is late. A record is not the public
  specification, and a person's name is the one detail of them it holds.
- `status`: proposed, planned, ready, in-progress, done, withdrawn.
  `ready` and `done` are checked, not just written (3.2): ready is QA's
  entry criterion, done its exit criterion.
- `effort`: as on a change request, so the progress burn-up counts it.

### 3.2 Rules

| Rule | Level | When |
|---|---|---|
| `work_package_cycle` | error at every entry on the cycle | `after` comes back to its own package, as `decision_cycle` |
| `work_package_order` | warning | an element the package's requirements need is satisfied only by a package not in its `after` chain |
| `work_package_overlap` | warning | one requirement in two packages; split the requirement instead |
| `work_package_release` | warning | a requirement's `release` differs from its package's |
| `work_package_not_ready` | warning, only on a package marked `ready` | a requirement not accepted or without acceptance criteria, an open question on an element that satisfies it, or `work_package_order` |
| `work_package_unverified` | warning, only on a package marked `done` | a golden or red test that verifies one of its requirements has no passing run in the records |
| `work_package_unconfigured` | warning | the package's requirements add a setting that no deployment of an implementation file gives a value |

An accepted requirement of a planned release in no package is not a
problem; `document progress` lists it as unplanned.

### 3.3 `specarch plan`

- `specarch plan [--release <version>]` proposes packages for the
  requirements of a release that no package holds: it sorts the reference
  graph (entities before the paths that use them, a called module before
  its caller, a path before the page that calls it, a job after what it
  writes) and groups requirements that satisfy the same subject. It writes
  records with `status: proposed` and prints them; a person accepts, splits
  or merges them.
- `specarch plan --next` prints the first package whose `after` packages
  are done and which passes the readiness check.
- `specarch plan --brief WP-3` writes one page for whoever takes the
  package: its requirements with their acceptance criteria, the design
  elements to build, the tests that must pass, the goals it serves, the
  out-of-scope entries near it and the quality goals in order. A
  programmer or an agent can work from it with nothing else, and the
  brief is the same for both. For DevOps it lists what the package
  changes in operation: settings, migrations, jobs, processes and
  monitors.
- `specarch plan --export csv|json` writes the packages for a tracker, and
  `--import` reads back what the tracker owns. The specification owns the
  scope and the order (`requirements`, `after`, `release`); the tracker
  owns `status`, `assignedTo` and `effort`. Each side wins on its own
  fields, and a field changed on the wrong side is a warning naming both
  values, so the project manager keeps one tool and the plan does not
  drift.
- The test plan groups a release's tests by package, so QA tests each
  package as it lands.
- `document progress` is the weekly report: per goal the baseline, today
  and the target; per package done, in progress, or blocked and by what;
  and the open questions with the role that must answer.

The decision it helps: who takes what next, and whether they can start
now without waiting on an answer.

## 4. Plan in phases

1. Intent: `problem`, `goals`, `outOfScope`, `qualityGoals`, `goals` on a
   need, `weighs` on a decision, the six rules, `document intent`, the
   techspec chapter, the goal column, and SpecArch's own specification
   writing its intent first. Optional keys in the 1.0 schemas, a minor
   toolchain release, both validator builds.
2. Work package records, their seven rules, per-package lines in
   `document progress` and the test plan.
3. `specarch plan` with `--next`, `--brief`, `--export` and `--import`.

Each phase records its decision (the next free ADR) and its requirements
in `spec/` before the code, and passes `specarch validate spec examples`.

## 5. Decisions

Taken by the owner on 2026-11-04, judged by the comfort of the project
manager, the system analyst, the QA engineer and DevOps:

1. Goals are their own section, not a need with a level: a goal has a
   measure and an owner a need does not, and the analyst keeps needs in
   the stakeholders' own words.
2. Work packages are records, round-tripping with the tracker (3.3).
3. `problem_missing` is a warning on full and a `gaps --next` question on
   lean (2.5).
4. `assignedTo` names a person or a team (3.1).
