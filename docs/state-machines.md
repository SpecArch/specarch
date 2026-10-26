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
ADR-189 to ADR-192 hold the choices with `status: proposed`, and the owner
is asked to approve or change them before anything is built.

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
  duration, or `at` a moment), under what condition it fires (`when`, an
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
- Everything is additive: a 1.0 file validates as before, with no new
  output on this repository's specification and examples, so the step is
  a minor one, 1.3.0.

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
| time event | `after(duration)`, `at(time)` | `<send delay>` on entry, cancelled on exit | `after`, an ISO 8601 duration since the state was entered; `at`, an expression giving a timestamp or a date | follows UML's two words; the duration is SpecArch's own `duration` (days, hours, minutes, seconds), not SCXML's CSS2 form |
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
| conflicting transitions | 14.2.3.9.3: transitions in conflict; priority to the deeper source; otherwise undefined | 3.13: the deeper source wins, then document order | an inner state's transition wins over an ancestor's on the same cause; two from one state on one cause are an error when they cannot be told apart and a warning when the validator cannot show their conditions exclusive | follows both on depth; departs from SCXML's order and UML's undefined, since SpecArch does not pick a meaning for an ambiguous entry (`docs/principles.md`) |
| run to completion | 14.2.3.9.1 | the macrostep | the order exit, set, entry, within the operation or job that causes the move | follows both |
| choice, junction, fork, join | pseudostates | none (conditions and `<parallel>`) | none: `when` and `otherwise` on the transitions out of a state say what a choice says | departs from UML's vertices, as SCXML does: every decision is on a transition, so each has a cause and a condition the validator reads |
| deferred events | a State's deferrableTrigger | none | none | departs: an event not handled in a state is refused by the operation, `from wrong state`, which the test plan already derives; queuing it is a stack's choice |
| internal and local transitions | transition kinds | `type="internal"` | a transition with `from` and `to` the same state runs exit and entry (external); no internal transition | departs for now: an internal transition is a `set` without a move, which an operation's `writes` says; open question 10.5 |
| submachine state | a State referring to another StateMachine | `<invoke>` | none | departs: a machine named twice is written twice; nothing has asked for it |

## 3. The meta-model

### 3.1 Two spellings of a machine

The short form is today's: an entity's `stateField` and `transitions`.
It is read as one machine named after the entity, whose control state is
the field, whose extended state is the entity's other fields, whose
initial state is the field's `default` or, without one, the states no
transition reaches, and whose final states are the states no transition
leaves. Its transitions take every key of 3.4.

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
| `at` | an expression of type `timestamp` or `date` over the extended state, the moment the transition fires; a date fires when that day begins in the time zone the implementation runs in |
| `when` | an expression giving a bool over the extended state and, with a `trigger`, `event.<property>`; the transition fires only when it holds |
| `otherwise` | `true`: the transition fires when no other transition of the same `from` and cause has a `when` that holds; one per `from` and cause |
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
and `set` into what it writes.

Two transitions of one state on one cause are told apart by their
conditions. A transition of a descendant state wins over one of an
ancestor on the same cause (UML 14.2.3.9.4, SCXML 3.13), so those never
conflict. Among the transitions of one `from` on one cause (a trigger, an
`after` of one duration, an `at` of one expression):

- a transition with neither `when` nor `otherwise` must be the only one;
  otherwise `transition_ambiguous`, an error at each;
- two with the same `when`, or two `otherwise`, are `transition_ambiguous`;
- two with different `when` are exclusive when the validator can show it:
  one is the negation of the other (`!(a)` against `a`, or `a == x`
  against `a != x`), or each compares the same name with a literal and
  the two ranges do not meet (`n < 3` against `n >= 3`, `s == "a"` against
  `s == "b"`); any other pair is the warning `transition_overlap` at
  both, which says that the validator cannot show them exclusive and that
  writing one as `otherwise` settles it.

The rule is exact where it can be and honest where it cannot: deciding
overlap in general is a satisfiability question, and SpecArch would
rather warn than claim.

### 3.5 States

    states:
      starting:
        description: The job is being started; a failed start is retried.
        entry: { set: { lastError: null } }
      running:
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
from moving between its children. A state's `invariant` is checked with
the invariants of its ancestors; the validator reads it as an expression
and a generator may write it as a check constraint of the entity,
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
starts it at its initial state, and leaving them resets it. A transition
of JobLink whose `from` is its initial state is still reachable. The
diagram draws JobLink inside the composite that holds those states, and
the test plan takes JobRun's state as the given of JobLink's paths.
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
today, the states no transition reaches; when there are two or more, the
warning `state_initial` asks for the default. A full-form entity machine
whose field has no default is `state_initial`, an error, since the full
form promises the machine is complete. A machine with no entity writes
`initial`, and a composite state writes its own.

`final` lists the states the machine ends in. The short form keeps the
derivation of today, the states no transition leaves. In the full form a
state no transition leaves that `final` does not name is the warning
`state_dead_end`, which says to name it final or give it a way out; a
final state with a transition from it is the error `state_final`.

### 3.8 Time

`after` and `at` are the design's statement that time moves the machine;
which program watches the clock is the implementation's. For a stored
entity the implementation needs the instant the `from` state was entered,
which the entity does not declare: the build item designs it as an idiom
column beside the state field, as the audit fields are an idiom (open
question 10.3). `specarch-gen-scripts` writes no script for a time
transition and says so. The library lending example's nightly job emitting
`loan.overdue/LoanOverdue` is the shape `at: dueOn` replaces: the design
then says when a loan is overdue, and the job becomes the implementation
of the clock.

## 4. The validator

Every check runs in both builds and prints the same line
(`docs/conventions.md`, Two validator builds). The rules that exist keep
their names; the new ones are frozen at the release that ships them.

| Rule | Severity | When it fires, and where |
|---|---|---|
| `state_field` | error | as today; also a field that is the control state of two machines, at the second; a full-form entity machine whose `stateField` is not an enum field of its `entity` |
| `state_value` | error | as today: a `from`, `to`, `initial`, `final`, `within.states` or `states` key that is not a value of the machine's enum |
| `trigger` | error | as today: a trigger naming nothing |
| `state_machine` | error | `entity` names no entity; `stateEnum` names no enum; `within` names no machine of the same entity; `enter: history` to a state that is not composite; a `to` that is a composite state without `initial` |
| `state_initial` | error, warning | a composite state with `states` and no `initial`; a full-form entity machine whose field has no `default`; a standalone machine without `initial`: errors. A short-form entity with two or more states no transition reaches and no default: a warning naming them |
| `state_unreachable` | warning | a state that is not initial and that no transition reaches, nor is a descendant of one reached; in the short form a state no transition reaches is initial by derivation, so it never fires there |
| `state_dead_end` | warning | full form: a simple state no transition leaves, nor any ancestor of it, that `final` does not name |
| `state_final` | error | a transition from a state `final` names |
| `state_unused` | warning | a value of the machine's enum that no transition names and that is not initial: the requirement that asked for the value has no move to or from it |
| `transition_cause` | warning | a transition from a simple state with no `trigger`, `after` or `at` |
| `transition_ambiguous` | error | 3.4: two transitions of one `from` on one cause that cannot be told apart |
| `transition_overlap` | warning | 3.4: two conditions the validator cannot show exclusive |
| `transition_history` | error | `enter: history` to a state without `history` |
| `expression_syntax`, `expression_name`, `expression_type` | error | as today, for `when`, `set`, `invariant`, `at` and the `set` of `entry` and `exit`: an unknown variable or `event` property, a wrong type, a value outside the subset; `at` must give a timestamp or a date and `when` and `invariant` a bool |
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
machine `within` another is drawn inside the composite state that holds
the states it names.

| What | How it is drawn | Checked with Mermaid 11 (mermaid-cli 12.0.0) |
|---|---|---|
| a state | `not_started` | yes |
| the initial state | `[*] --> not_started` | yes |
| a final state | `stopped --> [*]` | yes |
| a transition | `starting --> running : job.started/JobStarted [retryCount < maxRetries] / retryCount = 0`, the cause, then `[when]` or `[otherwise]`, then `/ ` and each `set` as `name = expression`, comma separated; `after PT30S` and `at deadline` as the cause | yes |
| a composite state | `state running { [*] --> warming ... }`, nested as deep as the design | yes |
| history | the arrow to the composite state with `(history)` after its cause, `paused --> running : resumeJob (history)`; Mermaid has no history pseudostate, and an arrow from outside to a state drawn inside the composite pulls that state out of it, as the check showed | yes |
| parallel machines of one entity | `state Job { state JobRun {...} -- state JobLink {...} }` | yes |
| entry, exit, invariant | a note at the state: `note right of starting : entry / lastError = null`; a `state : text` line would replace the state's name with the text | notes yes; the replacement seen in the check |
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
- `specarch generate sql` changes nothing: the control field is a column
  as today, and an entry-time column is the build item's idiom (10.3).
- `specarch model` lists `stateMachines` with their references (entity,
  field, enum, within, triggers), so the editor shows them.
- `specarch gaps` counts a machine's transitions without a cause and
  states without a description with the other gaps.

## 7. Compatibility and the version

Every change is an addition: one optional root key, `stateMachines`;
five optional keys on a transition; nothing removed, renamed or made
required. A 1.0 file validates as before. On this repository's
specification and the examples the one entity with a state field, the
library lending example's Loan, has a trigger on every transition, one
derived initial state and no enum value outside its transitions, so no
new warning appears and no document changes; of the conformance
cases under `spec/tests/` that carry a state field, three would gain a
line, `document-testplan-state-machine` and `validate-derived-flow` the
warnings `state_unused` for `lost` and `transition_cause` for three
transitions, `validate-state-value` the warning `state_unused` for
`live`, which their expected output records when the rules are built.
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
          starting:
            entry: { set: { lastError: null } }
          running:
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
are left out here. What the validator says of it: nothing. What it would
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
          note right of starting : entry / lastError = null
          note right of running : invariant lastError == null
        }
        --
        state JobLink {
          [*] --> disconnected
          disconnected --> connected : link.opened/LinkOpened
          connected --> disconnected : link.closed/LinkClosed
        }
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

Here the first two `answerChallenge` conditions are shown exclusive
(`a == b` against `a != b`), and `otherwise` takes the rest; the two
`after` transitions are on different states and never meet.

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

Each is `status: proposed` until the owner approves; the build items
follow the approval, in the order of 11.

## 10. Open for the owner

1. The word for the condition: `when` (this design) or `guard`, UML's
   word, which SpecArch already uses for an operation's checked change.
2. The initial state of an entity machine from the field's `default`,
   with `initial` refused on it (this design), or `initial` on every
   machine with the default required to agree.
3. The entry-time column for `after` on a stored entity: an idiom column
   named `<stateField>EnteredAt`, set by every move, or a column the
   entity declares itself; and whether a `date` in `at` fires at the
   start of the day in the implementation's time zone.
4. A change event, UML's `when(expression)` with no trigger on a simple
   state, which needs the implementation to watch the expression: left
   out here as `transition_cause`; include it, or keep it out.
5. An internal transition, a `set` on a trigger with no move and no entry
   or exit: left out, since an operation's `writes` says it; include it
   as `to` left out, or keep it out.
6. A page event (`actions`, `onSubmit`) as a trigger of a standalone
   machine for a flow of screens: left for a later item.
7. The names of the new rules and keys, before the release freezes them.

## 11. Build plan, after approval

1. The schema (design and fragment design, 1.0), both validators with
   every rule of section 4 printing the same line, the conformance cases
   under `spec/tests/`, and the rule enum: one item, Go and Swift
   together, since a validator change goes into both.
2. The diagram and the technical specification, `specarch model`.
3. The test plan's paths and cases, `gaps`.
4. `specarch-gen-scripts`, and the entry-time idiom for `after` with
   `generate sql`.
5. The library lending example rewritten with `at: dueOn` where the
   owner agrees, as the acceptance test of the design.
6. The release, 1.3.0.
