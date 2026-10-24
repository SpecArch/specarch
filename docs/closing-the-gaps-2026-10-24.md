# Closing the gaps, 2026-10-24: from a specification to a deployed system

The owner's working model: a system analyst and a system designer write
the specification; an agent fills in what is left, generates the code,
runs the tests and takes the system to deployment, with no step done by
hand between. This file follows the capability check of the same day
(`docs/capability-check-2026-10-24.md`, 32 gaps for a website from a
design tool's file, an information system with single sign-on, and a
multi-tenant office suite) and does three things: it keeps only the gaps
that block that pipeline and says in which order they close and why
(section 1); it names the task that closes each, in that order (section 2); and it
proves, with the toolchain as it is today, how far the smallest example of
each job goes and where it stops (section 3). The owner's open questions
are in section 4. "Today" is the main branch at 0.7.6-dev, commit
d1cfbb41, built and run on this machine; every claim cites a file and
line, a command's output, or is marked as an assumption. Nothing was
deployed anywhere: the proof is scratch folders, a compile against the
local dxlib checkout and the repository's own round-trip test.

## Conclusion

The pipeline exists end to end today for a system made of lists, forms,
views and task pages on Go with dxlib. On the two-tenant example it ran
`validate`, `gaps`, `document`, `approve`, `generate` for sql, openapi,
go-dxlib and tests, the generated service compiled against the local
dxlib checkout once the agent wrote the one handler body the compiler
named, and the repository's round-trip test ran a generated service and
client under dxlib (section 3). What the agent writes by hand on that
path is bounded and named by a compiler, a generator warning or a
document: handler bodies the design gives no derivation for, the test
bodies of drafts, the harness, the service's `main`, the container recipe
from the deployment's `build`, and the release and commissioning records.
None of that blocks the model.

Six gaps block it, because the specification cannot name the thing, a
verb refuses it, or the output is wrong by omission with nothing said.
In the order they close, one task following for each:

| Order | Gap | Job | Blocks where |
|---|---|---|---|
| 1 | G3.1 tenant scope per role and entity, fail-open by omission | 3 | the specification: an entity left out is reached whole, no error |
| 2 | G3.1b the generated dxlib service does not apply the rows rules (new finding) | 3 | the generated code: the policies are written, the service answers every tenant's rows, no warning |
| 3 | G1.2 no content page | 1 | the specification: the site cannot be written |
| 4 | G2.1 no sign-in flow through an identity provider | 2 | the specification and the tests: the flow and its red cases are outside it |
| 5 | G3.7 no page kind for a screen built by hand | 3 | the specification: the editor has no route, guard, menu entry or test |
| 6 | G3.4 cross-module screens refused | 3 | `generate`: `reference` error in every code target |

The smallest example of each job, today: the one-page site stops at the
specification, since `kind: content` is refused by the schema, and only
its contact form goes through to generated screens. The login with
single sign-on goes through to a compiled service and generated tests,
but the flow itself (the redirect, the state, the nonce, the identity
token's checks) is outside the specification, so what the agent writes
for it is untraced and untested by anything derived. The two tenants go
through to a compiled service and a running round trip, but the
generated service does not apply the rows rules, so the isolation holds
only where the agent writes it by hand. Section 3 has the commands and
the lines.

The owner's questions (section 4): who approves in this model and
whether a draft build may go to a test environment (Q1); whether the
tenant shorthand the meta-model deferred is wanted now (Q2); OpenID
Connect first and additive only, SAML later (Q3); whether an interface
beyond HTTP is planned for the editors (Q4); whether a container recipe
should be generated from the deployment (Q5). No new generator language
and no GPL tool is needed by any task.

## 1. The filter: what blocks the pipeline

The capability check's "blocks" meant the owner does that part outside
SpecArch. This file's filter is narrower and different: the pipeline
`specification -> filled specification -> generated code -> tests ->
deployable artifact -> deployment record`, run by an agent that may
write code but must not have to invent what the specification could not
say. A gap blocks when one of these holds:

- **(a) The specification cannot name it.** What the agent writes for
  it has no element: no pointer to trace it to, no permission or guard
  derived for it, no derived test, no line in the techspec or the manual.
  The result is outside the specification, which is the one thing the
  model forbids.
- **(b) A verb refuses it or has nothing to write.** The pipeline stops
  at that verb with an error, or runs and leaves the thing out.
- **(c) The output is wrong by omission with nothing said.** The pipeline
  runs green and the system is unsafe. `docs/principles.md`: a problem is
  marked at the entry, never silent.

A gap does not block when:

- **(d) The hole is bounded and named.** The specification names the
  thing and SpecArch leaves a hole a compiler, a generator warning or a
  document names: `body<Operation>` for a handler the design gives no
  derivation for (`docs/generators.md`, Go for dxlib), `newHarness` and
  `body<Test>` for the tests (`docs/generators.md`, Tests from the
  specification), a container recipe from the deployment's `build`,
  `method` and `infrastructure` (`docs/stages.md`, Deployment). The agent
  writes it where it is named, and the rest of the pipeline checks it.
- **(e) A workaround writes more than it should**, inside the
  specification.
- **(f) It costs review time or a later change** (the check's "nice to
  have").

The 32 gaps under that filter:

| Gap | Check | Here | Why |
|---|---|---|---|
| G3.1 | blocks | **blocks (c)** | an entity carrying the tenant key with no rows rule is reached whole and nothing warns (`docs/conventions.md:782`); with seven modules the agent writes roles times entities lines and the validator cannot tell an omission from a choice |
| G3.1b | not listed | **blocks (c)** | new, section 3: the generated service lists every tenant's rows with the policies written and no warning |
| G1.2 | blocks | **blocks (a, b)** | `content is not allowed here` (section 3); the site has no element and the generators nothing to write |
| G2.1 | blocks | **blocks (a)** | the redirect cannot be written (`is not a page of the specification`, section 3); no derived case for state, nonce, issuer or the code |
| G3.7 | blocks | **blocks (a)** | an editor screen has no page kind, so no route, guard, menu entry, flow or derived test; the same refusal as G1.2 |
| G3.4 | blocks | **blocks (b)** | `reference` error in every code target (`spec/tests/generate-openapi-reference-refused/case.yaml`) |
| G1.1 | blocks the import | (d) | the analyst and designer type the tokens and the frames into the specification, as the check's workaround says; the agent can read an export too; nothing in the pipeline refuses it |
| G1.3 | workaround | (e) | typography, shadows and borders in the stack's CSS, beside the generated theme |
| G1.4 | workaround | (e) | assets committed beside the generated code; with G1.2 an image block names a path |
| G1.5 | owner decision | (e) | geometry stays in the stack by D1 and D5 |
| G1.6 | nice to have | (f) | a visual check is a commissioning check with prose steps today |
| G1.7 | workaround | (e) | metadata and a sitemap are the project's files; the plain JavaScript output is static already; a follow-up after the third task if the owner wants it |
| G1.8 | workaround | closes with the third task | every block's text a string key |
| G1.9 | by design | (e) | the idiom is overridden with the library built from the design |
| G2.2 | workaround | (e) | a `signOut` operation and a page action, written in the specification by hand; a follow-up after the fourth task |
| G2.3 | nice to have | (f) | |
| G2.4 | by design | (d) | the cookie and CSRF are the generated server routes' and the project's `getSession` |
| G2.5 | wording | (f) | |
| G2.6 | workaround | (e) | an algorithm with worked examples maps the claim; the harness applies it |
| G2.7 | nice to have | (f) | |
| G3.2 | workaround | (e) | a settings entity and a `plan` claim filling a caller property; `when` on page actions |
| G3.3 | workaround | (e) | PostgreSQL with row security (A4); elsewhere the service's filter, which the second task writes |
| G3.5 | workaround | (e) | the theme carried by copy; a shell module cannot name other modules' pages, which is a follow-up with the sixth task |
| G3.6 | workaround | (e) | a reverse proxy in the deployment, outside the specification |
| G3.8 | nice to have | (f) | the agent runs `generate` and `check` per module in dependency order |
| G3.9 | nice to have | (f) | |
| G3.10 | nice to have | (f) | |
| C1 | known | (d) | on Go with dxlib the list, create and read handlers are generated and every other body is named by the compiler (section 3); on another stack the handlers implement the generated interface and the compiler refuses a wrong signature (`docs/generators.md`, Server code on other stacks) |
| C2 | known | (d) | `newHarness` and the `body<Test>` of every draft are named in the generated file; a design test with `fixture`, `input` and `expect` is complete without them |
| C3 | by design | (d) | `build` names the task, the base images by digest and the SBOM task (`schema/specarch-implementation-0.1.schema.json:1407-1465`); the recipe the task runs is the project's, written from it; Q5 asks whether SpecArch should write it |
| C4 | workaround | outside the chain | the acceptance loop is the next round, after the deployment record |
| C5 | nice to have | (f) | only for a job that starts from existing screens |
| C6 | nice to have | (f) | each job is the first real screen |

Two more things were checked for the chain's last two steps, since the
check was per job and might have missed a step with no verb at all:

- **Deployable artifact.** The implementation file's `deployments` give
  `build` (the task, the base images by digest, how a build is compared
  with the vendor's, the SBOM task), `method` and `infrastructure`, and
  `tasks` holds the build, test, scan and CI commands
  (`schema/specarch-implementation-0.1.schema.json:169-178`, `1407-1465`).
  The artifact is made by running the named task; SpecArch writes no
  container file (C3). Not a block: the recipe is the project's, derived
  from facts the specification holds. Q5.
- **Deployment record.** The record kinds are change, defect, release,
  incident, commissioning and approval (`schema/specarch-record-0.1.schema.json:13-25`).
  A release record holds the version, `status` (planned, released,
  withdrawn), `date`, what it `includes`, the `specificationVersion`, the
  `implementations` versions, the `commissioning` runs that accepted it
  and the `commit` it was built from (`schema/specarch-record-0.1.schema.json`, the `release` definition from line 517);
  a commissioning record holds the environment, the date, the version,
  the `build`, the operator, the results per check and the sign-off
  (`docs/maintenance.md:308-313`), checked against the checks that exist
  (`spec/tests/validate-commissioning-record`). Together they are the
  deployment record: which build of which version went to which
  environment, when, with what result, accepted by whom. Not a block;
  the agent writes both files.

### The order, and why

1. **G3.1, then G3.1b** (the first and second tasks): the first entity of job 3,
   and a safety matter; a pipeline that runs green and leaks across
   tenants is worse than one that stops. The shorthand closes the
   omission at the source (an entity with the key and no rule becomes an
   error); the generator change makes the generated service honour the
   rules the specification states. The shorthand goes first so that the
   generator derives from one set of rules, written or derived, rather
   than two paths. Q2 is the owner's.
2. **G1.2** (the third task): job 1 is nothing without it, and it is an addition
   with no question attached.
3. **G2.1** (the fourth task): job 2's proof. Later than G1.2 only because Q3 is
   open (OpenID Connect first, additive only) and the design is larger.
4. **G3.7** (the fifth task): met at the first editor screen of job 3, after the
   first entity and the first module; small, additive, and the same
   refusal as G1.2, so it reuses that work.
5. **G3.4** (the sixth task): met at the first screen across modules, which comes
   after the first module is whole; a generator change only, Go only.

The six tasks are serial: each starts after the one before has landed, so a
schema change lands and its tests pass before the next starts. Every
schema change is additive to meta-model 0.3, a 0.7.x release by the rule
of `docs/maintenance.md:176-190`; the fourth task stops and asks if its design
finds that only a breaking change of the page event works.

## 2. The tasks

Each task says what done looks like, the conformance cases that
prove it, the job it unblocks, and the question it waits on, if any.

| Task | Closes | Job | Kind of change | Waits on |
|---|---|---|---|---|
| 1 | G3.1 | 3 | schema (additive), both validators, `derive`, `generate sql`, techspec; new `spec/tests/` cases | Q2 |
| 2 | G3.1b | 3 | `generate go-dxlib`: first a warning at every handler that does not apply a rows rule, then the rule applied; an ADR on how the caller's properties reach the query on dxlib; the round-trip test gains a two-tenant fixture | nothing |
| 3 | G1.2, G1.8 | 1 | schema (additive), both validators, the three UI generators, `derive`, techspec, manual; new cases | nothing |
| 4 | G2.1 | 2 | schema (additive), both validators, `derive`, the three UI generators (the redirect), go-dxlib (the callback stays `body<Operation>`), techspec; new cases | Q3 |
| 5 | G3.7 | 3 | schema (additive), both validators, the three UI generators (route and guard, the screen the project's), techspec, manual; new cases | nothing (Q4 is the WebSocket half, not this task) |
| 6 | G3.4 | 3 | `generate` given the estate: openapi, sql, go-dxlib and the UI generators resolve a cross-module reference; `examples/estate` gains a page across modules; new cases | nothing |

No task follows, on purpose, for G1.7 (site metadata and a static setting), G2.2
(sign-out), G2.6 (claims to roles), G3.5 (a shell module) and G3.8 (one
run across the estate). Each is a workaround inside the specification
today and a natural follow-up once its blocking task has landed; they
come when the job reaches them.

## 3. End-to-end proof, per job

Three scratch specifications, each the smallest example of its job, run
through the toolchain built from this commit (`go build ./cmd/...`, the
plug-ins on PATH). The folders are not in the repository: a failing case
belongs in a document, and `specarch validate spec examples` and CI's
documents check would pick up anything under `examples/`. The YAML
quoted here is what was run; paths are shortened to the scratch
folder's name.

### The path every job takes

What an agent runs, in order, as the three examples did:

1. `specarch validate <folder>`: errors and warnings at their entries;
   the validator proposes the derived cases it misses as test entries,
   line by line, which the agent copies in or `derive` writes as drafts.
2. `specarch gaps <folder>`: the open questions, and the outputs table,
   which says what waits on the approval.
3. `specarch decide <ID> --by <role> --answer ... --source ... <folder>`
   for each question answered by the stakeholder who holds `decidedBy`;
   the agent writes the entries the command prints.
4. `specarch document <target> <folder>` for every document target; the
   stakeholders read them.
5. `specarch approve --by <role> <folder>`: refused until no `must` or
   `should` question is open, there is no error, and every configured
   document on disk is current; then `records/approvals/<version>.yaml`.
6. `specarch generate <target> <folder>` for every code target. Before
   the approval, `--unapproved` writes a marked draft, and a draft's
   migrations are refused after the approval: "start a new folder for
   the approved one" (the two-tenant run below).
7. The implementation file's tasks: build, test, the SBOM scan; the
   harness, the bodies and `main` written where the compiler and the
   generated file name them.
8. The deployment's `build` task makes the artifact; the release record
   and, after the checks run in the environment, the commissioning record
   are written under `records/`.

### Job 1: a one-page site with a contact form

The specification: one content page of a heading, a paragraph and a
link, and a contact form posting an enquiry.

    pages:
      home:
        kind: content
        title: Welcome
        route: /
        permission: public
        blocks:
          - { heading: Who we are }
          - { text: A small studio that builds things. }
          - { link: contact, label: Write to us }
      contact:
        kind: task
        title: Contact us
        route: /contact
        permission: public
        submit: sendEnquiry
        fields: [email, message]
        onSubmitted:
          "201": { navigate: home, message: "Thank you, we will write back." }

`specarch validate job1/site-content`:

    job1/site-content/specarch.yaml:31:11: error: /pages/home/kind: schema: content is not allowed here; use one of form, list, task, view [schema@specarch.yaml#/pages/home/kind]
    job1/site-content/specarch.yaml:35:5: error: /pages/home/blocks: schema: blocks is not a key this object can have; remove it or correct its spelling [schema@specarch.yaml#/pages/home/blocks]

**Stops at the specification** (G1.2). With the content page taken
out, the rest goes through: the validator proposed the four tests of
`sendEnquiry` (a golden one, `missing email`, `missing message`, `email
not a valid email`) and the two of the page, line by line; with them
written, `validate` passed with no warning, and `specarch generate ui
--unapproved job1/site` with a `ui` target of platform `web`, framework
`plain-javascript` and `language: en` wrote six files, `events.js`,
`form.js`, `pages.js`, `service.js`, `transport.js` and `theme.css`,
with the one page, `contact`, whose HTML page the project writes. That
is a form on a static host, not a site. One authoring lesson on the way: a request body that `required`
the entity's read-only `id` was refused by the generator at the page
("whose request body requires id, which the page does not show"), which
is the specification's mistake and was fixed there.

| Step | Today |
|---|---|
| specification | stops: no content page |
| filled specification | n/a |
| generated code | the form's screens, plain JavaScript; the server behind `sendEnquiry` is a Go service on dxlib or a Next.js server route, generated as in job 2 |
| tests | derived for the form and the operation |
| deployable artifact | static files, with the project's HTML page |
| deployment record | release and commissioning records, written by hand |

### Job 2: a login through an identity provider

The specification: the provider as a dependency of kind
`identityProvider`, the callback as an operation of this system taking
`code` and `state` and calling the provider, the access token this
system accepts with `iss` and `aud` fixed, a web client proving itself
with that token, security events, a session, and one list page behind a
permission.

    dependencies:
      identityProvider:
        description: The organisation's identity provider, which signs people in and issues the identity token.
        timeout: PT5S
        kind: identityProvider
        protocol: https
    tokens:
      access:
        format: jwt
        lifetime: PT15M
        signature: ES256
        claims:
          iss: { const: https://idp.example.org }
          sub: { description: The person's identifier. }
          aud: { const: https://desk.example.org }
          client_id: { description: The application the person signed in through. }
          personId: { type: string, maxLength: 36, description: The person signed in. }
        default: true
    clients:
      desk-web: { description: The desk's web front end., kind: application, proof: { kind: token, token: access } }
    securityEvents: { store: eventStore, alarm: alarmChannel }
    session: { description: ..., idleTimeout: PT30M, absoluteTimeout: PT12H }
    paths:
      /auth/callback:
        get:
          operationId: finishSignIn
          permission: public
          callers: [people]
          calls: [identityProvider]
          parameters:
            - { name: code, in: query, required: true, schema: { type: string, maxLength: 512 } }
            - { name: state, in: query, required: true, schema: { type: string, maxLength: 128 } }
          responses: { "200": ..., "502": ..., "504": ... }
    pages:
      sign-in:
        kind: task
        title: Sign in
        route: /sign-in
        permission: public
        submit: finishSignIn
        fields: []
        onSubmitted:
          "200": { navigate: https://idp.example.org/authorize, message: You are signed in. }

`specarch validate job2/sso-stop`, the lines about the sign-in page:

    job2/sso-stop/specarch.yaml:96:13: error: /pages/sign-in/submit: page: finishSignIn takes no request body, and a task page's fields are properties of the body it sends; submit to an operation that takes one [page@specarch.yaml#/pages/sign-in/submit]
    job2/sso-stop/specarch.yaml:99:26: error: /pages/sign-in/onSubmitted/200/navigate: flow: https://idp.example.org/authorize is not a page of the specification; use one of cases-list, sign-in [flow@specarch.yaml#/pages/sign-in/onSubmitted/200/navigate]

And the derived cases the validator asked tests for, on the callback:
`dependency fails identityProvider`, `dependency times out
identityProvider` and a golden case; on the list: `denied with expired
access`, `denied with expired session`, `denied without cases.read`; on
the page: the session and permission refusals. Nothing for the state,
the nonce, the issuer, the signature, the token's expiry on the way in,
or a code used twice.

**Stops at the specification for the flow** (G2.1): a page cannot leave
the application, and the checks that make the flow safe are not
derived, so what the agent writes for them is untraced and untested by
anything the specification produced. The rest goes through. With the
sign-in page taken out and the ten proposed tests written, `validate`
passed; `generate openapi`, `generate go-dxlib` and `generate tests`
with `--unapproved` wrote the document, `specarch_dxlib.go`,
`specarch_dxlib_client.go` and `specarch_design_test.go`. The generated
handler:

    // FinishSignIn handles finishSignIn.
    func FinishSignIn(aepr *api.DXAPIEndPointRequest) error {
        r, err := readFinishSignInRequest(aepr)
        ...
        return bodyFinishSignIn(aepr, r)
    }

`go build ./...` on the two generated files against the local dxlib
checkout (a `go.mod` with a `replace`, as the round-trip test writes
one) named one symbol: `undefined: bodyFinishSignIn`. That body is the
whole exchange: the code for the token, PKCE, the state, the nonce, the
identity token's checks, the session. The ten generated tests each call
a `body<Test>(t, h)` the project writes, since the tests were the
validator's one-line proposals with no `fixture`, `input` or `expect`;
the harness's stub of the provider is `testing.mocks` prose.

| Step | Today |
|---|---|
| specification | stops for the flow; the resource-server side is complete |
| filled specification | `gaps`, `decide`, `approve` as in job 3 |
| generated code | the service compiles with `bodyFinishSignIn` written by the agent, outside the specification |
| tests | derived for the token, the session, the permission and the dependency; none for the flow |
| deployable artifact | the `build` task; the recipe the project's |
| deployment record | release and commissioning records |

### Job 3: two tenants with isolated data

The specification is `spec/tests/validate-0-3-rows-valid/specarch.yaml`
with a title, `stages` and a stakeholder added: a `Loan` carrying
`organisationId`, the caller with `organisationId` and `memberId`, a
`member` role bound by `memberId == caller.memberId` and a `staff` role
bound by `organisationId == caller.organisationId`, a list, a create, a
read, a job, and the sixteen tests the rules derive. The implementation
file names the sql (PostgreSQL), openapi (dxlib dialect), go-dxlib and
tests targets and, for the approval, a techspec document.

The whole path ran:

    $ specarch validate job3/tenants
    specarch: 1 input checked: 0 errors, 0 warnings
    $ specarch gaps job3/tenants            # the outputs table, in part
    | code target go-dxlib | waits | not approved: there is no records/approvals/1.0.0.yaml beside the specification |
    $ specarch generate sql --unapproved job3/tenants
    specarch generate: 2 files written, 0 files already current
    $ specarch document techspec job3/tenants
    specarch document: 1 file written, 0 files already current
    $ specarch approve --by owner job3/tenants
    specarch approve: version 1.0.0 of job3/tenants approved by owner; records/approvals/1.0.0.yaml written beside it
    $ specarch generate sql job3/tenants
    job3/tenants/specarch.yaml:1:1: error: /: generator: 0001_expand.sql was written by a draft, and the specification is approved now; a draft's migrations are not the approved schema, so start a new folder for the approved one [generator@specarch.yaml#/]
    $ specarch generate go-dxlib job3/tenants
    specarch generate: 2 files written, 0 files already current

The migration written by the draft, `0001_expand.sql`, lines 11 to 13:

    ALTER TABLE loan ENABLE ROW LEVEL SECURITY;
    CREATE POLICY rls_loan_member ON loan FOR ALL TO member USING ((member_id = CAST(NULLIF(current_setting('specarch.caller.member_id', true), '') AS VARCHAR(36)))) WITH CHECK (...);
    CREATE POLICY rls_loan_staff ON loan FOR ALL TO staff USING ((organisation_id = CAST(NULLIF(current_setting('specarch.caller.organisation_id', true), '') AS VARCHAR(36)))) WITH CHECK (...);

The generated service's list handler, `specarch_dxlib.go:164`:

    return Tables.Loan.RequestSearchPagingList(aepr)

Nothing in the two generated files sets `specarch.caller.organisation_id`
or `specarch.caller.member_id` on the connection, and nothing runs as
the database role `staff` or `member`: `grep -n 'current_setting\|set_config\|organisation' specarch_dxlib.go`
finds only the request struct's `organisation_id` parameter. The
service's own connection is the table's owner, which row security does
not bind, by the generator's own words (`internal/gensql/rows.go:12-27`:
"the table's owner is not bound, so the service's own connection reaches
it whole too"). So the generated system answers every tenant's loans to
any staff, and no warning says so. **That is G3.1b**, the finding of
this check, and it is condition (c): a green pipeline and an unsafe
system. The generated tests for `shows only the rows of staff` and the
others are `body<Test>` stubs, since the design's tests give no
`fixture`, `input` and `expect`; the harness would be where the agent
wires the tenant, by hand.

`go build ./...` on the two generated files against the local dxlib
checkout named one symbol, `undefined: bodyCreateLoan` (this example's
`id` is not read-only, so the create is the service's); with a one-line
body it compiled. And the repository's own round trip, which generates
a service and a client from a fixture, binds the service to a real dxlib
API through the document the dxlib dialect writes and drives the client
through it (`internal/gendxlib/roundtrip_test.go`), ran with
`SPECARCH_DXLIB` naming the local checkout, under `heavy-lock`:

    --- PASS: TestEnvelopeStreamRoundTrip (3.71s)
    --- PASS: TestSessionV3RoundTrip (2.26s)
    --- PASS: TestSessionV4RoundTrip (2.10s)
    --- PASS: TestExtractedRoundTrip (2.21s)
    ok      github.com/SpecArch/specarch/internal/gendxlib  10.482s

So a generated service runs and answers a generated client today, and
the two-tenant service compiles; what nobody has run is the two-tenant
service against PostgreSQL with the policies, and section 1's reading
of the generated code says it would not isolate. The second task's
round-trip fixture is the test that will say so.

| Step | Today |
|---|---|
| specification | goes through, with one rows rule per role and entity (G3.1) |
| filled specification | goes through: `gaps`, `document`, `approve`, the gate on `generate` |
| generated code | goes through and is wrong by omission: the policies are written, the service does not apply them (G3.1b) |
| tests | derived and generated; their bodies the agent's, as drafts |
| deployable artifact | the `build` task; `main`, the recipe and PostgreSQL the project's |
| deployment record | release and commissioning records |

## 4. Questions for the owner

- **Q1. Approval in the working model.** `generate` refuses without
  `records/approvals/<version>.yaml`, which `specarch approve --by
  <role>` writes for a stakeholder role, never for the agent
  (`docs/refinement.md`, Approval; ADR-019); `--unapproved` writes a
  draft marked on every file and screen, and a draft's migrations are
  not the approved schema. Two choices: whether the analyst's or
  designer's approval is given once per round through the dispatcher
  (the agent runs `approve --by system-analyst` on their word, recorded
  as such), and whether a draft build may be deployed to a test
  environment for the user acceptance test, with production only from
  an approved build. Recommendation: yes to both; it is what
  `docs/acceptance.md` designs.
- **Q2. The tenant shorthand** (the first task). Meta-model 0.3 left it out
  "until a specification is long enough to want it"
  (`docs/meta-model-0.3.md:134-137`). The suite is. Confirm, or drop
  the first task; the second stands either way.
- **Q3. Single sign-on** (the fourth task). OpenID Connect with the
  authorisation code flow and PKCE first, additive to 0.3; SAML as a
  later task when the customer who asked for it is scheduled. If the
  design finds that the page event's shape must change, that is 0.8.0
  and the task stops to ask.
- **Q4. An interface beyond HTTP** for the editors and live views
  (WebSocket, the second half of G3.7): `docs/roadmap.md:361-363` keeps
  it unplanned until asked. The fifth task gives the editor its page; the
  live channel stays the project's unless the owner asks for it.
- **Q5. A container recipe from the deployment** (C3). The
  implementation file holds the build task, the base images by digest,
  the settings and the endpoints; a `container` target writing a
  Containerfile and a compose file from them is derivable, which is
  where `docs/roadmap.md`'s "generation stops at what can be derived"
  draws the line. A new generator is the owner's call; until then the
  agent writes the recipe per project from those facts.
- **No GPL tool and no new generator language** is needed by any task:
  the generators stay Go, the validators Go and Swift.

## Assumptions

- A1 to A5 of the capability check hold here unchanged; A4 (PostgreSQL)
  is what makes G3.1b a generator matter rather than a dialect one.
- A6. The two-tenant service was compiled, not run against PostgreSQL;
  the claim that it does not isolate rests on reading the generated
  handler and the policy's `TO <role>` and `current_setting` clauses
  against the generator's own comment on the owner's connection. The
  second task's fixture turns it into a test.
- A7. The job 2 example takes the provider's `iss` as a constant of the
  accepted token; a provider with several issuers would use `enum`.

## Left out

The three jobs are described in generic words only; which customer or
product asked for each, and any name, data or detail of theirs, is not
in this file. The scratch
specifications are not committed.
