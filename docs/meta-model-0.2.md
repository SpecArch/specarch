# Meta-model 0.2

Meta-model 0.2 adds what extracting real systems and the first requests for
generated screens found missing: permissions one holder must never have
together, approval workflows, pages that are not about an entity and the
page elements every back-office screen uses, records that are passed but
not stored, and a few smaller keywords. It ends with the one change that
can break a file written for 0.1: a derived case no test covers becomes an
error. The steps below build it in order, each with what done looks like.
`docs/roadmap.md`, section 5, keeps the list of candidates; this file is
the plan for the ones taken.

## What makes it 0.2

A keyword added to the meta-model has always been an addition no existing
file breaks on, and the meta-model stayed at 0.1 through views, jobs,
guards, sessions and the UI keywords. 0.2 follows the same rule: every new
keyword in steps 1 to 10 is an addition to the one design schema, valid in
a file that says `specarch: "0.1"`. The version moves only with step 11,
the rule that turns a missing test into an error, because that rule is the
one that makes a valid 0.1 file invalid (ADR-055).

The bump comes last so that the specifications in this repository, both
examples and the trees `specarch extract` writes are migrated once, after
every new keyword has brought its derived cases, rather than once per
step.

## The boundary with the UI generators

0.2 owns what a specification can say about a page: the keywords, the
validator's rules, the derived cases and what the technical specification
draws. A UI generator owns how a stack draws it. A page element the web or
iPhone generator needs is a 0.2 step when it is a word the design does not
have yet; its rendering is the generator's step, which waits on the 0.2
step that gives it the word.

## Taken in 0.2

### Separation of duties

Sets of permissions one holder must never have together, after the static
separation of duty of the ANSI RBAC standard (INCITS 359), which states it
as a set and a cardinality: no one holds that many of the set at once.

    separationOfDuties:
      lend-and-write-off:
        description: Whoever lends a copy must not be able to write the loss off.
        permissions: [loans.create, loans.writeoff]
        cardinality: 2

`cardinality` is the number of the set's permissions one holder may not
reach, at least 2 and 2 when left out. The validator checks that every
permission named is declared and is not `public`, and that no role grants
`cardinality` or more of the set (`separation_of_duties`).

What the validator cannot see is one person holding two roles that each
grant part of a set: which person holds which role is data, not design.
The technical specification lists, for every set, the combinations of
roles that together reach its cardinality, as roles that must never be
given to one person, so that the rule is written where a role assignment
is reviewed. A workflow's approval step (below) uses the same sets: when
the trigger's permission and the approval's permission are in one set, no
role may grant both.

### Workflows

A workflow is a request that finishes later, after people approve it, as
its own object. It is named `workflows`, because `flows` already names a
person's navigation across pages (ADR-034). The steps take their meaning
from BPMN 2.0, a subset of it: an approval step is a user task with its
potential owners, an operation step is a service task, a deadline is a
timer on the user task, and the steps run in order, with an approval's
refusal ending the request.

    workflows:
      fee-waiver:
        description: A late fee above the desk's limit is waived only after a second person approves.
        trigger: requestFeeWaiver
        subject: FeeWaiverRequest
        steps:
          - name: approve
            kind: approval
            approvers: [desk-supervisor]
            permission: fees.approve
            deadline: P3D
            onDeadline: refuse
          - name: waive
            kind: operation
            operation: waiveFee
        satisfies: [LIB-8]

- `trigger` is the operation that starts it. Its request body is the
  workflow's form, so there is no second schema for it, and it must answer
  202: the request is accepted and pending, not done.
- `subject` is the entity that holds the request while it waits, so a
  page can show where it is and an approver's list can read it.
- An approval step names the roles that may approve, the permission it
  checks (granted by each of those roles), its deadline as an ISO 8601
  duration that is never zero, and what the deadline does: `refuse` ends
  the request, or `escalate` names a later approval step.
- An operation step names an operation the system calls once every
  approval before it has passed.
- The person who made the request never approves it: the four-eyes rule
  is part of every approval step and has no switch. It is checked when the
  workflow runs, and the derived case `requester approves own request` is
  critical whatever the requirement's harm.

The validator checks that the trigger exists and declares a 202 response,
the subject is an entity, every role and permission is declared, each
approval's roles grant its permission, the trigger's permission is not the
approval's, no role grants both when a separation-of-duties set holds the
pair, every operation step names an operation, an escalation names a later
approval, and the step names are unique (`workflow`). The derived cases
are the approved path (golden), a refusal at each approval, each deadline
passing, the requester approving their own request, and someone without
the approval's permission approving. The technical specification draws
each workflow as a Mermaid flowchart of its steps, with the deadline on
the edge it takes.

Parallel approvals, a number of approvals out of a pool, and loops are
left out of the subset until a real workflow asks for one, and a workflow
that needs them is written with a question.

### Pages that are not about an entity

A task page submits to an operation without loading a record: sign-in, a
second factor, a password reset, a confirmation of an e-mail address.

    sign-in:
      kind: task
      title: Sign in
      route: /sign-in
      permission: public
      submit: signIn
      fields: [email, password]
      onSubmitted:
        "200": { navigate: home }
        "202": { navigate: second-factor, with: { challengeId: challengeId } }

A task page has no `entity` and no `source`. Its fields are properties of
the submit operation's request body, and every property the body requires
is one of them, since the request could not succeed otherwise.
`onSubmitted` is keyed by the response status the page acts on, each a
status the operation declares, and `with` maps the target's route
parameters to properties of that response's body (`page`, `flow`).
Validation shown to the person comes from the body's schema, so a password
rule is written once, in the operation, and the screen shows the rule the
server applies. The derived cases are one per declared answer: golden for
a success, red for each problem type, and the session's expiry does not
apply to a public page.

### Page elements

The elements every back-office screen uses and the design cannot say yet:

- a lookup field: a field that holds the key of another entity's record
  through a relation, picked from a list read from a list operation, with
  the fields of the record it shows and the fields of the form it fills
  from the chosen record;
- a row action available only in some states: `when`, an expression over
  the row in the expression subset, such as `status == "active"`;
- a confirmation that asks for a reason, sent with the request as a named
  property of its body;
- a field read-only or hidden by mode (create, edit, view) or by an
  expression over the record;
- a check across a form's fields, an expression over them with its
  message, and a field entered twice to confirm it and never sent;
- child rows: a form edits the records of a relation inline, with the
  fields of each row, a maximum number of rows, and whether loaded rows
  are locked so only new ones change.

Each has its validator rule (the relation, list operation, fields and
expressions resolve and type-check), its derived cases (a lookup that
finds nothing, an action offered in a state its `when` excludes, a missing
reason, a violated cross-field check, one row over the maximum), and its
place in the technical specification's table per page.

### Maker-checker on a page

A form or task page whose submit operation is a workflow's trigger acts on
the 202 answer: `onSubmitted` for it says where the page leads and what it
says ("Sent for approval."), and the page gains the state `pending`. A
list page may name the workflow and approval step it is the inbox of; its
source lists the workflow's subject, and its permission must be the step's
permission. A workflow may name the channel message it publishes when it
finishes, so a page waiting on it knows how the end is announced. Derived
cases: the pending message is shown, and the inbox lists nothing to
someone without the step's permission.

### Value objects

Data that is passed around but not stored and has no identity (a
diagnostic, a request summary, a token's claims as a body) is written
today as an entity with a made-up key. `schemas` holds a named object
schema with no key and no table, taken with its name from OpenAPI's
`components.schemas`; a request body, a response, a message or another
schema may refer to it, an entity may not relate to it, and the SQL
generator writes nothing for it. `extract openapi` then writes a component
schema with no key as one, instead of an entity with a question about its
key.

### Smaller keywords

- A unique constraint that holds only where a condition holds: `where`, an
  expression over the entity's fields, as a partial unique index. `generate
  sql` writes it on engines that have one (PostgreSQL, SQL Server) and
  refuses on those that have not, rather than dropping it.
- An operation, page or command served only when a setting is on:
  `enabledBy`, naming a boolean setting of `configuration`. The derived case
  `disabled by <setting>` checks that it is refused with the setting off.
- A requirement's target release: `release`, a version that a release record
  exists for or is planned, so a scope split into a first and a later
  release is written in the requirement and the test plan can group by it.

### A name on the wire

A property's name must be camelCase, and an API whose wire names are
snake_case cannot be specified without renaming its contract. One rule per
specification says it: `info.wireNames: snake_case`, applied to every
property of every body, parameter and message, with the validator refusing
two properties that map to one wire name. One rule rather than a name per
property, because an API names its properties one way throughout, and a
name per property would be a second way to say the same thing on every
field.

### A schema per fragment file

The tree's files each hold one stage's sections, and an editor validating
one file against the whole-specification schema reports what the other
files hold as missing. A schema per stage, written from the one design
schema by a script and checked current in CI, gives each fragment's
`$schema` line something to point at without a second definition to
maintain by hand.

## Left out of 0.2

| Candidate | Why it waits |
|---|---|
| `guard` | Built. The only open point, a postcondition, is checked by the guarded operational scripts after the change (`docs/generators.md`), so it is the scripts generator's work, which no longer waits on the meta-model. |
| row-level permissions, gates by client identity or a signed header, token formats | Each needs the caller as something an expression can name: who is asking, through which client, with which claims. They are designed together in 0.3, around that one concept. The four-eyes rule of a workflow needs no expression, so it does not wait. |
| golden tests of a designed but unbuilt command | Already settled: each extract step brings its own golden tests, `extract-not-offered` covers the sources not read yet, and `spec/` validates with no warning. |
| a fixed expression grammar | The expression subset is fixed by its table in `docs/conventions.md` and by the conformance cases both builds pass. A grammar written beside them would be a second definition to keep equal, and the conformance suite is the one both builds are held to. |
| interfaces beyond HTTP, messaging and the command line | No project being specified uses gRPC, file exchange or a serial protocol. The menu-bar application on the roadmap will show what a menu-bar interface needs when it is extracted. |
| sets and lists in the expression language | No rule asks for one yet. Separation of duties is a validator check, not a formula. |
| configuration as a first-class concept | Settings are built (`configuration`); what was missing, an element served only behind a setting, is `enabledBy` above. Seed data per profile waits for the permissions reader to show how seeds are read. |
| data held in a cache with an expiry, and tables kept but unused | Where data is held is an implementation choice; an unused table is a mapping marked `ownedBy` or a question. |
| more than one deliverable per specification | Which repository builds an element belongs to the implementation files; it waits for a second front end to be specified. |

## Building 0.2

Each step is one item, built spec first in both validator builds where
the validator changes, with conformance cases, the techspec regenerated,
and a history entry. Steps 1 to 10 are additions to the 0.1 schema.

1. Separation of duties. `separationOfDuties` in the design schema, the
   rule `separation_of_duties` in both builds, and the techspec's table of
   sets with the role combinations that reach each. The library-lending
   example gains a write-off permission and one set. Done when a role
   granting two permissions of a set, a set naming an undeclared
   permission, a set naming `public` and a cardinality below 2 are each
   refused with the same output in both builds (red), the example
   validates with its set (golden), and the techspec lists the set.
2. Task pages. `kind: task`, `onSubmitted` keyed by status, the `page` and
   `flow` rules extended in both builds, derived cases per answer. The
   library-lending example gains a sign-in page. Done when a task page
   that names an entity, leaves out a required body property, or keys
   `onSubmitted` by a status its operation does not declare is refused in
   both builds (red), the example's sign-in page validates with its
   derived cases (golden), and the techspec draws it in the screen flow.
   Item 052's task-form pages build on this step.
3. Workflows. `workflows`, the rule `workflow` in both builds, the derived
   cases and the techspec's flowchart. The library-lending example gains
   the fee waiver. Done when each check listed under Workflows has a red
   case with the same output in both builds, the fee waiver validates
   (golden), `specarch derive` writes its five derived cases, and the
   techspec draws it. Item 052's maker-checker waits on this step and
   step 4.
4. Maker-checker on a page. The 202 answer on a form or task page, the
   state `pending`, the inbox list, and a workflow's completion message.
   Done when a page submitting to a trigger with no event for 202, and an
   inbox whose permission is not its step's, are refused in both builds
   (red), and the example's fee waiver has a request page and an inbox
   (golden).
5. Page elements: lookup fields, `when` on row actions, a confirmation with
   a reason, a field's mode and condition, and checks across a form's
   fields. Done when each has a red case for what does not resolve or
   type-check, with the same output in both builds, the library-lending
   loan form picks its member and book by lookup, the members list offers
   Deactivate only to an active member, and the derived cases are written.
   Item 052's lookup, row-action, confirmation and cross-field steps build
   on this step.
6. Child rows. Done when a form with child rows of a relation, a maximum and
   locked loaded rows validates (golden), a child row over an entity the
   relation does not reach is refused (red) in both builds, and the derived
   case for one row over the maximum is written. Item 052's child-row step
   builds on this step.
7. Value objects, `schemas`, and `extract openapi` writing a component
   with no key as one. Done when a body and a message referring to a
   schema validate, an entity relating to one is refused (red), `generate
   sql` writes nothing for it, and the lending desk's OpenAPI tree, with a
   component given no key, writes a schema and asks no key question.
8. The smaller keywords: `where` on a unique constraint, `enabledBy`, and a
   requirement's `release`. Done when each has its red case in both builds,
   `generate sql` writes the partial unique index in PostgreSQL and SQL
   Server and refuses it in Oracle and MariaDB, and the test plan groups
   the requirements by release.
9. Built. A name on the wire, `info.wireNames: snake_case` (ADR-062): one
   mapping in each build, applied by the validator, `extract openapi`,
   `merge` and `generate openapi`. Two properties of one object that map to
   one wire name are refused as `wire_name` in both builds. The lending
   desk's OpenAPI document names its properties in snake_case and extracts
   with no line about a name, its tree and the merged specification carry
   the rule, and `generate openapi` on the lending desk writes the
   document's names back; CI checks both.
10. A schema per fragment file, written from the design schema by a script,
    with a CI check that it is current. Done when every fragment in `spec/`
    and both examples validates in an editor against its stage's schema
    and CI fails when the design schema changes without the fragments'
    schemas.
11. Meta-model 0.2. The design schema becomes `specarch-design-0.2`, with
    `specarch` accepting `"0.1"` and `"0.2"`. In a 0.2 file a derived case
    no test covers is an error, except for a subject an open must or should
    question holds up, which stays a warning because `specarch derive`
    does not write its test either; in a 0.1 file it stays a warning, and
    the file gets one warning that 0.1 is read until 1.0. `specarch extract`
    and `merge` write 0.2 trees and run the derivation into them, so their
    output carries the tests derive writes. `spec/`, both examples and the
    lending desk's extraction script move to 0.2. Done when a 0.2 file
    missing a test is refused and the same file as 0.1 warns, in both builds
    with the same output; every tree in the repository validates as 0.2
    with no errors; and the extract script's output is still byte-identical
    on two runs.
12. Built. The workflows reader, extract step 11: `specarch extract workflows` for
    BPMN 2.0 XML, read with the standard library, writing the user tasks,
    service tasks and timers of the subset as `workflows`, and anything
    outside it as a line and a question. It needs step 3 and extract step
    10. Done when a BPMN file added to the lending desk with one approval,
    a deadline and a service task gives one workflow, validated and
    byte-identical, and merge joins its trigger to the router's operation.
    The file names an operation without declaring it, so the reader
    leaves the key out with a must question whose `names` carries the
    name, and merge writes it at the key once a tree declares it.

## How 0.2 meets the extract steps

- Extract 8, permissions: unchanged; a separation-of-duties set is design
  and is not read from seeds. Once step 1 lands, a seed that grants one
  role two permissions of a set is a validator error on the merged tree.
- Extract 9, pages: a page folder whose schema submits to an operation
  without loading a record is written as a question until step 2 lands;
  step 2's item then lets the reader write it as a task page, if extract 9
  is in by then.
- Extract 10, the lending desk end to end: runs on 0.1; step 11 migrates
  its script and output.
- The workflows reader, which `docs/extraction.md` leaves out, is step 12,
  after step 3 and extract 10.
