# State machines: control state, extended state, guards, updates and the rest

A system's state machines, the owner says, are among the most important
parts of its architecture: which states a thing can be in, what moves it,
under what condition, and what else changes with the move. SpecArch models
an entity's single control state and its moves, as `stateField` and
`transitions`, and nothing more. This document designs the rest: control
state and extended state variables with types, initial values and
invariants; a condition and updates on a transition; several machines per
entity and machines of things that are not stored; entry and exit actions,
nested and parallel states, history, time, initial and final states; the
checks in both validators and the diagram. Each concept is mapped to UML
2.5.1 and to W3C SCXML 1.0 with the reason SpecArch follows or departs from
each. It changes no code, schema or generator: the decision records
ADR-189 to ADR-192 hold the choices, approved by the owner on 2026-10-26
before anything is built and revised the same day at the owner's word
that nothing in a machine may depend on the implementation or on a
reading; ADR-193 holds the specification's time zone and ADR-194 the
rule for every date-time of a specification (12), both proposed with
that revision.

In short:

- A machine has a control state, one value of one enum, and extended
  state, the variables a condition reads and an update writes. The terms
  are Harel's (1987); UML and SCXML keep them. An entity's fields are its
  machines' extended state; a machine without an entity declares
  `variables`.
- Today's `stateField` and `transitions` on an entity keep their meaning
  and stay the ordinary spelling of an entity with one flat machine. A
  richer machine, a second machine of an entity, or a machine with no
  entity is a named element under `stateMachines` (ADR-189).
- A transition says what causes it (`trigger`, as today, `after` a
  duration, or `at` a moment, an instant or a date read in the
  specification's `timeZone`), under what condition it fires (`when`, an
  expression over the extended state and `event.<property>`), what else
  it sets (`set`, variable to expression), and `otherwise` for the
  transition that fires when no sibling's condition holds (ADR-190).
- A state may carry `entry` and `exit` (what is set and what is emitted),
  an `invariant`, child `states` with their `initial`, and `history`;
  `final` names the final states; the initial state of a stored machine is
  the field's `default`; parallel states are several machines of one
  entity, one field each, a machine `within` the states of another when
  its life is scoped to them (ADR-191).
- The validator reports an unreachable state, a dead end that is not
  final, an enum value no transition names, a transition that says
  nothing of its cause, a condition or update naming an unknown variable,
  and two transitions from one state on one cause that cannot be told
  apart; the diagram shows the states, the causes, conditions and updates
  on the arrows, the variables, the nested states and the parallel
  machines (ADR-192).
- Nothing is left to the implementation or to a reading: the
  specification's `timeZone`, an IANA name, says where a date meets the
  clock; the instant a state was entered is a field the machine implies,
  set with every move; every move is one change on one record, so no two
  causes are simultaneous; a timer's life, what an update reads, when a
  message goes out, when an invariant is checked and the order of exits
  and entries are written in 3.4, 3.5, 3.6 and 3.8; where the design
  cannot say which transition fires, the validator reports it (ADR-193).
  Every date-time of a specification is an instant with its offset, a
  zoneless one says why, and a plain date stays a date (12, ADR-194).
- Everything is additive: a 1.0 file gets no new error, since every rule
  that can fire on one is a warning, and this repository's specification
  and examples get no new line once the library lending example declares
  its `timeZone`, so the step is a minor one, 1.3.0.

## 1. What SpecArch holds today

An entity names one of its enum fields as `stateField` and lists
`transitions`, each `from`, `to` and an optional `trigger` naming an
operation, a command, a channel message (`channel/Message`) or an
algorithm, with `description`, `why`, `satisfies` and `cites`
(`schema/specarch-design-1.0.schema.json`, `$defs/transition`). The
validator checks that the field exists and is an enum (`state_field`),
that `from` and `to` are its values (`state_value`) and that the trigger
exists (`trigger`), in both builds. The initial states are derived as the
states no transition reaches and the terminal states as those no
transition leaves; the technical specification draws one Mermaid
`stateDiagram-v2` per entity with a state field, with the trigger on each
arrow (`internal/generate/diagrams.go`, `stateDiagram`); the test plan
derives one path per way from an initial to a terminal state and one red
case per transition, `from wrong state` (`docs/conventions.md`, Tests;
`docs/test-generation.md`); `specarch-gen-scripts` writes a guarded script
for an entry that triggers transitions of one `to` (ADR-168). A design
test names a transition by its entity, `from` and `to`.

Around it: an operation's or command's `guard` is the checked data change,
its entity, a `precondition` over the records as they are, the exact
`recordsChanged` and what it `writes` (ADR-158); a page action's `when` is
the condition under which the action is offered; a check constraint's
`expression` is the invariant of an entity's records; every expression is
in the subset of CEL of `docs/conventions.md`, in which `caller.<property>`
is the one field access (ADR-095); a job runs on a `schedule`, `every`
duration or for each message it `consumes`; a page has a fixed set of
states and no machine of its own (ADR-035). Enum values are written
`snake_case` (`$defs/enum`), so the owner's `ptNotStarted` is written
`not_started`.

What cannot be said: a variable beside the control state that a move
reads or changes (a retry count, a deadline, the last error); a condition
on a move; a second control state of one entity; a machine of a device,
a connection, a session or a protocol that no entity stores; what happens
on entering or leaving a state; a state inside a state; a move caused by
time; which state is initial or final when the derivation is ambiguous.

## 2. The terms and the two standards

Harel's statecharts (1987) added to finite automata the three things a
real system needs: hierarchy (a state inside a state), orthogonality
(states that hold at the same time) and broadcast, and with them
conditions on transitions and actions on states. UML's behaviour state
machines (UML 2.5.1, clause 14) are statecharts with a precise event
model: run to completion, triggers, guards, effects, entry, exit and do
behaviours, composite states with regions, history, choice and junction
pseudostates, time and change events, final states, and firing priorities
for conflicting transitions. SCXML (W3C, 2015) is statecharts as an
executable XML document with a data model: `<state>`, `<parallel>`,
`<transition event cond target>`, `<initial>`, `<final>`, `<onentry>`,
`<onexit>`, `<history type>`, `<datamodel>`, `<data>`, `<assign>`,
`<send delay>`, and an algorithm, in document order, that says exactly
which transitions fire.

The owner's two terms are theirs: the **control state** is the one value
that says where the machine is, and the **extended state** is the
variables a condition reads and an update writes. SCXML calls the second
the data model; UML calls its values the attributes of the context
object. SpecArch keeps the owner's words, since they name the split a
reader has to see first: the control state is an enum value and the
extended state is everything else.

### 2.1 The mapping

| Concept | UML 2.5.1 | SCXML 1.0 | SpecArch | Follows or departs, and why |
|---|---|---|---|---|
| the machine | a StateMachine with regions, owned by a classifier (its context) | the `<scxml>` document | `stateMachines.<Name>`, with `entity` or standalone; an entity's `stateField` and `transitions` are one such machine named after the entity | follows UML in tying a machine to its context object (the entity) and SCXML in letting one stand alone; departs in having two spellings, since the one-field flat machine is the common case and already written in every specification (ADR-189) |
| control state | the active state configuration (one state per region) | the configuration (one atomic state per region) | one value of one enum, held in a field (`stateField`) or named by `stateEnum` | follows both: a configuration is one leaf per region, and a region is one field; departs in making every state an enum value, so that the stored column and the machine have one vocabulary |
| extended state | attributes of the context object | `<datamodel>` with `<data id expr>` | the entity's fields, or `variables` (a field each, `default` as the initial value) | follows both; SpecArch's `field` already has the type, width, format and default, so no second shape is made |
| state invariant | State::stateInvariant, a Constraint | none | `invariant` on a state, an expression that must hold while the machine is in it | follows UML; the entity's check constraints already hold the invariants that do not depend on the state |
| event, trigger | a Trigger naming an Event (call, signal, change, time) | the `event` attribute, a name matched by prefix | `trigger`: an operation, a command, a message or an algorithm, as today | follows both in kind; departs from SCXML's prefix matching, which SpecArch has no event names for, and from UML's change event (`when(expression)`), whose polling a design cannot promise (open question 10.4) |
| time event | `after(duration)`, `at(time)` | `<send delay>` on entry, cancelled on exit | `after`, an ISO 8601 duration since the state was entered; `at`, an expression giving an instant, or a date read as the first instant of that day in the specification's `timeZone` (3.8) | follows UML's two words; the duration is SpecArch's own `duration` (days, hours, minutes, seconds), not SCXML's CSS2 form; the zone is the specification's, never the implementation's (ADR-193) |
| guard | `[expression]`, a Constraint on the transition | `cond`, an expression of the data model | `when`, an expression over the extended state and `event.<property>` | follows both in meaning; departs in the word, since `guard` already names an operation's checked change (ADR-158) and `when` already names a page action's condition, and one word keeps one meaning |
| else | `[else]` | `<else>` in executable content; transitions are tried in document order | `otherwise: true`, the one transition of a state and cause that fires when no sibling's `when` holds | follows UML's `[else]`; departs from SCXML's document order, which chooses silently where SpecArch reports an ambiguity (ADR-190) |
| effect | the transition's effect Behavior | executable content: `<assign>`, `<send>`, `<raise>`, `<log>` | `set`, a variable to an expression, in the shape `writes.set` already has; `emits` on a state's entry and exit | follows both in what it does; departs from arbitrary behaviour, which a design cannot say without a stack: a transition sets variables, a state emits messages |
| entry, exit, do | entry and exit Behaviors, a doActivity | `<onentry>`, `<onexit>` | `entry` and `exit` on a state, each `set` and `emits`; no do activity | follows both for entry and exit; departs in leaving the do activity out, since what runs while the machine waits is the operation or job that drives it, already in the design |
| event data | the event's parameters, as the Trigger's event | `_event.data` | `event.<property>`, the second reserved record after `caller` | follows SCXML; the one place a `.` appears beside `caller`, for the reason ADR-095 gave: one record, declared properties, no join |
| composite state | a State with one region (nested states) | `<state>` holding `<state>` | `states` inside a state, with `initial` | follows both; a transition to the parent enters its initial child, as UML's default entry does |
| parallel state | a State with two or more regions, or several regions of the machine | `<parallel>` | several machines of one entity, one field each; `within` when a machine lives only while another is in given states | departs from regions inside one state: a stored record holds one value per field, so each region is a field and its machine, which also gives each its own diagram and tests; `within` keeps the scoped life of a region (ADR-191) |
| history | shallow and deep history pseudostates | `<history type="shallow\|deep">` | `history: shallow` or `deep` on a composite state; a transition `to` it with `enter: history` | follows both in meaning; departs in spelling, since a pseudostate is a vertex SpecArch has no word for, and the target is still the state |
| initial | the initial pseudostate of a region | `initial` attribute or `<initial>`, the first child by default | an entity machine's initial state is the field's `default`; a standalone machine and a composite state write `initial`; a short-form entity without a default keeps today's derivation | departs from SCXML's first-child default, which decides silently; a record is created with its field's default, so that is the initial state and a second spelling is refused |
| final | a FinalState | `<final>` | `final`, the list of final states of a machine; derived as today for the short form | follows both; a final state has no outgoing transition, and a composite's completion is a transition with no cause from it |
| completion transition | a transition with no trigger, taken when the source completes | an eventless transition, taken when `cond` holds | a transition with no `trigger`, `after` or `at` from a composite state, taken when a child in `final` is reached; from a simple state it is a warning, `transition_cause` | follows UML; departs from SCXML's eventless transition on a simple state, which fires as soon as its condition holds and so needs an evaluation loop the design does not promise |
| conflicting transitions | 14.2.3.9.3: transitions in conflict; priority to the deeper source; otherwise undefined | 3.13: the deeper source wins, then document order | an inner state's transition wins over an ancestor's on the same cause; two from one state on one cause are a warning when neither says what tells them apart or the validator cannot show their conditions exclusive, and an error when one is told apart and the other left unsaid (3.4) | follows both on depth; departs from SCXML's order and UML's undefined, since SpecArch does not pick a meaning for an ambiguous entry (`docs/principles.md`) |
| run to completion | 14.2.3.9.1 | the macrostep | the order exit, set, entry, within the operation or job that causes the move, as one change on one record; no move starts inside another, and a message an entry or exit emits goes out after the change | follows both |
| choice, junction, fork, join | pseudostates | none (conditions and `<parallel>`) | none: `when` and `otherwise` on the transitions out of a state say what a choice says | departs from UML's vertices, as SCXML does: every decision is on a transition, so each has a cause and a condition the validator reads |
| deferred events | a State's deferrableTrigger | none | none | departs: an event not handled in a state is refused by the operation, `from wrong state`, which the test plan already derives; queuing it is a stack's choice |
| internal and local transitions | transition kinds | `type="internal"` | a transition with `from` and `to` the same state runs exit and entry (external); no internal transition | departs for now: an internal transition is a `set` without a move, which an operation's `writes` says; open question 10.5 |
| submachine state | a State referring to another StateMachine | `<invoke>` | none | departs: a machine named twice is written twice; nothing has asked for it |

## 3. The meta-model

### 3.1 Two spellings of a machine

The short form is today's: an entity's `stateField` and `transitions`.
It is read as one machine named after the entity, whose control state is
the field, whose extended state is the entity's other fields, whose
initial state is the field's `default` or, without one, derived as today,
and whose final states are derived as today. The derivation runs over the
states the transitions name, as the generators do today: initial are
those no transition reaches, final those none leaves. An enum value no
transition names is outside the machine, which the validator reports
(`state_unused`, 4), and is never initial by derivation. Its transitions
take every key of 3.4.

The full form is an element of `stateMachines`, a map at the root of the
design keyed like entities (`PascalCase`):

    stateMachines:
      JobRun:
        description: The life of one job, from not started to stopped.
        entity: Job
        stateField: stateJob
        states: ...
        final: [stopped]
        transitions: ...
      Session:
        description: One sign-in session of a client.
        stateEnum: SessionState
        initial: idle
        variables: ...
        constraints: ...
        states: ...
        final: [closed]
        transitions: ...

| Key | Meaning | Required |
|---|---|---|
| `description` | what the machine is the life of | no |
| `entity` | the entity whose record holds the machine's state | with `stateField` |
| `stateField` | the enum field of that entity holding the control state | with `entity` |
| `stateEnum` | for a machine with no entity, the enum whose values are its control states | without `entity` |
| `variables` | for a machine with no entity, its extended state: a map of `memberName` to a field, as an entity's `properties`; each with a `default`, its initial value | no |
| `constraints` | for a machine with no entity, the invariants of its variables, as an entity's `constraints` of kind `check` | no |
| `initial` | for a machine with no entity, the initial control state | without `entity` |
| `final` | the final states, each a value of the enum | no |
| `within` | the machine runs only while another machine of the same entity is in one of the named states (3.6) | no |
| `states` | what a state carries beyond its name (3.5), keyed by enum value; a state not listed is a simple state | no |
| `transitions` | as today, each with the keys of 3.4 | yes |
| `satisfies`, `why`, `cites`, `origin`, `decidedIn`, `audience` | as on every element | no |

An entity's field is the control state of at most one machine: a full-form
machine naming the field the entity's own `stateField` names, or two
machines naming one field, is an error at the second (`state_field`). An
entity machine has no `variables`, `constraints` or `initial`: its fields,
its constraints and the field's `default` are those (a schema error). A
machine with no entity is held wherever its implementation holds it, in
memory, in a device, in a session store; the design says only what it is.

### 3.2 Control state

The control state is one value of one enum, always: the state field's
enum for an entity machine, `stateEnum` for the rest. Every state,
composite or simple, is a value of that enum. A record holds a simple
state; a transition to a composite state enters its initial child or its
history (3.5), so a stored value never names a composite state. Every
`from` and `to` is a value of the enum (`state_value`, as today).

### 3.3 Extended state

The extended state of an entity machine is the entity's other fields,
every one of them, with their types, widths, defaults and constraints as
declared; nothing is declared twice. Two machines of one entity share it,
and each other's control field is part of it, so a condition of one may
read the state of the other. A machine with no entity declares
`variables`, each a `field` as an entity's `properties` has them, with a
`default` as its initial value; a variable with no default is null at
the start and must allow null (a schema error otherwise). `constraints`
hold what must always hold across the variables, as an entity's check
constraints do. Both are read in `when`, `set`, `invariant` and `at`
by their names, as a check constraint reads an entity's fields.

### 3.4 Transitions

    - from: starting
      to: starting
      trigger: job.failed/JobFailed
      when: retryCount < maxRetries
      set: { retryCount: retryCount + 1, lastError: event.reason }
      description: A failed start is tried again until the retries are used up.
    - from: starting
      to: stopped
      trigger: job.failed/JobFailed
      otherwise: true
      set: { lastError: event.reason }
    - from: warming
      to: steady
      after: PT30S
    - from: running
      to: stopping
      at: deadline
    - from: paused
      to: running
      enter: history
      trigger: resumeJob

| Key | Meaning |
|---|---|
| `from`, `to` | as today; a `from` that is a composite state means from it or any of its descendants; a `to` that is a composite state enters its `initial` child, or, with `enter: history`, the child it last left |
| `trigger` | as today: an operation, a command, a `channel/Message` or an algorithm |
| `after` | a `duration` (ISO 8601, days to seconds) since the `from` state was entered; leaving the state cancels it |
| `at` | an expression over the extended state giving the moment the transition fires: a `timestamp`, an instant, or a `date`, read as the first instant of that day in the specification's `timeZone` (3.8); a specification with a `date` in `at` and no `timeZone` is the error `time_zone` at the transition; a name it reads that may be null needs a `when` ruling null out, in the form 3.8 names, `expression_type` otherwise |
| `when` | an expression giving a bool over the extended state and, with a `trigger`, `event.<property>`; the transition fires only when it holds, evaluated at the instant the cause is applied, over the record as it then is |
| `otherwise` | `true`: the transition fires when no other transition of the same `from` and cause has a `when` that holds, read at the instant the cause is applied, for a time transition among the transitions due at that moment (3.8); one per `from` and cause |
| `set` | a map of variable to expression, the shape of `writes.set` (ADR-158), each a value of the variable's type over the extended state and `event.<property>`; never the control field (the move sets it), never the primary key |
| `enter` | `history`: enter the composite `to` state at the child it last left (3.5) |
| `description`, `why`, `satisfies`, `cites`, `origin`, `decidedIn` | as today |

A transition has at most one of `trigger`, `after` and `at`, its cause.
One with none is a completion transition: from a composite state it fires
when the machine reaches a child the state's `final` names; from a simple
state it says nothing of what causes it, which is read and drawn as today
and reported as the warning `transition_cause`.

`event` is the second reserved record after `caller`, read only with a
`trigger`: for a message, its payload's properties; for an operation, the
request body's properties and its parameters; for a command, its
arguments and options; for an algorithm, its inputs. A name in `when` or
`set` is a variable of the machine, a field of the entity (the control
fields of its other machines included), or `event.<property>`;
`expression_name` reports any other, and `expression_type` a value of the
wrong type, as for every expression. `caller` is not read in a machine: a
move is caused by the trigger, whoever called it, and the permission is
on the operation.

The order of a move, after UML's run to completion: the `exit` of the
states left, innermost first; the transition's `set`; the `entry` of the
states entered, outermost first; a self-transition (`from` and `to` the
same state) runs both. All of it runs in the operation, job or
implementation step that causes the move, as one change with the move
itself; the guarded script of ADR-168 takes `when` into its precondition
and `set` into what it writes. Since the `entry` of the state entered
runs after the transition's `set`, a variable both set holds the entry's
value afterwards: a value a move records goes into a variable the target's
`entry` does not set.

What a move reads and tells, so that no two implementations differ:
every expression of one `set`, a transition's or a state's, reads the
extended state as it was before that `set`, so the order of the map's
entries means nothing, and the entry's `set` reads what the transition's
`set` left; a message `emits` names is published once the change that
made the move is committed, never before, so a consumer never sees a
state the change did not keep; no move starts inside another, so a
message a move emits that triggers the same machine is a later move, on
the record as the first left it. Moves of one record are serial: each
runs as one change under the record's lock, so two causes are never
simultaneous. Of two that arrive at one instant one is applied first,
and the other is applied to the state the first left: a trigger with a
transition from that state fires it, one without is refused `from wrong
state`, the red case the test plan derives, and a time transition whose
`from` state was left does not fire (3.8). The order of
the `transitions` list says nothing of which fires, ever.

Two transitions of one state on one cause are told apart by their
conditions. A transition of a descendant state wins over one of an
ancestor on the same cause (UML 14.2.3.9.3, SCXML 3.13), so those never
conflict. The cause of a completion transition is the completion of its
composite `from`; transitions with no cause from a simple state are not
grouped, since each already carries `transition_cause`, which names what
is missing. Among the transitions of one `from` on one cause (a trigger; an `after`
of one duration; the `at` transitions of the state due at one moment,
which the validator, unable to see the data, holds every `at` of the
state to be, since two moments given by data may fall together, 3.8; a
completion):

- two with neither `when` nor `otherwise` are the warning
  `transition_ambiguous` at each: the design does not say which fires and
  the operation decides, as it does today; `when` or `otherwise` on one
  settles it. A warning, since every such pair written today is valid
  and a 1.x makes no valid file invalid (ADR-186);
- one with neither `when` nor `otherwise` beside one with either, two with
  the same `when`, or two `otherwise`, are the error
  `transition_ambiguous` at each: the author began telling them apart and
  left one unsaid, and no 1.0 file holds such a pair;
- two with different `when` are exclusive when the validator can show it:
  one is the negation of the other (`!(a)` against `a`, or `a == b`
  against `a != b` over the same two operands), or each compares the same
  name with a literal and the two ranges do not meet (`n < 3` against
  `n >= 3`, `s == "a"` against `s == "b"`), or an operand of one's
  top-level `&&` is exclusive with the other, or with an operand of the
  other's top-level `&&`, by these two rules (`a != b && n < 3` against
  `a == b`), since a conjunction holds only where each operand does; any
  other pair is the warning `transition_overlap` at both, which says that
  the validator cannot show them exclusive and that writing one as
  `otherwise` settles it.

The rule is exact where it can be and honest where it cannot: deciding
overlap in general is a satisfiability question, and SpecArch would
rather warn than claim. The three forms it does show are the ones a hand
writes for a choice: the else of a comparison, the ranges of one value,
and a condition narrowed by `&&`. Where the validator warns, the design
says nothing of which fires, and that is what the warning names: a pair
left so is a decision not yet taken, never one the implementation takes.

### 3.5 States

    states:
      starting:
        description: The job is being started; a failed start is retried.
      running:
        entry: { set: { lastError: null } }
        invariant: lastError == null
        initial: warming
        history: shallow
        states:
          warming: { description: The first thirty seconds after a start. }
          steady: {}
        exit: { emits: [job.left-running/JobLeftRunning] }

| Key | Meaning |
|---|---|
| `description` | what being in the state means; the enum's `valueDescriptions` holds the value's meaning, this the state's, and one may be left out |
| `entry`, `exit` | what happens on entering or leaving the state: `set`, a map of variable to expression as on a transition, and `emits`, messages published as `channel/Message` (the `emits` rule) |
| `invariant` | an expression giving a bool that holds while the machine is in the state, over the extended state |
| `states` | the child states, each with these keys again; the state is then composite |
| `initial` | with `states`: the child entered when a transition targets the state and names no history; required with `states` (`state_initial`) |
| `history` | with `states`: `shallow`, the child last left is entered again and its own children start at their initial; `deep`, the whole configuration last left is entered again; a transition with `enter: history` to a state without `history` is `transition_history` |
| `final` | with `states`: the children at which the state completes, each a child |

A composite state's entry runs before its child's and its exit after its
child's. An `after` on a composite state counts from entering it, not
from moving between its children (3.8). Creating a record is the move
into the initial state: the record is written with its defaults, then
the initial state's `entry` runs, its `set` over the new record and its
`emits` after the change, with no exit and no transition `set`; a
machine with no entity starts the same way at its `initial`. A state's
`invariant` is checked with the invariants of its ancestors at the end
of every change that writes the record, the move's own change included,
and a change that leaves it false is refused as a check constraint's
violation is, to the caller of an operation's move, and recorded for a
time transition's, which has no caller (3.8); the validator reads it as an expression and a generator
writes it as a check constraint of the entity,
`<stateField> != <state> || (<invariant>)`, which is what it means.

### 3.6 Several machines, and machines of things not stored

An entity with two independent lives, a job's progress and its link, has
two enum fields and two machines, the entity's own short form and a
full-form machine with `entity`, or two full-form machines. Each is drawn,
checked and tested on its own, and the entity's diagram shows them side
by side as concurrent regions. A condition of one may read the other's
field; an update of one may not set the other's control field (`set` never
sets a control field).

A machine whose life is scoped to another's states, UML's region inside a
composite state, says so:

    JobLink:
      entity: Job
      stateField: stateLink
      within: { machine: JobRun, states: [running, paused] }

While JobRun is not in `running` or `paused`, JobLink is in its initial
state and none of its transitions fires; entering one of those states
starts it at its initial state, and leaving them resets it. The scoped
machine is inner to the states it is within, so the order of 3.4 holds across the two, and its start and its reset are moves of it
that JobRun's move makes, in the one change: when JobRun leaves `running`
and `paused`, JobLink's active states exit first, innermost first, its
field is set to its initial state and its entry instants (3.8) to null,
and then JobRun's exits run; when JobRun enters one of them from outside
them, JobRun's entries run first, and then JobLink is started: its field
holds its initial state, its `<stateField>EnteredAt` and the
`<State>EnteredAt` of a composite initial state are set to the instant
of that move, and the entry of its initial state runs. A move of JobRun
between `running` and `paused` is no move of JobLink and touches nothing
of it: not its state, not its entry instants. A record created with a
default that puts JobRun in one of them starts JobLink in the creation,
after JobRun's initial entry; one created outside them holds JobLink's
default and null entry instants. A transition of JobLink whose `from` is
its initial state is still reachable, and an `after` on it counts from
the start. The diagram draws JobLink as a concurrent region beside JobRun, as any second
machine of the entity, with a note at it naming the machine and the
states it is within (5); the test plan takes JobRun's state as the given
of JobLink's paths.
`within` names a machine of the same entity and states of that machine's
enum (`state_machine`).

A machine of a device, a connection, a session, a protocol or a flow of
screens has no entity: `stateEnum` names its control states, `variables`
its extended state, `initial` its first state. Its triggers are
operations, commands, messages and algorithms as any machine's. A page's
own states stay the fixed set of ADR-035; a flow across pages is such a
machine whose triggers are the operations the pages call, and a page
event as a trigger is left for a later item (10.6). Where the machine
runs is the implementation's: an implementation file may later point at
it under its own key, as it points at operations.

### 3.7 Initial and final

The initial state of an entity machine is the state field's `default`:
a record is created with it, so no second place can say otherwise. A
short-form entity whose field has no default keeps the derivation of
today, the states no transition reaches among those the transitions name;
when there are two or more, or none because every state the transitions
name is reached by one of them, the warning `state_initial` asks for the
default: without one initial state the diagram draws no `[*]`, the test
plan derives no path and a record has no first state, and the problem is
reported rather than left for the reader of the diagram to notice. An
enum value no transition names takes no part in the
derivation: it is `state_unused`, not initial. A full-form entity machine
whose field has no default is `state_initial`, an error, since the full
form promises the machine is complete. A machine with no entity writes
`initial`, and a composite state writes its own.

`final` lists the states the machine ends in. The short form keeps the
derivation of today, the states no transition leaves among those the
transitions name; a short form whose transitions leave every state has no
final state, which is not reported, since a machine may run for the
record's whole life, and the test plan then derives no path and keeps its
red case per transition. In the full form a
state no transition leaves that `final` does not name is the warning
`state_dead_end`, which says to name it final or give it a way out; a
final state with a transition from it is the error `state_final`.

### 3.8 Time

`after` and `at` are the design's statement that time moves the machine;
which program watches the clock is the implementation's, and everything
else about a timer is written here.

**The moment.** An `after` is due at the instant its `from` state was
entered plus its duration, counted in seconds, a day as 86 400 of them:
a duration is a length of time, not a count of calendar days, so a zone's
clock change never stretches or shortens it. An `at` is due at the
instant its expression gives, read from the record as it is: a `set`
that moves the value moves the moment. A `date` in `at` is the first
instant of that day in the specification's `timeZone`, the IANA name
declared once at the root of the design (12.2); a specification that
writes a `date` in `at` and declares no `timeZone` is the error
`time_zone` at the transition, since the day would otherwise begin
wherever the implementation runs. A name the `at` expression reads may be null when it is a field that is
not required or whose type allows null (`docs/conventions.md`, the
`null` literal of the expression table), or a variable whose type allows
null; such a value has no moment, and a transition over one says in
words that it then never fires, with a `when` ruling null out: for each
such name, the `when` is `<name> != null`, or has `<name> != null` as an
operand of its top-level `&&`, through nested `&&`, as the worked
example's `deadline != null` does. No other form counts, not a
disjunction and not `<name> == null ? false : ...`, since the validator
decides it by form, as it decides 3.4's three exclusive forms; an `at`
reading such a name without that form is `expression_type` at the
transition, naming the name and the form to write.

**When it fires.** A time transition fires at the first instant, while
the machine is in its `from` state, at which the clock is at or past its
moment, and never earlier: the implementation's step that records the
move may be later than the moment by its own latency, which is a
non-functional requirement of the specification when it matters, not a
property of the machine. An `after` is due only from the entry it counts
from, so it is never past its moment when the state is entered; an `at`
whose moment has passed when the state is entered, or which a `set`
moves into the past, fires at once, in the next step that looks at the
record: a deadline moved behind the clock is a deadline passed.

**While it is due.** A time transition past its moment while the machine
is in its `from` state is due until it fires or the state is left; it is
never consumed unfired, unlike UML's time event, which a false guard
discards. One whose `when` does not hold at its moment does not fire and
nothing is reported, since a transition fires only when its `when`
holds; it stays due, and its `when` is read again with every later
change of the record, which is the only thing that can change the
outcome, since neither a `when` nor an invariant reads the clock: a
`set` that makes the `when` hold fires it then. One whose change an
invariant refuses (3.5) is a failure of the system's own move, with no
caller to refuse it to: the step records it once at the record, naming
the machine, the transition and the invariant, where the implementation
records a job's failed run, and does not try it again until the record
changes.

**Its life.** Leaving the `from` state ends a timer: an `after` is
cancelled with the state, and an `at` applies only while the machine is
in its state. Entering the state again starts an `after` afresh, from
the new entry instant, whether by a self-transition, by history or by
any other way in; an `at` counts no entry and is simply due or not. An
`after` on a composite state counts from the instant the composite was
entered from outside it, and a move between its descendants does not
touch that instant. When a trigger moves the machine out of the state at
an instant a timer was due but not yet applied, the trigger's change
runs first, since every move is one change on the record (3.4), and the
timer then finds its state left and does nothing.

**Which is applied.** When a step finds two or more time transitions of
one state past their moments, which latency or a stop of the
implementation allows, the one with the earliest moment is read first,
then the next, and the first that fires ends the others when its move
leaves the state; at one
moment the `at` is applied before the `after`, since a moment given by
data is the more particular statement, and a descendant state's timer
before an ancestor's, as 3.4's priority says. The cause of a time
transition is its moment: the transitions of one `from` due at one
moment are one cause, and `when` and `otherwise` are read among them, at
that moment, over the record as it then is, as 3.4 says of every cause.
Two `after` of one state with one duration are due at one moment, two
with different durations never; two `at` of one state are due at one
moment when their expressions give it, which the validator cannot see,
so for its rules of 3.4 every `at` of a state is held to be one cause,
and a pair is told apart with `when` or `otherwise` or reported. The
whichever-comes-first pair is written so: `at: deadline, when: deadline
!= null` beside `at: dueOn, otherwise: true` fires the second at `dueOn`
when the deadline is later or null, since at that moment it is the only
transition due, and the first when the deadline is earlier, or at the
same moment, when the first's `when` holds; the same pair without
`otherwise` is the error `transition_ambiguous`, since the design would
not say which fires when the two moments fall together.

**The entry instant.** For a stored entity the machine needs the instant
its current state was entered, which the entity does not declare: as the
audit fields follow from `audited` (`docs/conventions.md`, Sensitive
data, encryption at rest, audit fields and soft delete), it follows from
the machine. A machine whose transitions use `after` implies, beside its
state field, the field `<stateField>EnteredAt`, an instant (`format:
date-time`), set by the system in the same change as every move of that
machine to the instant the move is recorded, at the record's creation to
the instant of the creation, and never by a caller, as the audit fields
are; and for each composite state that is the `from` of an `after`, the field `<stateField><State>EnteredAt`, where `<State>` is the state's
value with each part between underscores begun in upper case and the
underscores dropped (`Running` for `running`, `NotStarted` for
`not_started`, so `stateJobRunningEnteredAt` for `running` of
`stateJob`), set when the composite is entered from outside it, left as
it is on a move between its descendants, and null while the machine is
not in it; two such states of one machine whose names so formed are one
(`step_2` and `step2`) are the error `state_field` at the second. A
machine `within` another has its entry instants set by the move that
starts it, to that move's instant, and set to null by the move that
resets it, so they are null while the other machine is outside the
states it is within (3.6).
The names are
fields' names in form and follow `wireNames` into the table as every
field does; `generate sql` writes the columns as it writes the audit
columns, and the technical specification lists them with the entity's
fields; a field the entity declares under one of those names is the
error `state_field` at the field, since the machine already says it. A
machine with no entity holds its entry instants as it holds its
variables, with the same meaning. A transition with `at` alone implies
no field: its moment is in the extended state already.

**Who moves it.** A time transition's move is the system's, as a job's
is, under no caller and no permission: which program watches the clock
and runs the step is the implementation's, which an implementation file
may name as it names a job's worker. `specarch-gen-scripts` writes no
script for a time transition and says so; the test plan derives `before
<duration>` and `after <duration>` for an `after`, and the same two with
the expression for an `at` (6). The library lending example's nightly
job emitting `loan.overdue/LoanOverdue` is the shape
`at: dueOn + duration("P1D")` replaces: the design then says that a loan
is overdue from the first instant of the day after `dueOn` in the
library's `timeZone`, and the job becomes the implementation of the
clock.

## 4. The validator

Every check runs in both builds and prints the same line
(`docs/conventions.md`, Two validator builds). The rules that exist keep
their names; the new ones are frozen at the release that ships them.

| Rule | Severity | When it fires, and where |
|---|---|---|
| `state_field` | error | as today; also a field that is the control state of two machines, at the second; a full-form entity machine whose `stateField` is not an enum field of its `entity`; a field the entity declares under a name the machine's entry instants take, and two composite states of one machine whose `<State>EnteredAt` names are one, at the second (3.8) |
| `state_value` | error | as today: a `from`, `to`, `initial`, `final`, `within.states` or `states` key that is not a value of the machine's enum |
| `trigger` | error | as today: a trigger naming nothing |
| `state_machine` | error | `entity` names no entity; `stateEnum` names no enum; `within` names no machine of the same entity; `enter: history` to a state that is not composite; a `to` that is a composite state without `initial` |
| `state_initial` | error, warning | a composite state with `states` and no `initial`; a full-form entity machine whose field has no `default`; a standalone machine without `initial`: errors. A short-form entity with no default whose transitions name two or more states no transition reaches, or none: a warning naming them, or saying that every state is reached, and asking for the default |
| `state_unreachable` | warning | a state that is not initial and that no transition reaches, nor is a descendant of one reached; in the short form without a default a state no transition reaches is initial by derivation, so there it fires only when the field has a default |
| `state_dead_end` | warning | full form: a simple state no transition leaves, nor any ancestor of it, that `final` does not name |
| `state_final` | error | a transition from a state `final` names |
| `state_unused` | warning | a value of the machine's enum that no transition names and that is not the machine's initial state, the field's `default` or `initial`; in the short form without a default the derivation runs over the states the transitions name, so such a value is never initial by derivation and is reported: the requirement that asked for the value has no move to or from it |
| `transition_cause` | warning | a transition from a simple state with no `trigger`, `after` or `at` |
| `transition_ambiguous` | warning, error | 3.4: two transitions of one `from` on one cause (the `at` transitions of a state are held to be one cause, 3.8) with neither `when` nor `otherwise`, a warning; one with neither beside one with either, two with the same `when`, or two `otherwise`, an error |
| `transition_overlap` | warning | 3.4: two conditions the validator cannot show exclusive |
| `transition_history` | error | `enter: history` to a state without `history` |
| `time_zone` | error, warning | a `date` in `at` in a specification that declares no `timeZone`, an error at the transition; a `timeZone` that is not a name of the IANA Time Zone Database, an error at the key; and, outside a machine (12.3), a `validity` bounded by a `date` or a `date()` of an instant with no `timeZone`, a warning at each, a `default` or `examples` value of a `date-time` field written without its offset, a warning at the value, a `default` or `examples` value of a `time-zone` field that is no name of the database, an error at the value, since the format is new, and a `local-date-time` field without `why`, an error at the field |
| `expression_syntax`, `expression_name`, `expression_type` | error | as today, for `when`, `set`, `invariant`, `at` and the `set` of `entry` and `exit`: an unknown variable or `event` property, a wrong type, a value outside the subset; a `set` naming a control field or the primary key is `expression_name`, since neither is a variable a move sets; `at` must give a timestamp or a date and `when` and `invariant` a bool, and an `at` reading a name that may be null needs a `when` ruling null out in the form 3.8 names |
| `emits` | error | as today, for `entry` and `exit` |
| `schema` | error | as today: `trigger`, `after` and `at` together; `otherwise` with `when`; `variables` on an entity machine; a key unknown |

What the validator does not do: read a requirement's prose for a state
name (the link from a requirement to a state is `satisfies` on the enum
value's transitions and on the machine, as for every element), prove two
conditions exclusive beyond 3.4, or run a machine.

The compiler test holds for each: a machine with an unknown field, a
missing initial, a composite without `initial` or an ambiguous pair is
still read, every other problem listed, the error at the entry, and the
diagram still drawn with the entry marked. The acceptance test: a change
request on a move (a new condition, one more variable) is one entry's
edit, and the problems file says what else it reaches.

## 5. The diagram

One `stateDiagram-v2` per machine, `stateDiagram <Machine>`; the marker
`stateDiagram <Entity>` keeps drawing the entity's short-form machine, and
for an entity with two or more machines draws them as the concurrent
regions of one composite state named after the entity, Mermaid's `--`. A
machine `within` another is drawn as such a region, with a note at it
naming the machine and the states it is within; it is not drawn inside a
composite state, since the states it names need not be one composite (the
example's `running` and `paused` are not), and one drawing for every case
is read without a rule.

| What | How it is drawn | Checked with mermaid-cli 12.0.0 (Mermaid 12.0.0) |
|---|---|---|
| a state | `not_started` | yes |
| the initial state | `[*] --> not_started` | yes |
| a final state | `stopped --> [*]` | yes |
| a transition | `starting --> running : job.started/JobStarted [retryCount < maxRetries] / retryCount = 0`, the cause, then `[when]` or `[otherwise]`, then `/ ` and each `set` as `name = expression`, comma separated; `after PT30S` and `at deadline` as the cause | yes |
| a composite state | `state running { [*] --> warming ... }`, nested as deep as the design | yes |
| history | the arrow to the composite state with `(history)` after its cause, `paused --> running : resumeJob (history)`; Mermaid has no history pseudostate, and an arrow from outside to a state drawn inside the composite pulls that state out of it, as the check showed | yes |
| parallel machines of one entity | `state Job { state JobRun {...} -- state JobLink {...} }` | yes |
| a machine `within` another | its region as above, then `note right of JobLink : within JobRun [running, paused]` after the region's closing brace; written inside the region, the note names a state Mermaid has not drawn yet and the render fails | yes; the failure seen in the check |
| entry, exit, invariant | a note at the state, one line each: `note right of running`, then `entry / lastError = null` and `invariant lastError == null`, then `end note`; a `state : text` line would replace the state's name with the text | notes yes, on a composite state too; the replacement seen in the check |
| extended state variables | a note at the machine: one line per variable, `retryCount int32 = 0`, `lastError string, null` | yes |

The diagram's label for a condition is the expression as written; an
expression that does not parse is drawn as written and the entry marked,
as every output marks a problem's entry (`docs/principles.md`, rule 3).

## 6. Documents, tests and generators

- The technical specification's runtime view (section 6) draws one
  diagram per machine, with the variables table and the transitions table
  (from, to, cause, when, set) after it; a standalone machine is listed
  with the entities' machines.
- The test plan derives, per machine, the paths from the initial state to
  each final state over simple states, entering a composite state at its
  initial child and a history at the child the path last left, with each
  step's cause as its `when` sentence; a machine `within` another has the
  other's state as its given; a transition with a `when` adds the red
  case `<when> not holding` (occasional); `otherwise` is golden; `after`
  adds `before <duration>` (red) and `after <duration>` (golden), and
  `at` the same with the expression; the red case `from wrong state`
  stays per transition. A design test names a transition by its machine
  (`stateMachine`, or the entity as today), `from`, `to` and, when two
  share those, its cause.
- `specarch-gen-scripts` adds `when` to the precondition and `set` to
  the writes of the guarded script, writes the `exit` and `entry` sets
  with them, and names a time transition as one it does not script.
- `specarch generate sql` writes the control field as a column as today
  and, for a machine with `after`, the entry instant columns of 3.8
  beside it, as it writes the audit columns; the technical specification
  lists them with the entity's fields as it lists the audit fields.
- `specarch model` lists `stateMachines` with their references (entity,
  field, enum, within, triggers), so the editor shows them.
- `specarch gaps` counts a machine's transitions without a cause and
  states without a description with the other gaps.

## 7. Compatibility and the version

Every change is an addition: two optional root keys, `stateMachines`
and `timeZone`; five optional keys on a transition; two formats and
`why` on a field (12); nothing removed, renamed or made required. A 1.0 file gets no new error: every rule that can fire on a
file written today is a warning, `transition_cause` on a causeless
transition from a simple state, `transition_ambiguous` on two transitions
of one state on one trigger with neither `when` nor `otherwise`,
`state_unused` on an enum value no transition names, `state_initial` on
two or more derived initial states or none, `state_unreachable` on a
short-form entity with a default whose transitions name a state none
reaches, and `time_zone` on a validity bounded by a date or a `date()` of
an instant without `timeZone`, or a date-time default or example without
its offset (12.5); every error needs a key only a new file has. On this repository's specification and the examples the one
entity with a state field, the library lending example's Loan, has a
trigger on every transition, no two from one state on one trigger, no
default and one derived initial state, and no enum value outside its
transitions, so no line of the machine rules appears; `time_zone` would
warn at the Loan's check `lentOn <= date(loanedAt)` and at the Member's
validity bounded by dates, so the build item declares the example's
`timeZone` with the rule (12.5), and then no new line appears and no
document changes but the example's own; of the conformance cases under
`spec/tests/` that carry a state field, five would gain lines, `document-testplan-state-machine`
and `validate-derived-flow` the warnings `state_unused` for `lost` and
`transition_cause` for three transitions (causeless transitions are not
grouped, so no `transition_ambiguous`), `validate-state-value` the warning
`state_unused` for `live`, and `generate-scripts` and
`generate-scripts-mariadb` two lines each, the warning
`transition_ambiguous` at each of the two `settleReservation` transitions
from `pending` and the warning `state_initial` at the Loan's state field,
whose `renewLoan` from `overdue` reaches `open`, so that every state its
transitions name is reached and none is initial by derivation; their
expected output records both when the rules are built. The other cases
with a state field have one transition per state and trigger, every enum
value named, no default and one derived initial state. Of the cases
without one, `validate-validity` and `validate-concept-cases-listed` gain
one `time_zone` warning each, at the validity bounded by a date (12.5).
The step is a minor one, 1.3.0 (`docs/maintenance.md`, rule 7; ADR-186),
with the new rule names frozen at that release. Nothing is deprecated:
the short form is not replaced, it is the full form with the machine's
own keys left out.

## 8. A worked example

A job that is started, retried a bounded number of times, runs, may be
paused and resumed where it was, stops at a deadline, and has a link of
its own that lives only while it runs. The owner's `ProgressType` with
`ptNotStarted` and the rest, in SpecArch's spelling. Generic: no system
of anyone's.

    enums:
      ProgressType:
        type: string
        enum: [not_started, starting, running, warming, steady, paused, stopping, stopped]
        valueDescriptions:
          not_started: created and never started
          starting: being started; a failed start is retried
          running: doing its work; warming for the first thirty seconds, steady after
          paused: held by an operator, to resume where it was
          stopping: asked to stop, finishing its current step
          stopped: finished, for good or after the retries ran out
      LinkState:
        type: string
        enum: [disconnected, connected]

    entities:
      Job:
        type: object
        properties:
          id: { type: string, format: uuid, readOnly: true }
          name: { type: string, maxLength: 100 }
          stateJob: { title: Progress, $ref: "#/enums/ProgressType", default: not_started }
          stateLink: { title: Link, $ref: "#/enums/LinkState", default: disconnected }
          retryCount: { title: Retries so far, type: integer, format: int32, minimum: 0, default: 0, readOnly: true }
          maxRetries: { title: Retries allowed, type: integer, format: int32, minimum: 0, maximum: 10, default: 3 }
          lastError: { title: Last error, type: [string, "null"], maxLength: 500, readOnly: true }
          deadline: { title: Stop by, type: [string, "null"], format: date-time }
        required: [id, name, stateJob, stateLink, retryCount, maxRetries]
        primaryKey: [id]
        constraints:
          job_retries_bounded:
            kind: check
            expression: retryCount <= maxRetries
            message: A job is retried no more often than it allows.

    channels:
      job.started: { messages: { JobStarted: { summary: The job's process is up., payload: { type: object, properties: { jobId: { type: string, format: uuid } }, required: [jobId] } } } }
      job.failed: { messages: { JobFailed: { summary: The job's process failed to start or died., payload: { type: object, properties: { jobId: { type: string, format: uuid }, reason: { type: string, maxLength: 500 } }, required: [jobId, reason] } } } }
      job.stopped: { messages: { JobStopped: { summary: The job's process is down., payload: { type: object, properties: { jobId: { type: string, format: uuid } }, required: [jobId] } } } }
      link.opened: { messages: { LinkOpened: { summary: The job's link is up., payload: { type: object, properties: { jobId: { type: string, format: uuid } }, required: [jobId] } } } }
      link.closed: { messages: { LinkClosed: { summary: The job's link is down., payload: { type: object, properties: { jobId: { type: string, format: uuid } }, required: [jobId] } } } }

    stateMachines:
      JobRun:
        description: The life of one job, from not started to stopped.
        entity: Job
        stateField: stateJob
        final: [stopped]
        states:
          running:
            entry: { set: { lastError: null } }
            invariant: lastError == null
            initial: warming
            history: shallow
            states:
              warming: { description: The first thirty seconds after a start. }
              steady: {}
        transitions:
          - { from: not_started, to: starting, trigger: startJob, set: { retryCount: 0 } }
          - { from: starting, to: running, trigger: job.started/JobStarted }
          - from: starting
            to: starting
            trigger: job.failed/JobFailed
            when: retryCount < maxRetries
            set: { retryCount: retryCount + 1, lastError: event.reason }
            description: A failed start is tried again until the retries are used up.
          - from: starting
            to: stopped
            trigger: job.failed/JobFailed
            otherwise: true
            set: { lastError: event.reason }
          - { from: warming, to: steady, after: PT30S }
          - { from: running, to: paused, trigger: pauseJob }
          - { from: paused, to: running, enter: history, trigger: resumeJob }
          - { from: running, to: stopping, trigger: stopJob }
          - { from: running, to: stopping, at: deadline, when: deadline != null }
          - { from: running, to: stopped, trigger: job.failed/JobFailed, set: { lastError: event.reason } }
          - { from: stopping, to: stopped, trigger: job.stopped/JobStopped }
      JobLink:
        description: The job's link to its controller, up only while the job runs or is paused.
        entity: Job
        stateField: stateLink
        within: { machine: JobRun, states: [running, paused] }
        transitions:
          - { from: disconnected, to: connected, trigger: link.opened/LinkOpened }
          - { from: connected, to: disconnected, trigger: link.closed/LinkClosed }

The operations `startJob`, `pauseJob`, `resumeJob` and `stopJob` are
declared under `paths` as any operation is, each with a permission, and
are left out here. What the validator says of it: nothing. The two
`job.failed/JobFailed` transitions from `starting` are `when` against
`otherwise`; every other `from` and cause has one transition; every value
of both enums is named; `stopped` is final and every other state has a
way out. The `after` on `warming` implies the field `stateJobEnteredAt`,
which every move of JobRun sets and `generate sql` writes; `running` is
the `from` of no `after`, so it implies no field of its own; `at:
deadline` reads the record's `deadline`, an instant, so the machine reads
no `timeZone`. What the rules of 3.4 make of a start that fails twice and then
succeeds, following `lastError` and `retryCount`: `startJob` moves
`not_started` to `starting` with `retryCount` 0 and `lastError` null, the
record's default; the first failure takes the self-transition, `0 < 3`,
`starting`'s exit and entry run and set nothing, and the record holds
`retryCount` 1 and the reason; the second the same, `retryCount` 2 and
the second reason; `job.started` moves to `running`, whose entry sets
`lastError` null, and then to `warming`, so the invariant of `running`
holds. Had the start failed twice more instead, the third failure would
have set `retryCount` 3 and the fourth found `3 < 3` false and taken the
`otherwise` to `stopped`, where the reason stays. What it would
say of a slip: a third `job.failed/JobFailed` transition from `starting`
with `when: retryCount > 0` is `transition_overlap` beside the first,
since `retryCount > 0` and `retryCount < maxRetries` compare different
names; `set: { stateJob: stopped }` is `expression_name` naming the
control field; `to: running` without `initial` under `running` is
`state_initial`; a `paused` with no transition from it is `state_dead_end`.

The diagram `stateDiagram Job`, as the generator writes it and Mermaid
draws it (checked on 2026-10-26 with mermaid-cli 12.0.0):

    stateDiagram-v2
      state Job {
        state JobRun {
          [*] --> not_started
          not_started --> starting : startJob / retryCount = 0
          starting --> running : job.started/JobStarted
          starting --> starting : job.failed/JobFailed [retryCount < maxRetries] / retryCount = retryCount + 1, lastError = event.reason
          starting --> stopped : job.failed/JobFailed [otherwise] / lastError = event.reason
          state running {
            [*] --> warming
            warming --> steady : after PT30S
          }
          running --> paused : pauseJob
          paused --> running : resumeJob (history)
          running --> stopping : stopJob
          running --> stopping : at deadline [deadline != null]
          running --> stopped : job.failed/JobFailed / lastError = event.reason
          stopping --> stopped : job.stopped/JobStopped
          stopped --> [*]
          note right of running
            entry / lastError = null
            invariant lastError == null
          end note
        }
        --
        state JobLink {
          [*] --> disconnected
          disconnected --> connected : link.opened/LinkOpened
          connected --> disconnected : link.closed/LinkClosed
        }
        note right of JobLink : within JobRun [running, paused]
      }
      note right of Job
        retryCount int32 = 0
        maxRetries int32 = 3
        lastError string, null
        deadline timestamp, null
      end note

The test plan derives for JobRun the paths `not_started to starting to
running to stopped`, `... to running to stopping to stopped`, `... to
running to paused to running to stopping to stopped` and the rest, with
`warming` and `steady` inside `running`; the red cases `retryCount <
maxRetries not holding` and `deadline != null not holding`; `before PT30S`
and `after PT30S` for the warming step; and `from wrong state` per
transition. For JobLink, two paths with JobRun in `running` as the given.

A machine with no entity, for the same system's operator session:

    enums:
      SessionState:
        type: string
        enum: [idle, challenged, open, closed]

    stateMachines:
      OperatorSession:
        description: One operator's session at the console, held in memory by the console.
        stateEnum: SessionState
        initial: idle
        final: [closed]
        variables:
          attempts: { type: integer, format: int32, minimum: 0, default: 0 }
          maxAttempts: { type: integer, format: int32, minimum: 1, default: 3 }
        constraints:
          attempts_bounded: { kind: check, expression: attempts <= maxAttempts, message: A sign-in is tried no more often than allowed. }
        transitions:
          - { from: idle, to: challenged, trigger: beginSignIn, set: { attempts: 0 } }
          - { from: challenged, to: open, trigger: answerChallenge, when: event.answer == event.expected }
          - { from: challenged, to: challenged, trigger: answerChallenge, when: event.answer != event.expected && attempts + 1 < maxAttempts, set: { attempts: attempts + 1 } }
          - { from: challenged, to: closed, trigger: answerChallenge, otherwise: true }
          - { from: challenged, to: closed, after: PT5M }
          - { from: open, to: closed, trigger: signOut }
          - { from: open, to: closed, after: PT8H }

Here the first two `answerChallenge` conditions are shown exclusive by
the third form of 3.4: the first operand of the second's `&&`,
`event.answer != event.expected`, is the negation of the first,
`event.answer == event.expected`, over the same two operands; `otherwise`
takes the rest. The two `after` transitions are on different states and
never meet, and `after: PT5M` on `challenged` is alone on its cause.

## 9. The decision records

- ADR-189: a machine is a named element, in the two spellings of 3.1; the
  control state is always one enum's value; the extended state is the
  entity's fields or `variables`.
- ADR-190: a transition's cause, condition, else and updates (3.4), the
  `event` record, run to completion, and what the validator does with two
  transitions on one cause.
- ADR-191: states with entry, exit, invariant, children, initial, history
  and final (3.5, 3.7); parallel states as several machines, `within`
  for a scoped one (3.6); time (3.8).
- ADR-192: the checks (4), the diagram (5), the documents, tests and
  generators (6), and the version step (7).
- ADR-193: the specification's `timeZone` and where a date meets the
  clock (3.8, 12.2, 12.3), and the proposal for the jobs' schedule (12.4).
- ADR-194: every date-time an instant with its offset, `local-date-time`
  with `why`, `time-zone`, a date a date (12.2), the rule `time_zone`
  outside a machine and what it means for what exists (12.5).

The owner approved the design on 2026-10-26 and asked the same day that
nothing in it be left to the implementation or to a reading, which 3.4
to 3.8 and 12 answer; ADR-193 and ADR-194 are proposed until the owner
approves that revision, and the build items follow in the order of 11.

## 10. The forks, as the owner settled them

1. The word for the condition is `when`, not `guard`, UML's word, which
   SpecArch already uses for an operation's checked change.
2. The initial state of an entity machine is the field's `default`, with
   `initial` refused on it; not `initial` on every machine with the
   default required to agree.
3. The entry instant for `after` on a stored entity is the implied field
   `<stateField>EnteredAt`, with one more per composite `from` of an
   `after`, set by the system with every move (3.8); not a field the
   entity declares. A `date` in `at` is the first instant of that day in
   the specification's `timeZone`; never the implementation's zone.
4. A change event, UML's `when(expression)` with no trigger on a simple
   state, which needs the implementation to watch the expression, stays
   out: such a transition is `transition_cause`.
5. An internal transition, a `set` on a trigger with no move and no entry
   or exit, stays out, since an operation's `writes` says it.
6. A page event (`actions`, `onSubmit`) as a trigger of a standalone
   machine for a flow of screens is a later item.
7. The names of the new rules and keys are open until the release
   freezes them.
8. Open: whether a job's `schedule` names its zone as 12.4 proposes, an
   item of its own.

## 11. Build plan, after approval

1. The schema (design and fragment design, 1.0), both validators with
   every rule of section 4 printing the same line, the conformance cases
   under `spec/tests/`, and the rule enum: one item, Go and Swift
   together, since a validator change goes into both.
2. The diagram and the technical specification, `specarch model`.
3. The test plan's paths and cases, `gaps`.
4. `specarch-gen-scripts`, and the entry instant columns of 3.8 with
   `generate sql`.
5. The time zone and the date-times (12): `timeZone` at the root,
   `local-date-time`, `time-zone` and `why` on a field in the schema,
   the rule `time_zone` in both validators with its conformance cases,
   the type-rendering idiom's rows, `docs/conventions.md` Types and the
   library lending example's `timeZone`.
6. The library lending example rewritten with
   `at: dueOn + duration("P1D")` where the owner agrees, as the
   acceptance test of the design.
7. The release, 1.3.0.

## 12. Date-times and the time zone, everywhere

The owner's rule of 2026-10-26: a date-time carries its time zone,
everywhere in SpecArch, except one that has a valid reason not to; a
plain date stays a date; and where a date is turned into an instant, the
specification's own zone applies. This section is the design of that
rule for the whole meta-model, decided in ADR-193 and ADR-194 and built
as its own item (11); `docs/conventions.md`, Types, takes the rule when
it is built.

### 12.1 What the standards say

ISO 8601 writes a time of day with a zone designator, `Z` or an offset,
or without one, which it calls local time. RFC 3339, the Internet's
profile of it, requires the offset in every `date-time`, and says of
unqualified local time that its interpretation "will fail in
approximately 23/24 of the globe" (4.4): a reading of a clock on a wall
is not an instant until the wall is placed. RFC 3339 also keeps `-00:00`
for an instant whose local offset is unknown (4.3): the instant is
exact, the local time is not, the opposite case. RFC 9557 adds to RFC
3339 a time zone name in brackets,
`2026-10-07T09:30:00+07:00[Asia/Jakarta]`, because an offset is not a
zone: a zone's offset changes with its rules, so a future wall-clock
time needs the zone's name to stay the time its people meant; the names
are the IANA Time Zone Database's, kept under RFC 6557. SQL (ISO/IEC
9075-2) has `TIMESTAMP WITH TIME ZONE`, an instant with a displacement,
and `TIMESTAMP` without one, a wall-clock reading; PostgreSQL's
`timestamp without time zone` silently ignores any zone a literal
carries, and its documentation says so, which is the trap the owner's
rule closes: a zone dropped on write and guessed on read.

Three things follow, and the design keeps them apart: an instant is a
point on the one time line and is written with an offset; a wall-clock
reading is a date and a time of day as a clock in one place shows them,
and is not an instant until a zone is named; and a zone is a name from
the database, not an offset, because only the name survives a change of
the rules.

### 12.2 The rule

| Kind | Written as | Holds | In the implementation |
|---|---|---|---|
| instant | `type: string, format: date-time` | an instant, RFC 3339 with its offset, `"2026-10-07T09:30:00+07:00"`; two writings of one instant are one value, and the offset written is not kept; `date()` of it is the date in `timeZone`, UTC's without one (12.3) | the zone-aware type: `TIMESTAMP WITH TIME ZONE`, `DATETIMEOFFSET`, UTC text on MariaDB, `time.Time`, as the type-rendering idiom has it |
| wall-clock date and time | `type: string, format: local-date-time`, with `why` | a date and a time of day with no zone, ISO 8601 local time, `"2026-10-07T09:30:00"`; `why` says why it is not an instant, and documents show it beside the field | the zoneless type: SQL's `TIMESTAMP` without time zone, `DATETIME2` on SQL Server, `DATETIME` on MariaDB; the build item names each stack's type in the idiom |
| date | `type: string, format: date` | a calendar date, `"2026-10-07"`, no zone, as today | `DATE` |
| time of day | `type: string, format: time` | a clock reading with no date and no zone, `"09:30:00"`, as today | `TIME` |
| time zone | `type: string, format: time-zone` | a name of the IANA Time Zone Database, `"Asia/Jakarta"`, the form `timeZone` takes; a default or example that is no such name is `time_zone`, an error at the value | text; the stack's zone type where it has one |
| `timeZone` at the root of the design | `timeZone: Asia/Jakarta` | the specification's one zone, an IANA name: where a date or a wall-clock reading meets the clock, this is the clock (12.3) | the zone the implementation converts in, set once from the specification, never the host's |

A `date-time` is zone-aware by default, and nothing changes for it: it is
what the format meant, what every stack's rendering keeps and what an
expression literal is checked against (`expression_type` refuses
`timestamp("...")` without its offset). A `default` or an `examples`
value of a `date-time` field is not checked today, so one written
without its offset is the warning `time_zone` at the value, a warning
since a 1.0 file may hold it. A `local-date-time` is allowed only with `why`, and
a `local-date-time` field without `why` is the error `time_zone` at the
field: a specification that holds a wall-clock reading without saying
why has left the zone to whoever reads it. The cases the design holds
valid, and why:

- A future wall-clock time at a place: an appointment at 09:00 on a day
  next year, a store's opening at 08:00, a class every Monday at 10:00.
  The instant it will be is not known today, since the zone's rules may
  change before the day comes, and the people meant the clock on the
  wall (RFC 9557's reason). The field is `local-date-time`, or `time`
  for one with no date, and the zone that turns it into an instant when
  the day comes is the specification's `timeZone`.
- A reading from an outside system that has none: a device's log, a file
  from a system that writes its own local time, an import whose zone was
  never recorded. The value is kept as it came, since inventing an offset
  would claim what is not known, and `why` names the system. (RFC 3339's
  `-00:00` is the other case, an instant known without its local
  offset, and stays a `date-time`.)
- A value whose meaning is the calendar, not the clock, is a `date` and
  needs no reason: a due day, a birthday, a membership's last day.

Not valid, so `time_zone` stands until `why` says one of the above: "the
server runs in one zone anyway", which is the implementation's zone the
owner's rule removes; "the column is `TIMESTAMP`", which is a rendering,
not a meaning; and a past event, which happened at an instant and is
written as one. The validator reads none of this: `why` is prose, and
its presence is what the rule checks; its content is the reviewer's.

### 12.3 Where a date meets the clock

A date and a wall-clock reading become instants only in the
specification's `timeZone`, never in the implementation's, and the
validator reports the places that need it:

| Where | Meaning in `timeZone` | Without `timeZone` |
|---|---|---|
| `at` with a `date` (3.8) | the first instant of that day | `time_zone`, an error at the transition: the key is new |
| `validity` whose `until` or `from` is a `date` | expired from the first instant of the day after `until`; valid from the first instant of `from` | `time_zone`, a warning at the validity: a 1.0 file may hold it |
| `date(x)` of an instant in an expression | the calendar date of the instant in the zone | `time_zone`, a warning at the expression, for the same reason; the date is UTC's until the zone is declared |
| a `local-date-time` or a `time` the implementation compares with the clock | the instant the zone gives that reading | nothing: the design has no expression that reads the clock, so the comparison is the implementation's, and the specification's `timeZone` is the zone it uses |
| a `date-time` anywhere | an instant; a zone is read only to show it to a person | nothing |
| a job's `schedule` | the five fields of cron, in UTC, as today (12.4) | nothing |

`date()` of an instant is evaluated by the validator, over an
algorithm's worked examples and a test's records, and by any generator
that renders it, which none does today. With `timeZone` both read the
date in it; without one both read it in UTC, the one zone that is no
host's and the reading the instant's own text has once its offset is
not kept (12.2): the warning at the expression says that the calendar
date is UTC's until the zone is declared. Today the Go build takes the
date at the offset the literal was written with and the Swift build the
date in UTC, so the two builds disagree on an instant written with an
offset other than `Z` whose date differs in UTC, against the rule that
both print the same line; the build item makes both read the rule.

The first instant of a day is the zone's: a day whose midnight the zone
skips begins at the first instant that exists, as the database gives it.
Nothing else requires a `timeZone`: a specification whose dates never
meet the clock declares none and gets no line. One zone, not one per
entity, since a specification describes one system and its people's one
business day; a record that belongs to a place of its own holds a
`time-zone` field beside its wall-clock reading, and reading it instead
of the root is a later item if a specification asks.

### 12.4 The jobs' schedule: a proposal, not changed here

A job's `schedule` is the five fields of cron in UTC
(`docs/conventions.md`, Jobs and menus): explicit already, and kept. The
proposal for the item that takes it up: the `trigger` gains `timeZone`,
an IANA name in the same form as the root key, and a `schedule` without
one stays UTC, so that no job written today changes its meaning; a job
whose hour is a business hour of the specification's zone writes that
zone's name on the trigger, the same name as the root, rather than a
reference to the root, since a system that spans zones has jobs in
several and a word such as `design` would be a second spelling of one
name. The root `timeZone` is not read by a schedule by default because
that would move every schedule of a specification the day it declares
its zone, silently, and because a cron hour in a zone with clock changes
does not exist once a year and exists twice once a year, which
schedulers handle differently; UTC has neither, which is why it is the
rule today, and a zone on a schedule is chosen on purpose, with that in
view.

### 12.5 What it means for what exists, and the version

Every `date-time` literal in an expression written today carries its
offset, since the validator refuses one without; a `default` or
`examples` value of a `date-time` field is not checked today, so one
written without its offset gains the warning `time_zone` when the rule
is built, and none in this repository's specification, its examples or
its conformance cases lacks it. No file written today
holds `local-date-time`, `time-zone` or `timeZone`, since none existed.
The `time_zone` warning fires on a 1.0 file at two places only, a
`validity` bounded by a `date` and a `date()` of an instant: on this
repository's specification nowhere; on the library lending example at
the Loan's check `lentOn <= date(loanedAt)` and at the Member's validity
from `joinedOn` to `membershipEndsOn`, so the build item declares the
example's `timeZone` with the rule, the zone the owner names (the
example names no place, so `UTC` is proposed), and no line appears;
among the conformance cases, `validate-validity` and
`validate-concept-cases-listed` gain one warning each at a validity
bounded by a date, which their expected output records. `date()` of an
instant reads UTC without a `timeZone` and the zone with one (12.3),
where the Go build today reads the offset the instant was written with
and the Swift build UTC: a worked example or a test record over an
instant written with an offset other than `Z` whose date differs in UTC
may change its outcome under the Go build when the rule is built, and
under both when the specification declares a zone other than UTC; none
in this repository does, since the library lending example's instants
are all written in `Z` and no other expression of the specification,
the examples or the conformance cases reads `date()` of an instant.
Every other change is an addition: one root key, two formats, `why` on a field, one
rule with errors only where a new key is written and warnings where a
1.0 file may be. The step is the minor one the machines take, 1.3.0 (7),
one release for both, since both rest on `timeZone`. The implementation
files and the type-rendering idiom gain rows for `local-date-time` and
`time-zone` per stack, a 1.x of the idiom; `specarch upgrade` has
nothing to move.
