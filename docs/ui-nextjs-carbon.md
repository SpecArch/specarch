# A second web generator: TypeScript, Next.js and Carbon

A project with many web screens already built on the Next.js app router
and IBM's Carbon design system asked SpecArch to generate its screens, so
that a generated screen looks and works like the ones built by hand. This
document is the design of that generator and the plan that builds it, in
steps. In short:

1. A second web generator, `specarch-gen-ui-typescript`, for a `ui` target
   of platform web and framework `nextjs-carbon` in an implementation file
   whose language is TypeScript. It writes, per page, a schema file that
   holds data only and a thin `page.tsx`, and per application the menu,
   the route guard, the strings, the server routes and the theme. The
   plain JavaScript generator stays as it is.
2. What draws each page is an idiom, `ui-components`, a new concern. The
   shipped idiom draws with plain Carbon, so the example builds with
   public packages only; a project renders through its own component
   library by overriding it, as any idiom is overridden today.
3. The design keywords the screens need and SpecArch lacks (a page with no
   entity, a field picked from another entity's records, child rows, a
   rule across fields, an action
   offered by the row's state, a confirmation with a reason, an approval)
   are meta-model work, planned in meta-model 0.2. This plan consumes
   them; each step says which one it waits for.
4. The first step to build is the idiom concern and the shipped
   `ui-components` idiom. Steps 2 to 7 follow the order the request asked
   for: sign-in first, then lists, forms, the rest, and the comparison
   with screens built by hand.

Questions Q1 to Q10 at the end were the owner's, who answered each as
recommended on 2026-10-09 (ADR-051). Steps 1 to 5 are built.

## Where SpecArch stands

The design already says what a screen holds and how it behaves:
`pages` of kind list, form and view with their route, entity, permission,
source, submit, columns, fields, filters and actions; `menus`; the
states, events, flows, sections, compact columns, accessibility target
and theme of `docs/ui-design.md` (ADR-034 to ADR-039). `specarch generate`
finds a plug-in per implementation file, `specarch-gen-<target>-<stack>`
before `specarch-gen-<target>`, the stack being the file's language in
lower case, and hands it the idioms that apply with any override, so a
plug-in reads no disk. `ownedBy` on a mapping (ADR-046) marks an element
another stakeholder owns, which every generator leaves out.

The one UI generator, `specarch-gen-ui`, writes list pages in plain
JavaScript and refuses any other framework (ADR-040). Nothing in it is
meant to grow a second framework: the TypeScript generator is a plug-in of
its own, and the two share only what the design says.

## The line between this plan and meta-model 0.2

Anything a design file says is meta-model work, planned and built under
meta-model 0.2. This plan owns the rest: the idiom schema, the
implementation schema, the generator and its output, and the example.
The request asks for these design keywords, each waited for by a step
below:

| Design keyword | What it says | Waited for by |
|---|---|---|
| a page with no entity | a task form that submits to an operation without loading a record: sign-in, second factor, password reset | step 2 |
| an action offered by the row's state | an expression over the row, in the expression language, that says when a row action is offered | step 3, for that part |
| a picker (`pickers`) | a field that holds the key of another entity's record, picked from a list an operation reads, and may fill other fields | step 4 |
| a rule across fields (`checks`, `enteredTwice`, `fieldConditions`) | a confirmation equal to a password, an end after a start; a field read-only or hidden by an expression. A page is one mode (a form without `source` creates, one with it edits, a view shows), so a field hidden in a mode is one that page does not list | step 4 |
| child rows (`childRows`, a view's `rows`) | rows of a child entity edited under the parent's form, with a maximum and the loaded rows locked | step 5 |
| a confirmation with a reason (`reason`) | the reason is typed, required and sent with the request | step 5 |
| an approval (`workflows`, `inbox`) | a write that starts an approval and answers "accepted, pending", with how completion is announced | step 5 |

`ownedBy` (ADR-046, built) already covers what the request asks of a
page, a menu entry and a server route: a menu entry is a mapping pointer
like any element (`#/menus/members/items/list`), and the server routes are
written from operations, so an operation owned in the TypeScript file's
mappings gets no route. Nothing waits for it; the steps say where they
follow it.

## How the generator fits

**One implementation file per deliverable.** The web front end is its
own deliverable, so it has its own implementation file,
`<name>.typescript.specarch-implementation.yaml`, with `stack.language`
TypeScript and a `ui` target of platform web, framework `nextjs-carbon`.
`specarch generate ui` then finds `specarch-gen-ui-typescript` for it and
`specarch-gen-ui` for a plain JavaScript target in another file, and runs
each once. Its mappings carry `ownedBy` for what the front end does not
build. This is how one specification drives two repositories, a front
end and a service, without a keyword of its own.

**The framework is a stack for idioms.** An idiom applies to a file whose
stacks it renders: today the file's language and each target's SQL
dialect. A `ui` target's framework plays the part a dialect plays for
SQL, so it is a stack too (Q1). The `ui-components` idiom then renders
`nextjs-carbon`, a later SwiftUI generator renders `swiftui`, and an
override in a TypeScript file may render `nextjs-carbon`.

**The `ui-components` idiom.** A concern of its own. Its parts are the
page kinds and field types the generator draws (`list-page`, `form-page`,
`view-page`, `task-page`, `text-field`, `lookup-field`, `confirm-dialog`,
and so on, one part per thing drawn), and each part's rendering for
`nextjs-carbon` names the component, its import, the schema type it
takes and the schema's keys. Every name is a value under `names`, which
is a flat map of strings (Q5). The shipped idiom draws with Carbon:
`DataTable` with its toolbar and `Pagination` for a list, Carbon's form
controls for a form, `Modal` for a dialog. A project's override, in
`spec/implementation/typescript/idioms/ui-components.specarch-idiom.yaml`,
names its own package and components and the keys its schemas use, and
the generator renders through it. No project's names are written in this
repository: the example's override names a fictional library,
`@acme/screens`, whose stub of a few lines sits inside the example.

An override carries `description`, which every idiom file must have; the
request's sample leaves it out and is otherwise valid.

**Per page, two files.** `<route>/page.schema.ts` holds the page as data:
columns, fields and sections, each field's type, validation and title
key, filters, actions with their confirmation and permission, the
operation each calls, and the string key of every text. It is one object
literal, typed by the component's schema type with `satisfies`, and
nothing else. A behaviour data cannot say is a hook: the schema imports
it by name from `<route>/page.hooks.ts`, a file the generator never
writes, so an unwritten hook fails `tsc`, as a missing test body fails
the Go test package. Which fields take a hook is a stack choice, so the
`ui` target's settings name them; a value the expression language can
say is generated from its expression and needs none. `page.tsx` reads the
route parameters, builds the schema with the translator, hands it to the
component, and maps the page's events to routes, nothing more. It is a
server component unless it needs the client; `"use client"` goes only
where a component does.

**Events.** On the web in plain JavaScript, pages talk through one small
event bus, the roadmap's rule for a stack with no build step. Next.js has
a router and React's props, so a page's events become navigation in its
`page.tsx`, and the bus is not written (Q2).

**Per application.** The menu from `menus`, each entry shown only to
someone who holds the permission of the page it opens; a route guard that
refuses a page to someone without its permission and shows the page's
failed state; the strings file with every key the schemas use and the
specification's text as the default language, with a check that no key
is used without an entry; one server route per operation a page calls
when the target's settings put a server between the browser and the
service, or a direct call when they do not; and the theme as Carbon
theme tokens. The menu and the guard read the same `permission`, so a
menu entry cannot open a page that turns its reader away. A derived test
opens each page as someone without its permission and expects the
refusal.

**What the generator does not do.** It writes no `package.json`, no lock
file and no configuration of Next.js, TypeScript or ESLint; those are the
project's, and the libraries it needs are the implementation file's
`libraries`, pinned. The same input gives byte-identical files, keys in a
fixed order, and no time beyond the header every generated file carries.
The output passes `tsc --noEmit` under `strict` and the project's ESLint
with no rule turned off.

**Validation without a form library.** A field's checks come from the
entity's JSON Schema keywords and are written as plain TypeScript in the
schema's data, which the component reads. The base generator adds no form
or state library (react-hook-form, zod or zustand); a project whose
components use one keeps it inside its own library, behind the override.

## The libraries, checked

The example's web application, `examples/library-lending/web-nextjs`,
pins these; its lock file resolves 453
packages (pnpm 12.9.1), scanned with syft, grype and osv-scanner, and the
implementation file's `libraries` records them with the reason for each
version held back.

| Package | Version checked | Licence | Use |
|---|---|---|---|
| `next` | 16.4.0 | MIT | the app router, server routes, the build |
| `react`, `react-dom` | 19.3.0 | MIT | rendering |
| `@carbon/react` | 1.118.0 | Apache-2.0 | the components |
| `@carbon/styles` | 1.117.0 | Apache-2.0 | the styles |
| `@carbon/themes` | 11.83.0 | Apache-2.0 | the theme tokens |
| `@carbon/icons-react` | 11.90.0 | Apache-2.0 | icons |
| `sass` (build) | 1.105.1 | MIT | Carbon's styles are Sass |
| `typescript` (build) | 6.0.3 | Apache-2.0 | `tsc --noEmit`; 7.0 is outside the range `typescript-eslint` takes, which `eslint-config-next` lints with (below 6.1) |
| `eslint` (build) | 9.39.5 | MIT | the lint; 10 is outside the range of the React, accessibility and import plug-ins `eslint-config-next` brings |
| `eslint-config-next` (build) | 16.4.0 | MIT | Next.js's lint rules |
| `@types/react`, `@types/react-dom`, `@types/node` (build) | 19.3.0, 19.3.0, 26.6.4 | MIT | types |

Of the 400 packages resolved, all but four carry an OSI-approved licence
(MIT, Apache-2.0, ISC, the BSD licences, 0BSD, BlueOak-1.0.0, Python-2.0,
MPL-2.0, and OFL-1.1 for the IBM Plex fonts Carbon uses). What the scan
found, each for the owner (Q10):

- `@img/sharp-libvips-*`, LGPL-3.0-or-later, comes with `sharp`, an
  optional dependency of Next.js for resizing images. The generated pages
  resize no images, and `overrides: { sharp: "-" }` in
  `pnpm-workspace.yaml` removes it; that was checked.
- `caniuse-lite`, CC-BY-4.0, the browser data Next.js reads while it
  builds, and `language-subtag-registry`, CC0-1.0, a list the
  accessibility lint reads. Both are data read at build time and never
  shipped in a page; neither licence is OSI-approved.
- `braces` 3.0.3, advisory GHSA-vfj7-8cjw-p6xm (high: deeply nested
  patterns exhaust the stack), with no fixed version. It is reached only
  through the lint (`eslint-config-next`, `fast-glob`, `micromatch`),
  which reads the project's own patterns, never a visitor's.
- Every Carbon package runs IBM's telemetry, `ibmtelemetry`, as an
  install script. pnpm runs no dependency's install script unless told
  to, and the example tells it none, so nothing is sent.

## Questions for the owner

Each is where the request differs from `docs/ui-design.md`, the idiom
schema, the implementation schema, an earlier ADR or the repository's
rules, or a choice the plan cannot settle alone. The owner answered every
one as recommended on 2026-10-09.

| | Question | Recommendation |
|---|---|---|
| Q1 | The request names the idiom's stack `typescript` or `nextjs-carbon`. `docs/idioms.md` counts as a file's stacks only its language and its SQL dialects. Should a `ui` target's framework count as a stack? | Yes. `typescript` would bind the components to the language, and a TypeScript project on another framework would inherit Carbon. The framework is to a UI what the dialect is to SQL. The validator's `idiom_stack` follows, in both builds. |
| Q2 | The roadmap says pages and components on the web talk through one small event bus. Does that hold for Next.js? | No. The bus is the plain JavaScript stack's, where no framework carries events; in Next.js a page's events are navigation in its `page.tsx`. The roadmap and the target's description say so. |
| Q3 | `docs/generators.md` accepts a UI emitter only once it reproduces a screen built by hand in a real project; the real Next.js screens are private. | As ADR-040 did: the sign-in and second-factor screens are written by hand in the example first, committed alone, and the generator reproduces them; the real project repeats the check against its own screen through its override, outside this repository. |
| Q4 | The request's override records its own library as `licence: proprietary`; the implementation schema says a library's licence must be OSI-approved. | Keep the rule for third-party libraries. A library the project's own organisation writes is not a third-party dependency: record it with SPDX's form for a licence not on the list, `LicenseRef-<name>`, and say so in the schema's description. The example's stub is Apache-2.0, as the repository is. |
| Q5 | `names` in an idiom part is a flat map of strings. The request asks what an override needs when its schema shape differs per field type or nests. | A key per role in `names`, the part per field type carrying that type's keys, covers what the request's sample shows. Where a shape cannot be said that way, the project installs its own `specarch-gen-ui-typescript` ahead of the shipped one, as the plug-in lookup already allows. No schema-shape language now; add one when a real override fails. |
| Q6 | Which example carries the web application? | The library lending example, which already holds the pages, menus, states, theme and plain JavaScript screens. It gains a TypeScript implementation file and two operations, sign in and confirm a second factor, under its existing session. The lending desk stays the example for `extract`. |
| Q7 | The request wants the password rules shown to come from "the specification's password policy"; there is no such keyword. | The rules are the JSON Schema keywords of the sign-in and reset requests' password property (`minLength`, `maxLength`, `pattern`), shown and checked as written. A rule only the server can check, such as a password used before, is a problem type the page shows in its failed state. No new keyword. |
| Q8 | ADR-040 refuses a list whose operation does not page. The request wants a server route that pages such a list, said in the specification. | Whether a server route pages a list is how this stack is built, so it goes in the `ui` target's settings, naming the operations it pages. Without that, the list is refused as today. |
| Q9 | `generate --check` compares files byte for byte. The request wants pages built by hand compared as data, with the difference per page. | A step of its own, last. The generated schema is one object literal in a subset of TypeScript that is also JSON5; the check parses both files in that subset and compares the values. A hand-built file outside the subset is reported as not comparable, never as equal. |
| Q10 | The scan's four findings above: libvips (LGPL), two data packages under licences that are not OSI-approved, an advisory with no fix, and Carbon's telemetry. | Remove `sharp` with the override. Accept `caniuse-lite` and `language-subtag-registry` as data read at build time, as container base-image packages are accepted, and record them under `licence_exceptions`. Accept the `braces` advisory for the lint only, with a review date, and look again when step 2 pins its versions. Keep install scripts off, and set `IBM_TELEMETRY_DISABLED=true` in CI as well. |

What Q10's answer accepts is recorded where the dependency scan reads it,
`.dependency-allowlist.json`, so the commit of step 2 that brings the
lock file passes the scan with the reasons on record:

| Accepted | Under | Why, and until when |
|---|---|---|
| `caniuse-lite`, CC-BY-4.0 | `licence_exceptions` | browser data Next.js reads while it builds; never shipped in a page, as a container base image's packages are accepted |
| `language-subtag-registry`, CC0-1.0 | `licence_exceptions` | a list the accessibility lint reads; never shipped in a page |
| `braces` 3.0.3, GHSA-vfj7-8cjw-p6xm | `vulnerabilities` | no fixed version; reached only through the lint, which reads the project's own patterns, never a visitor's; review by 2027-04-09, or when a fixed release exists |

`sharp` and its LGPL `@img/sharp-libvips-*` are not accepted: the
example's `pnpm-workspace.yaml` removes them with `overrides: { sharp: "-" }`,
and its `allowBuilds` denies every dependency's install script by name. The
packages the shipped `ui-components` idiom names are under its
`application` part's `libraries`; the example's implementation file pins
them.

## Building the generator

Each step is a work item of its own: it changes the specification of
`specarch` first, adds its conformance cases where the validator moves,
and keeps `docs/`, the history and both validator builds current. Step 1
needs Q1, Q4 and Q5; step 2 needs Q2, Q3, Q6, Q7 and Q10.

1. **Built: the `ui-components` concern and the shipped idiom.** `ui-components`
   joins the concern list of the idiom schema; a `ui` target's framework
   counts as a stack of its file (Q1), in both validator builds and in
   `specarch idioms`. The shipped `idioms/ui-components/ui-components.specarch-idiom.yaml`
   renders `nextjs-carbon` with plain Carbon, one part per page kind and
   field type, every name under `names`. The implementation schema's
   licence rule says how a project's own library is recorded (Q4). Waits
   for nothing. Done when the idiom validates, `specarch idioms` lists it
   for a TypeScript file whose `ui` target is `nextjs-carbon` and not for
   one on `plain-javascript`, an override of it rendering a stack its file
   does not have is refused with the same message in both builds, and
   conformance cases show all three.
2. **Built: task forms, sign-in, second factor, password reset.** The
   library lending example has a second factor, `signIn` answering 201
   when it is asked for and `confirmSecondFactor` taking the code (Q6),
   and a Next.js application under `examples/library-lending/web-nextjs/carbon/`
   with its `package.json`, `pnpm-lock.yaml` and `pnpm-workspace.yaml`, the
   libraries pinned and scanned (Q10), which a CI job installs, checks with
   `tsc --noEmit` and the lint, and builds. Its sign-in and second-factor
   screens are the reference, written by hand on plain Carbon in a commit
   of their own before the generator. `specarch-gen-ui-typescript` (SA-52,
   ADR-067) writes, from the example's TypeScript implementation file, per
   task page the schema and `page.tsx`, the components under `screens/`
   and the texts in `strings.ts`; the events of a task form, to the second
   factor on 201, and back to the page that asked for sign-in through
   `returnTo`; checks as rules in the schema's data; the password rules
   from the request schema (Q7). The task-page part names its `checks`,
   `failed` and `events`, and the idiom has an `email-field` part. Done:
   generated over the reference, the two screens differ only by the
   header and `strings.ts` only by the password reset's texts; the CI job
   builds the result, and a second run writes the same bytes.
3. **Built: lists, with the override** (SA-53, ADR-068). List pages with
   server-side paging, sort, search and typed filters (the paginated-list
   idiom's names), a column picker, a refresh that keeps the page, row
   actions offered by the row's state with their confirmation, reason and
   message; the menu, the route guard and a derived test per page that
   opens it as someone without its permission and expects the refusal,
   all from the session the target's settings name, which the example's
   `getSession` answers; `ownedBy` on a page, a menu or a menu entry
   leaves it out. The soft-delete idiom's toggle is not written: the
   standard wire names have no parameter that asks for deleted records.
   The example's web screens are a pnpm workspace: `carbon/` on plain
   Carbon, `acme/` from a second TypeScript implementation file whose
   override draws the lists through `@acme/screens`, and `acme-screens/`,
   that library's stub; CI type-checks, lints, tests and builds both.
   Done: the members, loans and fee waivers lists build both ways, the
   refusal tests pass, and `generate --check` is clean on both and reports
   a schema edited by hand.
4. **Built: forms and views** (SA-54, ADR-069). Forms that create a
   record or load and change one, with sections, validation from the
   entity's JSON Schema keywords, `checks` across fields and fields
   `enteredTwice`, fields read-only or hidden by `fieldConditions`,
   layouts as one page, tabs or steps (`settings.layouts`); `pickers`
   drawn as lookups; the hooks of the fields `settings.hooks` names,
   handed on by `page.client.tsx` from `page.hooks.ts`, which the project
   writes; views as their sections read-only. Done: the loan form picks
   the member and the book, checks that the copy is due after the day it
   is lent and starts that day with a hook; it, the member's form and the
   member's view build both ways.
5. **Built: child rows, reasons and approvals** (SA-55, ADR-071). Child
   rows sent in the body's array named for the relation, drawn inline,
   loaded rows read from the view the source answers (`rows`, ADR-070),
   locked and never sent, a maximum count and per-row checks; a view's
   actions that run an operation, with a confirmation that requires a
   reason and sends it, through the list's one confirmation; a form that
   starts an approval acting on the 202 answer as its pending state, and
   the approver's inbox as a list with the approval's operations as row
   actions. Rows in a dialog are not written, and the page does not follow
   an approval to its end, which is a channel message no browser reads.
   Done: the member's copies lent as rows under their loans, the member
   deactivated from the view with a reason, a fee waiver asked for and
   approved or refused with a reason from the inbox, built both ways.
6. **Server routes, strings and theme.** A server route per operation a
   page calls, when the target puts one between browser and service,
   adding the session's token and paging what the settings name (Q8), and
   none for an operation the TypeScript file marks `ownedBy`; the strings
   file in two languages, the chosen one kept across sessions, and the
   check that every key has an entry; the theme as Carbon tokens, light
   and dark, through a token-rendering idiom, the one ADR-040 said a
   second stack would bring. Waits for nothing. Done when a key used
   without an entry fails generation, the example's routes forward to the
   service, and its theme shows in both modes.
7. **Comparing screens built by hand.** `generate --check` on a folder of
   pages built by hand says per page whether the schema and the page
   match, comparing values (Q9), and prints the difference. Waits for
   nothing. Done when the reference screens of step 2 compare equal and a
   changed column is reported as that column.

The handoff that asked for this stays in the mailbox until step 7 is
built and pushed.

## Left out

The request came from a project whose name, owner, domain and component
library are not in this repository. Left out on purpose: how many
applications and pages it has, which form and state libraries its own
screens use, and the account it gave of a menu permission nothing
enforced, which is kept only as the reason the menu and the guard read
one permission. The library in every example is the fictional
`@acme/screens`.
