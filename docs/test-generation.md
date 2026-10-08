# Tests from the specification: the golden path and the red paths that matter

A specification already says what its tests must show. Every subject (an
operation, a command, a page, a constraint, a transition) has a path that
succeeds and paths that are refused, and the validator derives the refused
ones from the limits, formats, permissions, relations, states and
responses the design declares. This document is the design of the step
after that: writing those derived cases as tests, choosing which red paths
to write when there are too many, saying in the specification why each
was chosen and what was left out, and turning the tests into runnable code
on each stack. The first implementation item is built: harm, mistakes,
the rank of each derived case, the narrowed warning, the left-out section
of the test plan and the Harm column. The second is built too: the
success case of every subject, the acceptance case per criterion, the
paths of a state machine and the decision-table cases of a check
constraint. So is the third: `fixture`, `input` and `expect` on a test,
checked against the design. And the fourth: `specarch derive`, which
writes the drafts. The sixth is built ahead of the fifth: the concepts the
waiting red paths needed, dependencies with a time limit, an idempotency
key, validity, sessions and guards, with their derived cases. The other
items are listed at the end.

The standards are ISO/IEC/IEEE 29119-4:2021 for the techniques and their
coverage measures, and ISO/IEC/IEEE 29119-1:2022 for why a test set is a
sample chosen by risk and not every combination (4.1.6, exhaustive
testing and sampling; 4.2.2 and 4.2.3, risks and requirements as the basis
of a test strategy).

## What exists

- Design tests: `tests/<name>/test.yaml`, one subject each, golden or red,
  system or acceptance, `given`, `when` and `then` in plain words, `covers`
  naming the derived cases the test covers, `verifies` naming the
  requirements it shows met (`docs/conventions.md`, Tests).
- Derived cases: the validator lists, per subject, the cases the rest of
  the specification implies (missing required field, a value just outside
  each limit and exactly on it, a value in the wrong form, not one of the
  enum's values, not a valid format, denied without the permission, not
  found, duplicate, a channel that fails, each 4xx or 5xx response, each
  non-zero exit code, a constraint violated, a move from the wrong state)
  and warns for every one no test covers, with a test to copy.
- Worked examples: each algorithm's examples are checked against its
  formula, and the plan for the tests target is one unit test per example.
- Documents: the test plan lists every test under its subject; the
  traceability matrix shows which requirement each test verifies.

The derivation is already equivalence partitioning (29119-4, 5.2.1: one
case per partition a limit or an enum creates), boundary value analysis
(5.2.3: just outside and exactly on each limit), syntax testing (5.2.4:
a pattern or a format broken) and the invalid half of state transition
testing (5.2.8: a move from a state the transition does not leave). What
is missing is the writing of the tests, the choice among the red cases,
the golden path of a flow, structured data a generator can run, and the
generator itself.

## What the specification says, so that tests can be derived

### Preconditions, inputs and the expected outcome

`given`, `when` and `then` stay: they are what a reader checks and what
the test plan prints. A test may add three structured keys beside them,
in the design's own vocabulary, for the generator:

| Key | Holds |
|---|---|
| `fixture` | the state before: records that exist, as entity name to a list of records with their fields; and `caller`, the role making the call, or `public` |
| `input` | what the call carries: for an operation, its parameters and body fields by name; for a command, its `arguments` and `options`; for a page, its route parameters and the action taken |
| `expect` | the outcome: for an operation, the `status` and the fields of the `body` that matter; for a command, the `exit` status and lines of `standardOutput`; for a constraint or transition, the `state` after, as entity name to records; and `emits`, the messages published, or `emitsNothing` |

A value has the type the design gives the field, written as worked
examples write theirs (`docs/conventions.md`, Expressions): a decimal and
an int64 carried as text as quoted strings, a date as a quoted date, a
timestamp as a quoted RFC 3339 instant. The validator checks the keys
against the design: a fixture's entity and fields exist and the values
have their types, an input's names are the operation's parameters and body
fields or the command's arguments and options, an expected status is one
of the operation's responses, an expected exit is one of the command's
exit codes, an expected message is one the subject emits. The same
checker that evaluates worked examples evaluates the fixture against the
entity's check constraints, so a fixture that could not exist is refused.

A test folder may still hold data files in the form the generated test
code wants, for what the structure cannot say: a large document, a binary
payload, a screenshot to compare. A generator reads the structured keys
for what they cover and the files for the rest, never both for the same
thing: a test with `input` has no `input/` folder, and a test with
`expect` no `expected/` folder (`test_data`, a new rule), so there is one
way to say each thing.

Written out, for an operation, a command and a constraint:

    fixture:
      caller: librarian
      Member: [{ id: "6f1c...", tier: standard }]
    input: { memberId: "6f1c...", bookId: "9b2e..." }
    expect:
      status: 201
      body: { status: open }
      emits: [loan.lifecycle/LoanCreated]

    input: { arguments: { paths: spec }, options: { check: true } }
    expect: { exit: 0, standardOutput: ["specarch: 1 input checked: 0 errors"] }

    expect: { state: { Loan: [{ status: returned }] }, emitsNothing: true }

`caller` is a role of the specification or `public`. A record names only
the fields that matter; the check constraints are evaluated on the ones
it gives, and a constraint that names a field it leaves out is not
evaluated. A command's `arguments` are its arguments by name, a list of
values for a repeatable one, and its `options` its options by name. A
page's input is its route parameters and `action`, the label of one of
its actions. `status` and `exit` are whole numbers; `body` holds fields of
the entity the response with that status returns, when it returns one.
`emits` and `emitsNothing` are not both given. Every one of these checks
reports as `test_data`.

### Error definitions

The error definitions are the ones the design already has, and no second
list is added: an operation's 4xx and 5xx responses with their
descriptions, a command's non-zero exit codes with their meanings, a
constraint's `message`, and the refusal of a transition from the wrong
state. A derived red case names the one it expects; a generated test
asserts it.

### Criticality

A requirement may say what is at stake when it is not met:

    requirements:
      LIB-3:
        statement: A member shall have at most three open loans.
        harm: [money]

`harm` is a list from a fixed set: `data-loss`, `money`, `security`,
`safety`, `privacy`, `availability`. A requirement is critical when its
`harm` names any of them. `priority: must` is a release decision (a
release without it is not acceptable) and does not make a requirement
critical: a must requirement about the colour of a button has no harm,
and a could requirement about a backup can lose data. The two values
beyond the ones the requirement asked for, privacy and availability, are
there because a regulation and an operations team ask about them in the
same breath as the others; they are a decision made here, for review.

A subject inherits the harm of the requirements its own `satisfies`
names (a constraint or a transition does not take its entity's), and
every derived case of a critical subject, golden or red, is critical. The
traceability matrices show the harm in a Harm column once a requirement
names one.

### How often users get a field wrong

Most red cases are user mistakes, and some mistakes are made every day
while others almost never happen. The default comes from a fixed table of
case kinds, below, which is 29119-4's error guessing (5.4.1) written down
once rather than guessed afresh per project. A field may say otherwise:

    fullName: { type: string, maxLength: 200, mistakes: rare }
    email: { type: string, format: email, mistakes: frequent }

`mistakes` is `frequent` or `rare`, and moves every case about that field
to that frequency. There is no override at the subject level: a subject's
cases are about its fields, and a frequency that is not about a field is
not a user mistake.

| Case kind | Default frequency | Why |
|---|---|---|
| missing required field | frequent | the commonest mistake a client or a form makes |
| not matching its pattern, not a valid format | frequent | typos |
| not one of its values | frequent | a stale client, a misspelt value |
| shorter than, longer than, below minimum, above maximum | occasional | happens, but a form usually stops it |
| at minimum, at maximum, of N characters (golden) | occasional | the boundary that shows the limit is right |
| denied without the permission | frequent | every role meets the screen it may not use |
| not found | frequent | a stale link, a deleted record |
| duplicate | occasional | a double submit, a re-import |
| dependency fails, dependency times out | rare | but critical by nature, see below |
| repeated with the same key | frequent | a client retries every lost answer |
| key reused for another request | occasional | a client bug |
| expired, not yet valid | occasional | a record used past its validity |
| denied with expired session | frequent | every user meets it after a pause |
| guard precondition fails | occasional | a stale screen |
| concurrent write | rare | but critical by nature, see below |
| response 4xx or 5xx not covered above | occasional | |
| from wrong state | frequent | a retried request after the state moved on |
| violates a check constraint | occasional | |
| usage error, exit N | frequent | a command is typed by hand |

A case that would stop a dependency (`dependency fails`, `dependency
times out`) or that needs two writers at once (`concurrent write`) is
treated as critical whatever the subject's harm, because it is the case
nobody exercises by hand.

## Which red cases are written

Every derived case has a rank:

| Rank | When |
|---|---|
| critical | the subject is critical, or the case is one nobody exercises by hand: a failing or slow dependency, two writers on one record |
| frequent | the case's frequency is frequent, by the table or by `mistakes` |
| other | everything else |

The cases written by default are the critical and the frequent ones. The
others are left out and listed, not forgotten: the test plan gains a
section "Derived cases left out", one row per case with its subject, its
scenario and the reason in words ("occasional case, and operation
createItem satisfies no requirement with a harm"), computed from the
specification, so that writing a test for one removes its row. Every row
has rank other, so the table names the scenario in its place. The validator's `test_case_missing` warning then
names only the chosen cases that no test covers; the left-out ones are
the test plan's to show. The owner adds a left-out case by writing its
test, as today.

The reason each written case was chosen is in the test itself: a derived
test carries `origin: inferred` and a `why` that says the derivation and
the rank ("Derived from the maximum of copiesOwned; frequent: a boundary
users meet on every edit"). The specification therefore records both
halves of what the requirement asked: why each red case is there, and
what was left out and why.

This is the sampling 29119-1 asks for (4.1.6): every combination cannot be
run, so the sample is chosen by risk (4.2.2, 4.2.3), and the risk is read
from the specification, not from a tester's memory.

## The golden paths

A golden test per subject exists today as a derived requirement (every
subject needs one) without a derived body. Three golden paths are
derivable:

- Per subject: the success response of an operation (its 2xx response,
  with a fixture that satisfies every constraint of the entity it touches
  and a caller that holds the permission), the exit 0 of a command, the
  page opened by a caller who may; `expect` from the design, `fixture`
  and `input` from the entity's fields with a value inside every limit.
- Per requirement: one acceptance test per `acceptance` criterion of a
  requirement, with `level: acceptance`, `verifies` naming the
  requirement, and the criterion as its `then`. This is requirements-based
  testing (29119-4, 5.2.12), and its coverage measure (6.2.12) is the
  share of acceptance criteria that have a test, which the traceability
  matrix already shows as the verified-by column.
- Per flow: an entity with a state field is a state machine, and 29119-4's
  scenario testing (5.2.9) gives one golden test per path from the initial
  state (the `from` no transition reaches) to each terminal state (a `to`
  no transition leaves), each step's trigger as a `when`. For the library
  example that is open to returned, open to overdue to returned, open to
  lost and open to overdue to lost. State transition coverage (6.2.8) at
  0-switch is every valid transition exercised once, which these paths
  reach, plus every invalid move from a state, which `from wrong state`
  already covers. A flow whose steps are not transitions of one entity
  waits for the `flows` concept of meta-model 0.2.

A check constraint with `&&` or `||` is a decision table (29119-4, 5.2.6):
one red case per way the expression can be false, each clause of an `&&`
false alone and every disjunct of an `||` false at once. The case names
the false clauses in the expression's own text, such as
`violates copies_in_range: copies >= 0 is false`, rather than in prose
such as "copies below zero": prose would have to be written the same way
by the Go and the Swift build from two parsers, and the source text is
already the same in both. A constraint false in only one way, such as
`returnedAt == null || returnedAt >= loanedAt`, keeps the plain
`violates <constraint>`. Decision table coverage (6.2.6) is each rule
exercised once.

A test names these subjects as `requirement: LIB-3`, its cases
`acceptance 1` and on, and `entity: Loan` alone, its cases the paths such
as `open to overdue to returned`. The paths take the transitions in
document order and never visit a state twice, so both builds list them
alike.

## The red paths and the concepts they rest on

The requirement's examples of red paths, each with the concept of the
meta-model it is derived from (`docs/conventions.md` has each concept):

| Red path | Derived from |
|---|---|
| invalid, empty or misspelt input | the limits, patterns, enums and formats of the fields |
| permission denied | the permission |
| missing record | a path parameter, or a body field that is the `via` of a relation (`not found`) |
| duplicate, double submit, back and retry | a unique constraint (`duplicate`), a transition (`from wrong state`), and an `idempotencyKey` on the operation (`repeated with the same <key>`, `<key> reused for another request`) |
| limits | the boundary cases |
| dependency down | a channel the operation `emits` on, or a dependency it `calls` (`dependency fails`) |
| timeout | the dependency's `timeout` (`dependency times out`) |
| expired data | `validity` on the entity a body field names (`expired`, `not yet valid`) |
| session expiry | the `session` (`denied with expired session`, for every permission other than public) |
| concurrency | the `guard` on the operation or command (`concurrent write`, and `guard precondition fails`) |

Each concept was chosen for what a test can set up from outside: a
dependency that fails or stalls, a key sent twice, a record past its
date, a session past its limit, a record changed under a writer. What a
test cannot set up from outside, a guard's postcondition for one, is not
derived; the guard's `recordsChanged` is for the emitted script that
checks it after the change.

## The verb that writes the tests

`specarch derive <folder>...` writes, for every chosen derived case that no
test covers, a test folder `tests/<name>/test.yaml` with the subject, the
scenario, the level, `covers`, `given`, `when` and `then` as the
derivation words them, `verifies` from the subject's `satisfies`,
`origin: inferred` and the `why` above, and `fixture`, `input` and
`expect` where the derivation can fill them (a missing field: the input
without it; a boundary: the value on or past it; a wrong state: a fixture
in another state; a success: the response and a fixture inside every
limit). The name is the one the validator suggests today.

The verb goes from the specification to more specification, a direction
ADR-013 does not have; `derive` names it, and it is one verb because the
derivation is one. What it writes is a draft the author completes, not a
generated output: no header, no `--check`, and it never overwrites a test
folder that exists, so an edited test stays as the author left it. It
skips a subject a must or should question blocks and names it on
standard error, since a test of a half-defined element would be a
placeholder. Writing tests changes the specification's files, so it
voids an approval, as any edit does. Exit 0 when it wrote or had nothing
to write, 1 when the specification has errors, 2 on a usage or read error
or a specification that keeps its tests in its root file, since each
derived test needs a folder of its own. The structured data it fills is
what the design says for certain: the status of a success or a response
case, the exit of a command, 404 for a not-found case when the operation
has that response, and for a denied case 403 and a role that lacks the
permission.

## From the tests to code

The code is a generator target, `tests`, a plug-in per stack as
`docs/generators.md` describes, Go first: `specarch generate tests` runs
`specarch-gen-tests-go` for an implementation file whose language is Go,
since a plug-in named for the file's stack is looked up before the
generic one. It reads the tests and the worked examples, the
implementation file's `testing` (framework, how the suites run) and
`mappings` (which type, table, handler or function each design element is
on this stack), and writes one test per design test and one per worked
example into the folder the target owns:

- `fixture` becomes the setup: records inserted through the entity's
  mapping (a table, a repository, a constructor), the caller signed in as
  the role; `input` becomes the call through the operation's mapping (an
  HTTP request to the handler under test, a command run with its
  arguments, a page opened); `expect` becomes the assertions, each named
  by the key it checks. `given`, `when` and `then` go in as comments, so
  the generated test reads as the specification does.
- A test without structured keys becomes a test with the three marked
  steps and a body the implementation fills in, as `docs/generators.md`
  says today; the generated file names it as needing a body, and the
  implementation file's suite that runs it fails until one is written.
- A worked example becomes a unit test that calls the algorithm's mapped
  function with the inputs and asserts the expected value in its exact
  type, decimals as decimals.
- A test marked `notApplicable` becomes no test; its reason goes in the
  file's header. A test of a subject that an open question blocks is not
  generated; the gate of `docs/refinement.md` already refuses the target.

What is stack-specific stays in the implementation file: the framework
and its assertion style, how a caller is signed in (a header, a token, a
session), how records are inserted, where the generated files go. The
Swift and Flutter plug-ins follow the same request and differ only there.

## Traceability

Nothing new is needed: a derived test carries `verifies` from its
subject's `satisfies`, so the traceability matrix shows each requirement's
tests, and the test plan shows each test under its subject with its
`covers`. The two additions are the harm column in the matrix and the
"Derived cases left out" section in the plan.

## Partial specifications

`derive` and the `tests` target read every design section, so a must or
should question that blocks an element holds up the tests of that
element's subjects: `derive` skips them and says so, `generate tests`
refuses as every target does, and `specarch gaps` shows the `tests` target
as waiting on those questions. A subject whose elements carry
`origin: inferred` gets its tests like any other; the tests then show the
inference to the reviewer, which is one reason to write them early.

## Stack-neutral and stack-specific

| In the specification | In the implementation file |
|---|---|
| `harm` on requirements, `mistakes` on fields | |
| tests with `fixture`, `input`, `expect` in the design's vocabulary | the framework, the assertion style, how a caller is signed in, how records are inserted |
| the derived cases, their ranks and the reasons | which design element maps to which type, table, handler or function |
| | the output folder of the `tests` target and the plug-in's tool |

## Implementation items

In order; each changes the specification of `specarch` first, both
validator builds where it adds a rule, and the conformance cases.

1. Built. `harm` on requirements, `mistakes` on fields, the rank of each derived
   case and the "chosen" rule: `test_case_missing` warns for chosen cases
   only, the test plan gains "Derived cases left out", the traceability
   matrix gains the harm column. Both builds; the conformance case
   `validate-derived-cases-listed` changes with it.
2. Built. The golden derivations: the success case per subject with its
   expected outcome, the acceptance test per criterion (a `requirement`
   subject), the paths of a state machine (an `entity` subject alone), and
   the decision-table cases of a check constraint. Both builds, with
   cases; the test plan draws each state machine with its paths.
3. Built. `fixture`, `input` and `expect` on a test, the rule
   `test_data`, and their checks against the design, including a fixture
   evaluated against the entity's constraints. Both builds.
4. Built. `specarch derive`. Go only, like the other verbs that write.
5. Built. The `tests` target for Go: `specarch-gen-tests-go`, mapping
   fixture, input and expect through the implementation file and a harness
   the project writes; worked examples as unit tests. Until a real
   project's hand-written test can serve as the acceptance test
   `docs/generators.md` asks of every emitter, the tests of the library
   lending example are compiled beside a harness that does nothing.
6. Built. The concepts the waiting red paths needed: `dependencies` with
   a `timeout` and `calls` on an operation, `idempotencyKey` on an
   operation, `validity` on an entity, `session`, and `guard` on an
   operation or a command, each with its rule and its derived cases.
   Both builds, with cases; the techspec shows each.
7. The Swift and Flutter test plug-ins, once the first project on each
   stack exists.
