# What SpecArch needs to define a user interface

A system analyst who writes a SpecArch specification can define the data,
the operations, the permissions and the tests of an application, and the
pages it shows: a list, a form or a view of one entity, its route, its
permission, the operation it reads or submits, its columns or fields, its
filters and its actions, and the menu that opens it. That is enough to
say what a screen holds. It is not enough to say how a person moves
through the application, what a screen shows while it waits or when it
fails, how it looks, how it changes on a small screen, or whether a person
who cannot see it or cannot use a pointer can use it. Each of those is
decided today by whoever builds the screen, and each is where screens of
one application drift apart.

This study sets out what an analyst should be able to write about a user
interface, in the design and stack-neutral, and what a generator needs
beside it in the implementation file. Its findings, in short:

1. Keep the design about structure, behaviour and rules, never about
   geometry or a component library. A page says what it shows, in what
   order and groups, what a person can do, what it shows in each state and
   where each event leads. Pixels, breakpoints, components and animations
   belong to a stack.
2. Add five things to the design: the states of a page (loading, empty,
   failed, and the problem types of what it calls), the events of a page
   and where each leads (a screen flow, after OMG's IFML), sections that
   group and order a page's fields, the conformance target and rules of
   accessibility (WCAG 2.2), and what a page keeps on a compact screen.
3. Put the visual design in design tokens, in the format of the W3C
   Design Tokens Community Group, as a design file of its own. Tokens are
   stack-neutral values the analyst or a designer owns; how each becomes a
   colour asset or a CSS variable is an idiom.
4. Draw what is defined: a screen-flow diagram and a per-page structure in
   the techspec, so the owner reviews the interface without running it.
5. Build one UI generator first, SwiftUI, against a real hand-built screen,
   and the plain JavaScript web generator after it.

Decisions D1 to D9 at the end are for the owner to confirm or veto before
the keywords are built.

## What exists

| Keyword | What it says | Checked |
|---|---|---|
| `pages` | `kind` (list, form, view), `title`, `route`, `entity`, `permission`, `source` (the operation a list or view reads), `submit` (the operation a form calls), `columns`, `fields`, `filters`, `actions` (each a `label`, `kind` operation or navigate, `target`, `permission`, `confirm`) | targets exist, fields are the entity's, permissions exist and are fail-closed |
| `menus` | a tree of entries, each a `title` with a `page` or `items` | every page exists; an entry shows to who may open its page |
| design tests | a page is a test subject, with route parameters and an action as its input | the action and the parameters exist |
| techspec | a flowchart of pages, their navigate actions and the operations they call, and a table per page | |

The roadmap names two UI generators, SwiftUI for the iPhone and plain
JavaScript for the web, and `docs/generators.md` sets the rule for them:
an emitter is written against one hand-built screen in a real project and
has to reproduce it before it is accepted. None is built.

## Where the standards and the platforms stand

Five sources settle most of what follows. Each was chosen because the
people who build interfaces already work to it, so a keyword that follows
it costs the reader nothing new.

**WCAG 2.2** (W3C Recommendation, October 2023). The conformance target
of nearly every accessibility law and procurement rule, written as
testable success criteria at levels A, AA and AAA. Level AA is what laws
ask for. Several criteria are about the design rather than the build, and
those are the ones a specification can check: text contrast of at least
4.5:1, and 3:1 for large text and for the parts of a control (1.4.3,
1.4.11); a label for every input (3.3.2) and a name for every control
(4.1.2); an error identified in text and a suggestion when one is known
(3.3.1, 3.3.3); a consistent order of navigation on every page (3.2.3); a
target of at least 24 by 24 CSS pixels (2.5.8, new in 2.2); no reliance on
colour alone (1.4.1); and a status message announced without moving focus
(4.1.3). Why it is the way it is: the criteria are written to be testable
by a person or a tool, so that "accessible" is a result and not an
intention. SpecArch takes the level as a target and checks what it can
check from the design.

**IFML** (OMG Interaction Flow Modeling Language 1.0, 2015). A standard
for the front end's flow: view containers (screens), view components
(lists, forms, details), the events a person or the system raises on
them, and the navigation flows that each event follows, with the
parameters it carries. It was made by people who had generated web
front ends from models for fifteen years (WebML), and it keeps what
proved useful: events and flows, not layout. Its own notation is a
diagram language, which SpecArch does not take; it takes the concepts,
written as YAML on the page that raises the event.

**Statecharts** (Harel, 1987, and W3C SCXML, 2015). A screen's states and
the events that move it between them. SpecArch already writes an entity's
states this way. A page's states are few and fixed (loading, content,
empty, failed), so SpecArch names them rather than letting each page
declare a machine.

**Design Tokens Format Module** (W3C Design Tokens Community Group,
first stable version 2025.10). A JSON format for the named values of a
visual design: colours, dimensions, font families and weights,
typography, shadows, durations, each with a `$type` and a `$value`, and
aliases to other tokens. Design tools and the token build tools that
turn tokens into CSS variables, iOS colour assets and Android resources
read it. Why it is the way it is: one source of truth for values that a
design tool and several code bases all need, which is exactly SpecArch's
split between design and stack. SpecArch takes the format with its `$`
keys, as it takes JSON Schema's, because a reader who knows the format
knows them.

**The platforms' own guidelines.** Apple's Human Interface Guidelines
(size classes compact and regular, a hit target of 44 by 44 points,
navigation by stack, split view or tab bar) and Material Design 3 (window
size classes compact, medium and expanded at 600 and 840 density
independent pixels). Both say the same thing a design needs to know:
what to keep and what to move when the window is compact. Neither belongs
in the design: the number of points and the kind of navigation are a
stack's.

ISO 9241-110:2020 (the interaction principles) and ISO 9241-210:2019
(human-centred design) are the standards behind the five above. They ask
that a system be self-descriptive, conform to the user's expectations and
tolerate errors; the states and the error messages below are how a
specification says that for each page.

## What the analyst writes

### States

Every page that reads or submits something is in one of these states:
loading, content, empty (a list with no records, or a list with no
records under the filters given), and failed. A failed page has failed
with a problem type the operation answers, and the catalogue under
`errors` already names them. So a page says what it shows in each:

    loans-list:
      kind: list
      ...
      states:
        empty: { message: No loans yet. Lend a copy from a member's page. }
        filteredEmpty: { message: No loans match these filters., actions: [clear-filters] }
        failed:
          default: { message: The loans cannot be shown right now. Try again in a moment., actions: [retry] }
          loan-closed: { message: This loan is already closed. }

Loading has no text to write, and a generator draws it the stack's way.
A form adds `submitting`, and its failed states are per problem type of
its `submit` operation, with `field` naming the field a problem is about,
so the message shows beside it (WCAG 3.3.1). Checks: every problem type a
page's operation answers is named under `failed` or covered by `default`;
a list has `empty`; a list with filters has `filteredEmpty`; a message is
a full sentence (`state`). Derived tests gain a case per state: `empty
<page> shows its message`, and a red case per problem type.

### Events and screen flows

A page raises events: the actions it has today, a form's submission that
succeeded or failed, a selected row of a list. Each event leads
somewhere: to another page with the parameters it carries, back, to the
same page with a message, or to an operation. Today only an action of
kind navigate says where it leads. The design adds `then` to an action
and `onSubmitted` to a form:

    member-form:
      kind: form
      submit: createMember
      onSubmitted: { navigate: member-view, with: { memberId: id }, message: Member registered. }
    members-list:
      kind: list
      onSelect: { navigate: member-view, with: { memberId: id } }

`with` maps the target's route parameters to the fields of the response
or the selected row, and each is checked (`flow`). An action of kind
operation gains `then` with the same shape. A message after an event is a
status message, announced without moving focus (WCAG 4.1.3).

A flow is a task a person does across pages, named once, so that the
documents can draw it and a test can walk it:

    flows:
      lend-a-copy:
        description: A librarian lends a copy to a member who is at the desk.
        actor: librarian
        steps:
          - { page: members-list, event: select }
          - { page: member-view, event: "action: Lend a book" }
          - { page: loan-form, event: submitted }
        satisfies: [LIB-7]

Each step must be reachable from the one before by an event the page
declares (`flow`). The techspec draws each flow as a Mermaid flowchart
of its steps, and the screen-flow diagram that exists grows to every
event. A flow is a test subject: its golden case walks the steps, and its
red cases are the failed states on the way.

### Sections and order

A form or a view with more than a handful of fields groups them. A
section has a title and its fields, in the order a person reads them, and
the order is the focus order (WCAG 2.4.3):

    member-view:
      sections:
        - { title: Member, fields: [cardNumber, fullName, email, tier] }
        - { title: Membership, fields: [joinedOn, membershipEndsOn, outstandingFees] }

`sections` replaces `fields` on the page, so there is one way to say it.
A page says nothing of columns, widths or positions: a section is a
group, and how groups sit on a screen is a stack's decision.

### Compact screens

What a page keeps when the window is compact is a design decision: a list
of loans on a phone shows the member and the due date and leaves the rest
for the view. The design says which columns stay, in order:

    loans-list:
      columns: [memberId, bookId, loanedAt, dueOn, status, lateFee]
      compactColumns: [memberId, dueOn, status]

The columns must be the page's own (`page`). What compact means in points
or pixels, and whether the columns become a stacked row, is the stack's.
SpecArch names one class, compact, because both platforms agree on it and
a design that names three classes would be negotiating with the stack.

### Accessibility

The root of the specification names the target once:

    accessibility: { standard: WCAG 2.2, level: AA }

With a target, the validator checks what the design decides (`accessibility`):

- every field shown on a page has a label: its `title`, or its name made
  readable when it has none, and a warning for a name that reads badly
  (`memberId` reads "Member id");
- every action has a label, and no two actions of a page share one, so a
  screen reader can tell them apart (4.1.2);
- the contrast of every pair of colour tokens the theme declares as text
  on a background is at least 4.5:1, and 3:1 for large text and control
  parts (1.4.3, 1.4.11), computed by WCAG's own formula for relative
  luminance;
- a state is never told by colour alone: a status column whose enum has a
  colour token per value also has its text (1.4.1);
- every failed state of a form names the field it is about when the
  problem is about one (3.3.1).

What it cannot check (a target size on the screen, the focus order as
built, the names a component library gives) is the generator's to honour
and a commissioning check to verify. The techspec lists, per criterion,
whether the design satisfies it, the generator must, or a person checks
it, so the owner sees what is left to test by hand.

### The visual design: tokens

The theme is a design file of its own, `design/theme.yaml`, in the
format of the Design Tokens Community Group, written as YAML:

    tokens:
      color:
        $type: color
        text: { $value: "#1b1f24" }
        background: { $value: "#ffffff" }
        accent: { $value: "#0b5cad" }
        danger: { $value: "#b3261e" }
      space:
        $type: dimension
        small: { $value: { value: 8, unit: px } }
    modes:
      dark:
        color.text: "#e6e8eb"
        color.background: "#101214"
    pairs:
      - { text: color.text, background: color.background }
      - { text: color.accent, background: color.background, size: large }

`modes` gives a token another value in a mode, light being the default.
`pairs` names which colours are used as text on which background, which
is what the contrast check reads. The design file holds values and names
only; a token is used by a stack through the token-rendering idiom (CSS
custom properties for the web, an asset catalogue and a `Color` extension
for SwiftUI). A specification without a theme is valid: the stack's own
look applies, and the contrast check has nothing to read.

What is left out on purpose: a typography scale beyond the format's own
`typography` type, motion, illustrations and icons as files. They are
added when a real specification needs them.

## What a generator needs beside the design

The implementation file says how a stack builds the interface, so the
design never names a component:

| Concern | Where | Example |
|---|---|---|
| the component library, and the component per page kind and per field type | target settings, and the ui-components idiom | SwiftUI `List` and `Form`; a project's own `DataTable` |
| the navigation container | target settings | a `NavigationStack` with a tab bar on compact, a split view on regular |
| how a token becomes code | the token-rendering idiom | a `Color` extension over an asset catalogue; CSS custom properties |
| the compact class in points or pixels | target settings, defaulting to the platform's | 600 density independent pixels |
| the event bus of the web | the generated code, per the roadmap | one module with every event name as a constant |
| strings and their translations | a strings file per stack, keyed by page and state | `Localizable.strings` |

Each is an idiom or a setting, so a project that renders its screens from
its own schema layer, as `docs/generators.md` asks, overrides the
component part and keeps the rest.

## The documents

The techspec's pages part grows to:

- the screen-flow diagram with every event: navigation, submission,
  selection, and the operations called;
- a Mermaid flowchart per flow, its steps and the failed states on the
  way;
- per page, its sections in order, its states with their messages, and
  what it keeps on a compact screen;
- the theme's tokens with their values per mode, and each text pair's
  contrast ratio;
- the accessibility table: each WCAG 2.2 criterion the design decides,
  and whether it is satisfied, the generator's, or a person's to check.

A wireframe drawn as an image is left out: the structure table says the
same in words an owner can review, and a drawing invites a review of
pixels the design does not fix.

## Which generator first

| | SwiftUI | Plain JavaScript web |
|---|---|---|
| a real hand-built screen to reproduce | several, in the owner's iPhone and Mac apps | none hand-built in plain JavaScript yet |
| what the generated screen depends on | the SDK only | nothing: ES modules, no build step |
| how it is checked | `swift build` and a preview, on the Mac that builds SpecArch's Swift validator | a browser test runner |
| reach | iPhone, iPad and Mac from one code base | every browser |

SwiftUI first. `docs/generators.md` accepts an emitter only once it
reproduces a hand-built screen, and SwiftUI is where such screens exist
today; it also covers the Mac and the phone at once. The web generator
follows, against the first web screen built by hand under the roadmap's
plain JavaScript rules. The pick is the owner's (D9); nothing is built
before it.

## Decisions for the owner

| | Decision | Recommendation |
|---|---|---|
| D1 | The design says structure, behaviour and rules; geometry, components and animation are a stack's | confirm |
| D2 | A page's states are the named set loading, content, empty, filtered empty, submitting and failed, with failed per problem type; a page does not declare its own state machine | confirm |
| D3 | Events and flows after IFML: `onSubmitted`, `onSelect` and an action's `then`, each with `navigate`, `with` and `message`; `flows` as named tasks across pages, each a test subject | confirm |
| D4 | `sections` replaces `fields` on a form or view when fields are grouped, and its order is the focus order | confirm |
| D5 | One screen class, compact, with `compactColumns`; the stack sets its size | confirm |
| D6 | `accessibility: { standard: WCAG 2.2, level: AA }` at the root turns on the checks listed above; without it they are warnings | confirm |
| D7 | The theme is `design/theme.yaml` in the Design Tokens Community Group format, `$type` and `$value` kept, with `modes` and `pairs` as SpecArch's | confirm |
| D8 | No wireframe images in the documents; the structure table and the diagrams instead | confirm |
| D9 | The first UI generator is SwiftUI, against a hand-built screen of an existing app; the web one follows | the owner picks |

## Implementation items, in order

1. The keywords of D2 to D7 in the schema, `docs/conventions.md` and the
   rule enum (`state`, `flow`, `accessibility`, `theme`), spec first, with
   an ADR per decision.
2. The validator rules in Go and Swift, with a conformance case per rule,
   printing the same output.
3. The derived cases: per state, per problem type of a page, and per flow.
4. The techspec: the screen-flow diagram with every event, the flows, the
   page structure, the theme and the accessibility table.
5. The library lending example: states on every page, a flow, sections
   on the member view, compact columns on the loans list, a theme and the
   accessibility target.
6. After the owner's pick, the first UI generator, written against the
   hand-built screen it must reproduce.

## Left out

The owner's applications are referred to only as the place hand-built
SwiftUI screens exist; none is named, and nothing of them is in this
repository.
