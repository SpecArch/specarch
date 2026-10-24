# Capability check, 2026-10-24: three jobs

What SpecArch lacks for three jobs the owner starts today: a website
from a design tool's file, an information system that signs in through
single sign-on (SSO), and a multi-tenant office suite. Each job is
walked from the first requirement to a deployed, tested system, as a
system analyst, a system architect and a tester would walk it; at each
step this file says what SpecArch specifies and generates today, what it
cannot, and what the owner does by hand. Every claim cites the schema, a
document or the code at the line it comes from; an assumption is marked
as one. "Today" is release 0.7.5 and the main branch at 0.7.6-dev.

## Conclusion

The three most blocking gaps per job, in the order the owner meets them:

**Job 1, a website from a design file.**

1. No page kind for content. A page is a list, a form, a view or a task
   bound to an entity or an operation (G1.2); a website is mostly pages
   of copy, images and links, and those have no element and no
   generator. Blocks: the site itself is written by hand.
2. No reader of a design file (G1.1). Frames, components and tokens are
   read by eye and typed into pages and `theme.yaml`. Blocks the import;
   the typing is a day's work for a small site.
3. The theme takes colour, dimension, font family and weight, duration
   and number only (G1.3); typography, shadows, borders, images and
   icons stay in the stack's own files. Workaround.

**Job 2, an information system with SSO.**

1. No element for the sign-in flow through an identity provider (G2.1):
   the authorisation-code redirect, the callback with its code and
   state, PKCE, the nonce, the identity token's checks, discovery and
   the keys. What exists is a sign-in task page posting to an operation
   of this system. Blocks the SSO proof: the flow and its red tests are
   written by hand.
2. No sign-out (G2.2): the session has timeouts and nothing ends it;
   provider-initiated and back-channel logout and revocation have no
   keyword. Workaround.
3. No mapping from the provider's claims to this system's roles (G2.6):
   a role is granted here, and a `groups` claim is only a claim.
   Workaround.

**Job 3, a multi-tenant office suite.**

1. Tenant scope is one `rows` rule per role and per entity, an entity
   with no rule is reached whole (G3.1). Across seven applications it is
   hundreds of lines, and one forgotten entity leaks across tenants.
   Blocks, as a safety matter.
2. Screens that cross modules are refused (G3.6): a picker or a list
   reading another module's records is the error `reference` in every
   code target, and an office suite crosses modules on most screens.
   Blocks for those screens.
3. The editors (mail, documents, spreadsheets) and anything live
   (collaboration, WebSocket) have no page kind and no interface beyond
   HTTP, messaging and the command line (G3.9). Blocks: those screens
   are built by hand, and today they cannot even be named in the design
   so that the menu, the guard and the flows cover them.

**Across the three jobs, what the owner meets first**, from the first
hour of design to the deploy:

| Rank | Gap | Jobs | Met when | Severity |
|---|---|---|---|---|
| 1 | G3.1 tenant scope per role and entity, fail-open by omission | 3 | the first entity | blocks |
| 2 | G1.2 no content page | 1 | the first page | blocks |
| 3 | G2.1 no identity-provider sign-in flow | 2 | the sign-in page | blocks |
| 4 | G3.9 no page kind for a hand-built screen, no interface beyond HTTP | 3 | the first editor screen | blocks |
| 5 | G3.6 cross-module screens refused | 3 | the first picker across modules | blocks |
| 6 | C1 handlers written by hand on every stack but Go on dxlib | 1, 2, 3 | `generate` | workaround, known |
| 7 | G1.1 no design-file reader | 1 | the start of design | blocks the import, workaround for the job |
| 8 | G3.2 no per-tenant settings or plan entitlements | 3 | the first plan-gated feature | workaround |
| 9 | G2.2 no sign-out | 2 | the second screen | workaround |
| 10 | C2 the test harness is hand-written per stack | 1, 2, 3 | `generate tests` | workaround, known |
| 11 | G3.7 no shared shell across modules (menu, theme, strings) | 3 | the second module | workaround |
| 12 | G1.3 theme without typography, shadows, images, icons | 1 | the theme | workaround |
| 13 | G2.6 claims to roles | 2 | the roles | workaround |
| 14 | G1.5 one compact class, no layout per breakpoint | 1 | the responsive rules | workaround, owner's decision D1 |
| 15 | C4 the acceptance loop is designed, not built | 1, 2, 3 | the first user test | workaround |
| 16 | G3.8 gateway routes by audience claim, not by path | 3 | the deploy behind one origin | workaround |
| 17 | G1.6 no visual check of a generated screen | 1 | the first review | nice to have |
| 18 | G3.5 no isolation model beyond row security on PostgreSQL | 3 | the implementation file | workaround |
| 19 | G2.3 refresh-token rotation has no derived case | 2 | the tests | nice to have |
| 20 | G3.10 estate-wide generation, idioms and CI in one run | 3 | the tenth module | nice to have |

## Method and legend

Read for this check: the design schema and its definitions
(`schema/specarch-design-0.3.schema.json`), the implementation and
estate schemas, `docs/stages.md`, `docs/conventions.md`,
`docs/ui-design.md`, `docs/ui-nextjs-carbon.md`, `docs/ui-swiftui.md`,
`docs/meta-model-0.3.md`, `docs/caller-context.md`, `docs/estate.md`,
`docs/gateway.md`, `docs/generators.md`, `docs/test-generation.md`,
`docs/traceability.md`, `docs/acceptance.md`, `docs/roadmap.md`, the
idioms, the library-lending example, and the code where a claim rests
on it (`internal/gensql/rows.go`, `internal/genui/genui.go`,
`internal/validate/caller.go`, `cmd/specarch/main.go`). The handoff
mailbox was empty.

Severity, per job:

- **blocks**: the job cannot be done through SpecArch at that step; the
  owner does that part outside it and the specification does not cover
  it.
- **workaround**: SpecArch covers it, with more writing than it should
  take, or with the piece written by hand beside the specification.
- **nice to have**: the job goes through; the gap costs review time or
  a later change.

Release placement follows the version rule of `docs/maintenance.md:176-190`:
before 1.0.0 an addition to the public interface takes the patch step
and a breaking change the minor step. A keyword that adds to meta-model
0.3 and breaks no file is a 0.7.x release (0.7.6 is planned,
`records/releases/0.7.6.yaml`); a change that breaks an existing file
moves the meta-model, as 0.2 and 0.3 did (`docs/meta-model-0.3.md:12-24`),
and is 0.8.0. "Owner decision" marks a gap whose smallest close is a
choice the owner has declined or deferred before, named where it was.

## The path every job takes today

The stages and what each holds are `docs/stages.md`; this is what the
owner does at each, for all three jobs, and the gaps that are the same
for all three (C1 to C6).

| Step | SpecArch today | By hand |
|---|---|---|
| Requirements | stakeholders, needs, requirements with acceptance criteria and verification method, glossary, assumptions, constraints; every gap a warning (`docs/stages.md`, Requirements; `docs/traceability.md:11-26`) | the interviews and the writing |
| Design | enums, entities, value objects, views, permissions, roles, caller, clients, tokens, session, middlewares, paths, commands, channels, dependencies, jobs, gateways, workflows, errors, pages, menus, flows, accessibility, theme, algorithms, decisions (schema root, lines 20 to 376) | every element; algorithm bodies |
| Implementation file | one per stack: libraries with licences, layout, mappings, bindings, targets, tasks, testing, idioms, deployments (`schema/specarch-implementation-0.1.schema.json`) | the file; `Package.swift`, `package.json`, the component library |
| Questions | `gaps`, `decide`, `approve`; `generate` runs only after approval, or as a marked draft with `--unapproved` (`docs/acceptance.md:16-19`) | the answers |
| Tests | derived cases per subject; `derive` writes drafts; `generate tests` writes Go, Swift or Dart tests against a `Harness` the project writes (`docs/generators.md:1014-1080`) | the harness, every test body the design gives no call for |
| Code | OpenAPI 3.1; SQL migrations for four engines; sqlc queries; a Go service on dxlib; screens in plain JavaScript, Next.js with Carbon, or SwiftUI; BPMN; guarded scripts (`README.md:201-233`) | the server on every other stack (C1) |
| Documents | techspec, requirements, test plan, traceability, deployment guide, commissioning procedure, handover, manual, operations guide, questions, comparison, problems, HTML site | reading them |
| Deployment | environments, settings, release and rollback steps, migrations, handover; servers, links, build, method, infrastructure per deployment | images, hosts, CI, the deploy itself |
| Commissioning | checks of five kinds with prose steps, sign-off, run records (`docs/stages.md:137`) | running them |
| Operation | monitors with objectives and responses, backup, restore, incident and defect records | the tooling |

Gaps common to the three jobs:

- **C1. Server code on every stack but Go on dxlib is written by hand.**
  `generate openapi` writes the document and the stack's own OpenAPI
  generator makes the interfaces; the handlers are hand-written, by
  the rule in `docs/estate.md:524-531`. Workaround, by design, in every
  job; each job's server effort is the owner's.
- **C2. The test harness is hand-written** per stack: how a caller signs
  in, how a record is inserted, how an operation is called
  (`docs/generators.md:1030-1040`). Workaround; it is where every SSO
  and tenant test below is actually wired.
- **C3. Deployment is described, not produced.** No container file,
  compose file or pipeline is generated; `build`, `method` and
  `infrastructure` are prose per deployment (`docs/stages.md:97-119`).
  Workaround, by design (`docs/roadmap.md:573-577`).
- **C4. The acceptance loop is designed, not built.** A tester's remark
  going back into the specification as a change request, with what it
  reaches listed, is ADR-083, proposed; "nothing here is built yet
  except the principle and the marked draft" (`docs/acceptance.md:10-11`).
  Workaround: change requests are records written by hand.
- **C5. Pages are not extracted**; an existing screen is read by hand
  into a tree (`docs/from-sources.md:458-460`). Relevant only where a
  job starts from existing screens.
- **C6. No real project's screen has checked the web generators**
  (`docs/roadmap.md:171-172`); the comparison with a hand-built screen
  ran on the example's own sign-in and lists. Each job is the first.

## Job 1: a website from a design file

The owner has a design file (frames per page and per breakpoint,
components, variables for colour and type, assets, copy) and wants a
public website with a few forms. The walkthrough:

| Step | Analyst | Architect | Tester | SpecArch today |
|---|---|---|---|---|
| Requirements | the pages, the audiences, the forms, the accessibility level, the languages | hosting, the stack, analytics as a dependency | acceptance criteria per requirement | all specifiable: `requirements`, `accessibility: { standard: WCAG 2.2, level: AA }` (`docs/ui-design.md:257-287`) |
| Tokens | colours, spacing, type, with light and dark modes | the component library that takes them | contrast at AA, checked by the validator | `theme` in the Design Tokens format, srgb colours, modes and pairs; contrast computed (`docs/conventions.md:1834-1879`) |
| Pages | one page per frame, with its sections, copy, images and links | routes, the navigation | each state, each link | only list, form, view and task (schema line 3053, `$defs/page/properties/kind`); a content page has no kind (G1.2) |
| Components | the design file's components | the `ui-components` idiom's parts, overridden with the project's library | a hand-built screen compared with the generated one | the idiom maps page kinds and field types to components (`idioms/ui-components/ui-components.specarch-idiom.yaml:1-40`), not a design file's components (G1.9) |
| Responsive | what each page keeps on a phone | breakpoints | the phone and desktop views | one class, `compact`, and `compactColumns` on a list (`docs/ui-design.md:227-240`) (G1.5) |
| Assets | images, icons, fonts | where they are served from | they load | left out of the theme on purpose (`docs/ui-design.md:333-336`) (G1.4) |
| Copy | every text, per language | a strings file per stack | every key has a translation | strings are generated from titles, labels and states, with translations from a file the project writes (`docs/ui-nextjs-carbon.md:380-383`); copy of a content page has no place (G1.8) |
| Forms | contact, newsletter, sign-up | the operations they submit to | derived cases per field and per problem type | fully covered: a task page with `permission: public`, `submit`, `fields`, `checks`, `states` (`docs/conventions.md:1632-1668`) |
| Visual check | | | the page looks like the frame | none: `generate --check` compares a hand-built page's schema values (`docs/ui-nextjs-carbon.md:390-402`); accessibility checks cover labels and contrast (`docs/ui-design.md:261-281`) (G1.6) |
| Deploy | | a static host or Next.js | smoke and end-to-end checks | commissioning checks with prose steps; the hosting by hand (C3) |

**How the design file becomes spec elements today.** By hand, element
by element. Tokens: the Design Tokens Community Group format is what
`theme.yaml` takes (`docs/ui-design.md:94-104`), so a token export in
that format is a JSON-to-YAML transcription; whether the design tool
exports its variables in that format without a plug-in is an assumption
this check did not verify (A1). Frames become pages only where a frame
is a list, a form, a view or a task; every other frame is outside the
specification. Components become an override of the `ui-components`
idiom once the owner has built them in the stack; the idiom is the
designed way for a project's own library (`docs/ui-nextjs-carbon.md:15-18`).
Copy becomes page titles, labels and state messages, and nothing else.

**How the spec becomes a website today.** Two web generators write
application screens: plain JavaScript (`docs/generators.md:638-700`)
and Next.js with Carbon (`docs/generators.md:777-972`). Both need an
operation behind every page: a list needs `source`, a task `submit`
(schema, `$defs/page`). A page event and a menu entry name a page of the
specification, never a URL (schema lines 3386 and 6140), so nothing
points outside the application. A site with no operations gives the
generators nothing to write. For the forms the Next.js generator writes
the route, the page, the server route that forwards to the service, the
strings and the theme (`docs/ui-nextjs-carbon.md:374-390`).

### Gaps, job 1

| Gap | What is missing | Severity | Smallest close | Release | Source |
|---|---|---|---|---|---|
| G1.1 | A reader of a design file: frames to pages, variables to tokens, components to idiom parts, assets to files | blocks the import; workaround for the job | A printer in the pattern of the route table (`docs/estate.md:398-409`): a dump the design tool's API or a plug-in writes, holding the tokens in the Design Tokens format, each frame with its name, its children's kinds and texts, and each component; `extract design` reads tokens into `theme.yaml` as facts and writes a `must` question per frame asking which page kind it is, since that is a judgment. Tokens first, frames later | tokens: 0.7.x; frames: with G1.2 | `docs/extraction.md:8-54` (the readers' rule), `docs/ui-design.md:94-104` |
| G1.2 | A page of content: a route, a title, ordered sections of headings, text, images and links, no entity, no operation; the menu, the route guard, the flows and the strings covering it | blocks | `kind: content` on a page with `sections` of blocks (`heading`, `text`, `image`, `link` to a page or an action), each text a string key; both generators write it; `accessibility` checks the image's text and the link's label | 0.7.x, an addition to 0.3 | schema line 3053; `docs/ui-design.md:19-23` (D1 keeps structure in the design) |
| G1.3 | Theme composite types: typography, shadow, border, gradient, transition | workaround (the stack's CSS by hand) | take the format's composite types the file names as left out, with the `design-tokens` idiom rendering them | 0.7.x | `docs/ui-design.md:333-336` |
| G1.4 | Assets as files: images, icons, fonts, with their alternative text | workaround (committed beside the generated code) | an `assets` section, each a file path, a media type and its text, referenced by a content block and by a field of format `uri` | 0.7.x, with G1.2 | `docs/ui-design.md:333-336` |
| G1.5 | Layout per breakpoint: what moves, what stacks, what hides, beyond a list's compact columns | workaround (CSS by hand) | none without revisiting D1 and D5, which keep geometry in the stack; a content section could take `compact: hidden` like a column | owner decision | `docs/ui-design.md:19-23`, `239-241`, `394-399` |
| G1.6 | A visual check: a screenshot of the generated page against the frame, or a browser test of each state | nice to have | a commissioning check of kind `end-to-end` with the frame as a cited source, run by hand; a browser harness for `generate tests` is a later target | later | `docs/stages.md:137-139`, `docs/ui-design.md:283-287` |
| G1.7 | Site output: static files, a page title and description per route, a sitemap, a canonical URL | workaround (hand-written site, generated forms inside it) | with G1.2: `description` on a page written as the document's metadata, and a `static` setting on the `ui` target for a site with no server routes | 0.7.x | `docs/generators.md:638-700` |
| G1.8 | Copy of a content page per language | workaround | with G1.2: every block's text a string key, translated as page strings are | 0.7.x | `docs/ui-nextjs-carbon.md:380-383` |
| G1.9 | Components of the design file as the idiom's parts | workaround, by design | none: the idiom is overridden with the library the owner builds from the design; a reader (G1.1) could list the components to override | with G1.1 | `docs/ui-nextjs-carbon.md:15-18` |

What the owner does by hand for job 1 today: the whole site except its
forms; the theme's colours and spacing typed from the design file; the
component library; the hosting; the visual review.

## Job 2: an information system with single sign-on

A new back-office system whose users sign in through an external
identity provider (OpenID Connect, with SAML asked for by one customer),
with roles here, a session in the browser, machine clients, and the
failure paths tested. The walkthrough:

| Step | Analyst | Architect | Tester | SpecArch today |
|---|---|---|---|---|
| Requirements | who signs in, with which provider, which roles, session limits, the audit | the provider as a dependency, the trust boundary | acceptance criteria: a refused sign-in is refused, a session ends | specifiable; the provider is a `dependency` of kind `identityProvider` with a `timeout` and `protocol: https` (schema line 2984; `docs/conventions.md:1089-1142`) |
| Sign-in flow | the redirect to the provider, the return, what the person sees | the callback endpoint, PKCE, state, nonce, the identity token's issuer, audience, signature, expiry, clock skew, discovery and keys | a red case per check | none of the flow is an element (G2.1); a sign-in page is a task page posting `email` and `password` to an operation of this system (`examples/library-lending/spec/design/pages/sign-in.yaml`, `docs/conventions.md:1632-1668`); the keys are a deployment's fact (`docs/gateway.md:584-585`) |
| Tokens | what the system accepts | format, lifetime, signature, claims, audience fixed, checks over claims | `denied with expired <token>`, `refused with a token for another audience`, `<check> fails` | covered: `tokens` with `format`, `lifetime`, `signature`, `claims`, `aud` as `const`, `checks` (`docs/conventions.md:907-1013`; schema lines 6700 and 6708) |
| Session | idle and absolute limits | the cookie, its attributes, rotation at sign-in, CSRF | `denied with expired session` on every non-public operation and page | `session` has the two timeouts and derives the case (`docs/conventions.md:1037-1051`, schema line 2365); the cookie and CSRF are the stack's (G2.4) |
| Roles and permissions | roles, what each may do, separation of duties, rows a role reaches | fail-closed: every operation names one permission | `denied without permission` per operation; `shows only the rows of <role>` | covered (`docs/conventions.md:745-813`); how a provider's group becomes a role here is not (G2.6) |
| Clients | the front end as an application, batches as services | proof per client, `callers` per operation, `through` on a grant | `refused for a signed-in person`, `refused with a bad signature`, `refused through a client not listed` | covered: `clients` with `kind`, `proof` of five kinds (schema line 6409), `callers`, `through` (`docs/caller-context.md:178-354`) |
| Logout | the person signs out; the provider signs them out everywhere | provider-initiated logout, back-channel logout, revocation | the session is gone after sign-out | nothing (G2.2) |
| Second factor, step-up | a code after the password; a stronger token for a sensitive operation | `acr` and `amr` as registered claims; an operation naming the token it accepts | `fails with <problem>` on the task page | covered: a second-factor task page exists in the example; `token` on an operation picks the step-up format (`docs/meta-model-0.3.md:268-274`) |
| Refresh | the session outlives the access token | a refresh token as a format, the exchange as an operation | rotation and reuse detection | formats and the exchange are expressible (`docs/meta-model-0.3.md:268-274`); rotation has no derived case (G2.3) |
| Failure paths | the provider is down or slow | `calls: [identityProvider]` on the callback | `dependency fails identityProvider`, `dependency times out identityProvider`, both critical | derived once the callback operation names the dependency (`docs/conventions.md:1089-1104`) |
| Security events | every refusal recorded | an append-only store and an alarm channel | each refusal's event checked by the generated test | covered: `securityEvents` with `store` and `alarm`, an event per derived refusal (`docs/caller-context.md:530-627`) |
| Tests | | the harness signs in through a stub of the provider | | the stub is `testing.mocks` prose and the harness is hand-written (C2; `schema/specarch-implementation-0.1.schema.json:1659`) |
| Screens | | the route guard reads the session the settings name | a derived test per page opens it without its permission | the Next.js generator writes the guard and the server routes with the session cookie's value as the token header; `getSession` is the project's (`docs/ui-nextjs-carbon.md:334-337`, `374-378`) |

**What is already proven by the derived cases** once the tokens and the
clients are written: an expired token on every operation, a token for
another audience, each claim check, a caller not in `callers`, a client
with a bad signature, a grant used through the wrong application, a
service calling with its own token, a dependency that fails or times out
on the callback, and the security event each writes
(`docs/conventions.md:1004-1013`). That is most of the red list a tester
would write for a resource server. What is not proven is the sign-in
flow itself.

### Gaps, job 2

| Gap | What is missing | Severity | Smallest close | Release | Source |
|---|---|---|---|---|---|
| G2.1 | The sign-in flow through a provider: a page event that leaves the application for the provider's authorisation endpoint, the callback operation with `code` and `state`, PKCE, the nonce, the identity token with `iss` fixed, the checks of RFC 9700 and OpenID Connect Core 3.1.3.7, discovery and keys; SAML's metadata, assertion consumer and signed assertion | blocks the SSO proof | a `signIn` object of the design: `kind: oidc` or `saml`, `provider: <dependency>`, `flow: authorizationCode` with `pkce: true`, the callback as an operation of this system named under `callback`, the identity token declared under `tokens` with `iss` and `aud` as `const` and `nonce` as a claim; derived red cases: `state does not match`, `nonce replayed`, `issuer differs`, `identity token expired`, `signature invalid`, `code used twice`, each a security event; the page event `navigate: signIn` as the one event that leaves the application. Discovery and the keys stay the deployment's (`jwksUri`, `issuerUrl`) | 0.8.0 if the page event's form changes, else 0.7.x; owner decision on OIDC first and SAML after | schema lines 3053 and 3386; `docs/conventions.md:1632-1668`; `docs/gateway.md:584-585` |
| G2.2 | Sign-out: ending the session here, provider-initiated logout, back-channel logout the provider calls, revocation of a refresh token | workaround: an operation `signOut` with its permission and a page action, by hand; back-channel logout as an operation only a declared service client may call | `session.endedBy: <operation>` so the derived case `denied with expired session` gains `denied after sign-out`; with G2.1, `logout: { backChannel: <operation> }` on `signIn` | 0.7.x | `docs/conventions.md:1037-1051`; schema line 2365 |
| G2.3 | Refresh-token rotation and reuse detection as derived cases | nice to have | on a token with `tokenType: refresh_token` (an added value), derive `refused when a rotated refresh token is presented again` | 0.7.x | `docs/meta-model-0.3.md:268-274`; schema line 6708 |
| G2.4 | The browser session's cookie attributes, rotation at sign-in and CSRF | workaround, by design: implementation, in the generated server routes and the project's `getSession` | a `session` idiom (`concern: session`) naming the cookie's attributes and the CSRF rule per stack, checked as idioms are | 0.7.x, owner decision | `docs/ui-nextjs-carbon.md:374-378`; `docs/idioms.md` |
| G2.5 | A token this system accepts but another issues: `tokens` is worded as what a service issues | none, in practice: the accepted token is declared the same way, with `iss` as a claim with `const` | wording in `docs/conventions.md` naming the accepted token | 0.7.x | `docs/conventions.md:907-910` |
| G2.6 | The provider's claims to this system's roles: a `groups` or `roles` claim mapped to `roles`, or just-in-time provisioning of the person | workaround: an algorithm with worked examples maps the claim, and the harness applies it; or the provider is configured to send the role names, `info.roleNames` fixing their wire form | `roles.<role>.claim: { name: groups, value: "..." }`, checked against the token's claims, shown in the matrix, with the derived case `refused with a group that maps to no role` | 0.7.x | `docs/estate.md:228-254`; `docs/conventions.md:907-960` |
| G2.7 | Sender-constrained tokens for people (DPoP) | nice to have | waits for a system that has one, as the design says | later | `docs/caller-context.md:779` |

What the owner does by hand for job 2 today: the sign-in route and the
callback handler with every check, the sign-out, the session cookie,
the stub provider for the tests, the harness, the handlers (C1), the
mapping from the provider's groups to roles, and the SAML side entirely.

## Job 3: a multi-tenant office suite

Mail, documents, spreadsheets, CRM, projects, administration and
billing, all under tenants, each application a module of an estate,
with shared services (identity, files, search, notifications). The
walkthrough:

| Step | Analyst | Architect | Tester | SpecArch today |
|---|---|---|---|---|
| Requirements | per application and for the suite; tenants, plans, limits | the estate, the identity module, the shared services | acceptance criteria per requirement, per module | one specification per module and an estate file listing them (`docs/conventions.md:1202-1231`); the identity module holds clients, roles and the trust domain (`docs/caller-context.md:95-176`); no estate-level requirements (G3.10) |
| Tenant isolation | every record belongs to one tenant, and nobody sees another's | the tenant as a caller property, a rule per entity, row security in the database | `shows only the rows of <role>`, `a record outside the rows is not found`, `a change outside the rows is refused`, per role and operation | `caller.organisationId` and `roles.<role>.rows.<Entity>: organisationId == caller.organisationId`, one line per role and entity; "an entity not listed is reached whole" (`docs/conventions.md:782`); `generate sql` writes a policy on PostgreSQL and a warning elsewhere (`internal/gensql/rows.go:12-27`, `97-112`) (G3.1, G3.5) |
| Per-tenant settings and data | a tenant's own settings, branding, limits | where they are kept | they apply | `configuration` is per deployment (`schema/specarch-implementation-0.1.schema.json:1312`), `enabledBy` a deployment boolean (`docs/conventions.md:2004-2018`); per-tenant settings are an entity the owner designs, with no keyword reading them (G3.2) |
| Plans and billing | plans, subscriptions, metering, invoices, payment | entities, a pricing algorithm, a monthly job, the payment provider as a dependency | worked examples of the pricing, derived cases of the job | specifiable: `decimal` for money (`docs/conventions.md:538-566`), `algorithms` with examples, `jobs` with a schedule, `dependencies` of kind `system`; plan entitlements gating features are not (G3.2) |
| Tenant admin | a tenant's administrator manages its users, roles and settings | a role with rows rules on User and Settings; separation of duties | `refused through a client not listed`, `denied without permission` | covered: roles with `rows`, `separationOfDuties`, `workflows` for approvals, `audited: true` and `deletion: soft` (`docs/conventions.md:1015-1035`, `1323-1355`) |
| Tenant lifecycle | sign-up, suspension, closure, data export | states on a Tenant entity, a download stream for the export, guarded scripts | a move from the wrong state | covered: `stateField` and `transitions`, `stream` (schema line 2642), `guard` and `generate scripts` |
| The applications as modules | each application its own specification; shared records referenced | `dependencies` with `specification` and `version`, references `<dependency>#/entities/X` | the umbrella run resolves every reference | validated across the estate; generated one module at a time, and a page or picker reading another module's list is refused (`docs/conventions.md:1192-1200`, `spec/tests/generate-openapi-reference-refused/case.yaml:1-4`) (G3.6) |
| Shared services | identity, files, search, notifications, audit | the identity module; a file store dependency; channels on a broker; jobs consuming them | `dependency fails`, `dependency times out` | covered for identity, files and events; search across modules and a suite-wide audit log have no element (G3.11) |
| The editors | mail, documents, spreadsheets, with people editing together | WebSocket, operational transforms, rich content | | no page kind, no interface beyond HTTP, messaging and the command line (`docs/roadmap.md:361-363`, `docs/dxlib-lessons.md:341`) (G3.9) |
| One suite in the browser | one menu, one theme, one sign-in, an application switcher | a shell | | `menus`, `theme` and strings are per specification; nothing is shared across modules (G3.7) |
| Behind one origin | `/mail`, `/docs`, `/crm` | a gateway, or a reverse proxy by path | | `gateways` forward by the token's audience claim only; a path prefix is declined (`docs/gateway.md:588-591`) (G3.8) |
| Deploy and operate | many modules, many hosts | servers, links, zones, monitors per deployment | smoke, end-to-end, data checks; monitors with responses | per module; the estate's map is a document (`docs/estate.md:154-166`); one run across the estate for generation, checks and CI is not there (G3.10) |

### Gaps, job 3

| Gap | What is missing | Severity | Smallest close | Release | Source |
|---|---|---|---|---|---|
| G3.1 | A tenant scope said once: the caller property, the key every entity carries, every role bound by it unless an entity is marked shared; and the validator refusing an entity with the key and no rule | blocks, as a safety matter: the shorthand was left out "until a specification is long enough to want it", and this one is; with seven modules the rules are roles times entities lines, and an entity left out is reached whole | a `tenant` object of the design: `property: organisationId` (a caller property), `key: organisationId` (the field), `shared: [Plan, Country]`; the validator derives `rows` for every role and every entity that carries the key, refuses an entity with neither the key nor a place in `shared`, and the derived cases follow; `generate sql` writes the policies from it | 0.7.x, an addition; owner decision, since the design declined it | `docs/meta-model-0.3.md:134-137`, `298`; `docs/conventions.md:782`; `internal/validate/caller.go:130-141`; `internal/gensql/rows.go:12-27` |
| G3.2 | Per-tenant settings and plan entitlements: a feature on for one tenant's plan, a limit per tenant | workaround: a `TenantSettings` entity and a `plan` claim in the token filling `caller.plan`, then `when: caller.plan == "business"` on page actions; operations have no `when`, so the server's check is a guard or hand-written | `enabledBy` taking a caller property beside a setting (`enabledBy: caller.featureX`), derived `disabled for <caller property>`; `limits.rate` per tenant as `per: caller.organisationId` | 0.7.x | `docs/conventions.md:2004-2018`, `1375-1379`; schema line 2688 |
| G3.5 | The isolation model: shared schema with row security, schema per tenant, database per tenant; and row security on engines other than PostgreSQL | workaround: PostgreSQL with row security, which is written; elsewhere the service's own filter alone | a `tenancy` setting on the `sql` target (`rowSecurity`, `schemaPerTenant`) with the generator writing the second; the mapping for the caller property is there already | 0.7.x | `internal/gensql/rows.go:97-112`; `docs/meta-model-0.3.md:124-131` |
| G3.6 | Generation across the estate: a page or a picker reading another module's list, a field typed by another module's schema | blocks for those screens: refused as `reference` by every code target | the estate given to `generate`: the UI generator reads the referenced specification for the columns and the dependency's base URL from the deployment, and writes the picker's remote list; the OpenAPI generator writes the referenced schema inline marked `x-specarch-ref`, as the design already names | 0.7.x; the design names it as waiting | `docs/estate.md:168-178`; `docs/conventions.md:1192-1200`; `spec/tests/generate-openapi-reference-refused/case.yaml:1-4` |
| G3.7 | A shared shell: one menu across modules, one theme, one strings file, an application switcher, sign-in once | workaround: a `shell` module whose menu entries name pages of other modules cannot be written (an entry names a page of its own specification, schema line 6140); each module carries the same theme by copy | `<dependency>#/pages/<name>` as a menu entry target and in `flows`, and the estate file naming the shell module whose `theme` and `menus` the others inherit | 0.7.x | `docs/conventions.md:1144-1200`; schema line 6140 |
| G3.8 | A gateway that routes by path prefix or header, and rewrites the path | workaround: a reverse proxy outside the specification, or one host per module with the token's audience naming it; the Next.js server routes forward to one service (`settings.server.service`), so a suite front end calling seven services needs a proxy anyway | `prefix` beside `claim` on a gateway's forwards, which the design says the element's shape admits "when a system asks"; `settings.server.services` as a map | 0.7.x | `docs/gateway.md:588-593`; `docs/generators.md:929-939` |
| G3.9 | A page kind for a screen built by hand (an editor, a live view) that still has a route, a permission, a menu entry, flows and the derived guard test; and interfaces beyond HTTP, messaging and the command line | blocks for the editors: they are built outside SpecArch, and today cannot be named in the design | `kind: custom` on a page: route, title, permission, optional entity, no generator output but the route, the guard, the menu entry and the derived test; a WebSocket channel stays unplanned until asked (`docs/roadmap.md:361-363`) | page kind: 0.7.x; WebSocket: owner decision | schema line 3053; `internal/genui/genui.go:209-226` (an unknown kind is a warning and left out); `docs/dxlib-lessons.md:341` |
| G3.10 | One run across the estate: generate every module, run every `check`, share idioms, and estate-level requirements | nice to have | `generate` and `check` given the estate file, iterating the modules in dependency order; an estate `idioms` folder; `requirements` in the estate file with `fulfilledBy` into modules | 0.7.x | `docs/conventions.md:1202-1231`; `docs/estate.md:443-455` |
| G3.11 | Search across modules; a suite-wide audit log a tenant's administrator reads | nice to have: `audited: true` gives every record its four fields; a central log is an entity fed by `emits` | an `auditLog` dependency kind, or a `securityEvents`-like `auditEvents` with a store, derived per write operation | later | `docs/conventions.md:1345-1355`; `docs/dxlib-lessons.md:344` |
| G3.12 | Per-tenant encryption keys and data residency | nice to have: `atRest: encrypted` is per field, the key an idiom's | a `keyPer: tenant` on the encrypted-column idiom | later | `docs/conventions.md:1323-1344` |

What the owner does by hand for job 3 today: every rows rule, the
editors and anything live, the cross-module pickers, the shell, the
proxy, the handlers (C1), the harness (C2), the images and CI (C3), and
the per-tenant entitlement checks on the server.

## Assumptions

- A1. The design tool exports its variables in the Design Tokens
  Community Group format, with or without a plug-in. Not verified here;
  if it does not, G1.1's token half needs a conversion first.
- A2. Job 2's provider offers OpenID Connect with the authorisation
  code flow and PKCE, and the SAML customer accepts a later release.
- A3. Job 3's tenant is the `organisationId` the meta-model's example
  uses, carried as a claim and filling the caller property; a tenant
  switch within one session (a person in several tenants) is a new
  token, not a new keyword.
- A4. Job 3 runs on PostgreSQL, so row security is written; on another
  engine G3.5 becomes a workaround for every entity.
- A5. The office applications are built as web screens on the Next.js
  generator or by hand, not on the plain JavaScript generator, which
  writes no server routes.

## Left out

The three jobs are described in generic words only. Which customer or
product asked for each, any name, data or internal detail of theirs,
and the owner's own applications are not in this file.
