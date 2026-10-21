# Bug history

Fixed bugs, one entry per bug, oldest first. Open bugs are in
`BUG_OUTSTANDING.md`; a bug moves here when the commit that fixes it, with
`Fixes BUG-NNN` in its message, is on `main`.

BUG-001 to BUG-021 were tracked as GitHub issues #1 to #21 before the
repository stopped using GitHub issues. Each keeps its old number as
"was #N", so a commit message that says `Fixes #N` names BUG-N. Numbers 014,
016 and 020 are not used: GitHub gave them to pull requests.

## BUG-001 — Expression errors in a specification tree name specarch.yaml instead of the file they are in

- Was: #1 (GitHub issue)
- Reported: 2026-10-07 by donnyhardyanto
- Closed: 2026-10-07, completed
- Labels: none
- Fixed by: cba15ae4
- Comments: 0

In a specification spread over stage folders, `specarch validate` reports an expression error (`expression_syntax`, `expression_name`, `expression_type`) against the root file with the line of the file the expression is really in.

Example: a misspelt input in the formula of `spec/design/algorithms/late-fee.yaml` (line 11) of the library-lending example is reported as

    spec/specarch.yaml:11: error: /algorithms/lateFee/formula: expression_name: ...

It should name `spec/design/algorithms/late-fee.yaml:11`. Both builds (Go and Swift) do this: the expression checks write the diagnostic against the checker's own file instead of the file of the node.

## BUG-002 — permission_ungranted is hidden when the permission has no description

- Was: #2 (GitHub issue)
- Reported: 2026-10-08 by donnyhardyanto
- Closed: 2026-10-08, completed
- Labels: none
- Fixed by: 4260cd80
- Comments: 0

A declared permission that no role grants and that has no description gets no permission_ungranted error, in both validator builds.

The validator drops a diagnostic as an echo when a schema error sits at the same path, and the schema error "description is missing" sits at /permissions/<name>, the same path as permission_ungranted. A must question that blocks the description then covers the schema error, so nothing is reported and validate exits 0.

A tree written by `specarch extract router` has this shape: it declares every permission a route checks, with a must question asking what each allows, and has no roles, so its ungranted permissions were never reported.

Expected: an ungranted permission is always reported, unless a must question blocks that permission itself.

## BUG-003 — derive writes one draft where a page and an operation share a name

- Was: #3 (GitHub issue)
- Reported: 2026-10-09 by donnyhardyanto
- Closed: 2026-10-09, completed
- Labels: none
- Fixed by: bf5b09b9
- Comments: 0

When a page is named after an operation (page `sign-in` and operation `signIn`), both subjects get the test name `sign-in`. `specarch derive` writes `tests/sign-in-succeeds` for the first and then reports the second as "kept, which exists", so the other subject's golden draft is never written. The validator suggests the same folder name for both.

Expected: two subjects never get the same suggested test name, and a draft name that still collides is an error that names both subjects, not a skip.

## BUG-004 — Go and Swift name a number with no fractional part differently in a schema message

- Was: #4 (GitHub issue)
- Reported: 2026-10-09 by donnyhardyanto
- Closed: 2026-10-09, completed
- Labels: none
- Fixed by: 6e1a618b
- Comments: 0

A number where the schema expects another type is named "a number" by the Go build and "an integer" by the Swift build when it has no fractional part (1 or 1.0), so the two validators print different output for the same input. For example `title: 1.0` gives "this is a number, but a string is expected here" in Go and "this is an integer, but a string is expected here" in Swift. The two builds must print the same output.

## BUG-005 — document --check over spec and examples exits 2 at testplan

- Was: #5 (GitHub issue)
- Reported: 2026-10-09 by donnyhardyanto
- Closed: 2026-10-09, completed
- Labels: none
- Fixed by: 6e1a618b, 817bd304, a86e026b
- Comments: 0

The documents loop in CONTRIBUTING.md runs `specarch document <target> --check spec examples` for each target. examples/lending-desk configures only some documents, so testplan, deployment and commissioning stop with exit 2 ("no output folder for examples/lending-desk/spec") and the loop ends at testplan. A specification that names no output folder for a target should be passed over with a warning at its entry, and CI should run that loop so it cannot break silently again.

## BUG-006 — generate openapi drops the keywords written beside a $ref

- Was: #6 (GitHub issue)
- Reported: 2026-10-09 by donnyhardyanto
- Closed: 2026-10-09, completed
- Labels: none
- Fixed by: 85c30773
- Comments: 0

A field may carry keywords beside a `$ref` (a description, readOnly, a default, constraints, properties), as JSON Schema 2020-12 and OpenAPI 3.1 allow, and the design schema accepts them. `specarch generate openapi` writes only the `$ref` and drops every other keyword, in both the standard and the dxlib dialect, without a word. That breaks the rule that a definition is never silently dropped (docs/principles.md).

Expected: the standard dialect keeps every keyword beside the `$ref`. The dxlib dialect keeps what it can carry (the description, the constraints dxlib does not enforce under x-specarch-unenforced, SpecArch's extensions) and reports an error at the field for what one dxlib parameter cannot hold, rather than dropping it.

## BUG-007 — CI greps extract pages for a summary line it no longer prints, so the check job fails on every push to main

- Was: #7 (GitHub issue)
- Reported: 2026-10-12 by donnyhardyanto
- Closed: 2026-10-12, completed
- Labels: none
- Fixed by: 8f9a5558
- Comments: 0

The check job's step "The lending desk's app folder extracts the same twice, and merged with the database the loans entity's question is answered" has failed on every push to main since f5efe5ee (2026-10-09, the commit that reads the Pages Router, Nuxt's pages and server, route files and middleware), fifteen runs through 11346faf.

Cause: that commit taught `extract pages` route files and put an operations count into its summary line, so on `examples/lending-desk/sources/web/app` it now prints

    wrote 3 pages, 0 operations, 1 entity, 1 permission and 9 questions: ...

while `.github/workflows/ci.yml` still greps for `wrote 3 pages, 1 entity, 1 permission`. The grep exits 1 and the step with it. `extract pages` and `validate` both exit 0; the seven `test_*_missing` lines the step prints before the failure are warnings, not errors, and are not the cause. The design tests under `spec/tests/extract-pages-*` pin the wording with the operations count, so the workflow is what is stale.

Because the step fails early, no step after it has run in CI since f5efe5ee.

## BUG-008 — extract go writes 0 operations when a router is passed to a method of a struct made by another package's constructor

- Was: #8 (GitHub issue)
- Reported: 2026-10-14 by donnyhardyanto
- Closed: 2026-10-14, completed
- Labels: none
- Fixed by: 1129244b
- Comments: 0

`extract go` follows a router through the calls that pass it (docs/extraction.md step 15, ADR-081), but only when the callee is a plain function or a method on a value whose type the reader knows, and the reader knows a value's type only when it is made as `T{}`, `&T{}` or `new(T)`. A value a constructor returns has no type for the reader, so a router handed to a method on it is not followed:

    // cmd/main.go
    r := gin.New()
    server := rest.NewServer()
    server.RegisterRoutes(r)

    // internal/api/rest/server.go
    func (s *Server) RegisterRoutes(r *gin.Engine) { r.GET("/items", s.List) }

Expected: the operation `GET /items`.

Got: the reader counts the registrations but writes 0 operations; each route becomes a must question of the form "registers GET /items on a router whose place the reader does not know (server.go:N is given the router r, and the reader found no call that passes it one whose path it knows)". The same happens when the call sits inside a closure, such as a command's run function, and when the constructor returns `(*Server, error)`.

Fix: type a name from the result of the function or method of the module the call names, in the one-value and the multi-value assignment, so the method is found and the router bound.

## BUG-009 — extract documents gives an all-caps file name a source key with a dash between every letter

- Was: #9 (GitHub issue)
- Reported: 2026-10-14 by donnyhardyanto
- Closed: 2026-10-14, completed
- Labels: none
- Fixed by: 1129244b
- Comments: 0

`specarch extract documents --out t ARCHITECTURE.md` gives the default source key `a-r-c-h-i-t-e-c-t-u-r-e`. The kebab-case conversion puts a dash before every capital letter, so a name written in capitals splits into its letters. Two such documents from different repositories then collide at `specarch merge`, which asks to declare the source `a-r-c-h-i-t-e-c-t-u-r-e` differently.

Expected: a run of capitals is one word: `ARCHITECTURE.md` gives `architecture`, `README.md` gives `readme`, `FSD-Login.md` gives `fsd-login`, and `HTTPServer` still gives `http-server`. The same default names an OpenAPI document's source key.

## BUG-010 — extract go leaves out a body decoded into an inline struct, and its question blocks a pointer the tree does not have, so validate refuses the tree

- Was: #10 (GitHub issue)
- Reported: 2026-10-14 by donnyhardyanto
- Closed: 2026-10-14, completed
- Labels: none
- Fixed by: ec72ef32
- Comments: 0

`extract go` writes a handler's request body from the struct it decodes the body into (docs/extraction.md step 15), but only when the variable's type is a named type of the module. A variable declared with a struct type in place is not read:

    func (s *Server) Create(c *gin.Context) {
        var req struct {
            Name string `json:"name"`
        }
        if err := c.ShouldBindJSON(&req); err != nil { /* ... */ }
    }

registered as `r.POST("/items", s.Create)`. The same holds for `var body struct{...}`, and for `json.NewDecoder(r.Body).Decode(&req)`.

Expected: a request body `{type: object, properties: {name: {type: string}}}`, read by the rules a named struct is read by (json tags, nested struct types in place, pointers, slices, validator tags).

Got: the operation has no `requestBody`, and a should question "request body: an expression is a type the reader does not find in the module's files" whose `blocks` is `#/paths/~1items/post/requestBody/content/application~1json/schema`. `specarch validate` on the tree then reports `question_block: ... points below a key that does not exist`, and `specarch merge` refuses the tree, since each tree is checked on its own before it is merged. Two such handlers keep a whole tree of a service out of the merge.

The pointer is the second bug, and it is not only this shape's: a reader asks its question at the place the element would have had, then leaves the element out. A map as a struct's only field blocks `.../schema/properties/extra` while the object has no `properties`; a dropped property's own questions (an `int64`'s width, a slice's items) block pointers below the dropped key.

Fix: write a struct type in place through the same field loop as a named struct; and, as a rule of every tree a reader writes, a question's pointer is anchored in the tree the reader writes, at the deepest element that exists and at most one missing key under it, so that the tree passes `validate` on its own. The conformance suite runs `validate` on every extract case's output.

## BUG-011 — validate: a must question that blocks a written field or constraint by pointer does not cover the keys missing under it

- Was: #11 (GitHub issue)
- Reported: 2026-10-14 by donnyhardyanto
- Closed: 2026-10-14, completed
- Labels: none
- Fixed by: d2d2d612
- Comments: 0

A must question that blocks an element by its pointer covers the required keys missing at or under it (docs/conventions.md, Open questions). That holds for an element known only by name, an empty mapping, but not for one written out in part.

Repro on a copy of examples/lending-desk: write `Member.properties.fullName` as `{ minLength: 1, maxLength: 200 }` with no `type`, give `Book` `constraints: { book_title_rule: { message: "..." } }`, and add two must questions with `blocks: ['#/entities/Member/properties/fullName']` and `blocks: ['#/entities/Book/constraints/book_title_rule']`.

`specarch validate spec` reports `type is missing` and `$ref is missing` at fullName and `kind`, `fields` and `expression` missing at the constraint, 5 errors; expected 0. The pointer is read as a key of the mapping before it, so only the key named by its last token is covered, and the node it points to is covered only when it is an empty mapping. Blocking a key such as `.../fullName/type` leaves the oneOf partner `$ref is missing` and the keys a `kind` makes required; only blocking the whole entity clears everything, which holds back far more than the question is about.

## BUG-012 — validate: the type-rendering idiom check reports a field a must question blocks

- Was: #12 (GitHub issue)
- Reported: 2026-10-14 by donnyhardyanto
- Closed: 2026-10-14, completed
- Labels: none
- Fixed by: d2d2d612
- Comments: 0

With a field blocked by a must question (see the previous issue: `fullName` with its width and no `type`, `blocks: ['#/entities/Member/properties/fullName']`), `idiom_contract` still reports that type-rendering has no go and no postgresql row for the field. Expected: the idiom checks skip a field a must question blocks, by its pointer, its entity's or one of its keys, as the schema check does: its shape is not known yet.

## BUG-013 — validate: a citation's clause is not checked against the source's outline

- Was: #13 (GitHub issue)
- Reported: 2026-10-14 by donnyhardyanto
- Closed: 2026-10-14, completed
- Labels: none
- Fixed by: d2d2d612
- Comments: 0

`validate` refuses a `cites.source` that is not declared under `sources`, but accepts any `clause`, including one that names no section of the source's outline when the source lists its `clauses` (from `extract outline`, `extract documents` or a code reader). A misspelt clause is found only by hand, or by reading the gaps document's "cited outside the listed clauses" line. Expected: a warning when a source lists its clauses and a citation names a clause that is neither listed nor a part of a listed one (3.3.1 under 3.3, file.go:14 under file.go), or names no clause at all.

docs/from-sources.md, Not built yet, planned this rule once a real project showed whether outlines stay complete; the readers' own trees cite clauses their outlines leave out (a file set aside with a question, a section of an OpenAPI document that was left out), so the outlines are completed with what the tree cites.

## BUG-015 — merge writes both $ref and an inline schema at one position when an OpenAPI tree and a Go tree describe the same request body

- Was: #15 (GitHub issue)
- Reported: 2026-10-14 by donnyhardyanto
- Closed: 2026-10-14, completed
- Labels: none
- Fixed by: ea98a763
- Comments: 0

`specarch merge` joins two trees key by key: a key only one tree gives is kept, and a key both give with different values is left out with a must question (ADR-045). A schema position where one tree refers to a named schema and the other writes the shape in place falls between the two rules: `{$ref: '#/schemas/CreateThingRequest'}` and `{type: object, properties: {...}, required: [code]}` share no key, so every key is kept and the merged tree holds both forms.

Minimal input, with `specarch 0.6.2`:

- an OpenAPI document with `POST /v1/things` (operationId `createThing`) whose request body schema is `{$ref: '#/components/schemas/CreateThingRequest'}`, an object with `code` (required) and `name`;
- a gin module with `v1.POST("/things", createThing)`, whose handler does `c.ShouldBindJSON(&req)` into

      type createThingRequest struct {
          Code string `json:"code" binding:"required"`
          Name string `json:"name"`
      }

`extract openapi` and `extract go` each validate with 0 errors. `merge`, in either order, writes

    schema: {$ref: '#/schemas/CreateThingRequest', type: object, properties: {code: {type: string}, name: {type: string}}, required: [code]}

and `validate` reports `error: schema: this matches more than one allowed form; write only one of them (for a field, either $ref or type, not both)`. `gaps` then exits with a status above 1, so an extraction stops before the documents step. Every module whose service has an OpenAPI document and handlers with named request structs hits it once the Go tree is merged. The same class is open at every position merge joins two schema mappings: responses, parameters, array items and properties.

Expected: one form at the position. Where the named schema agrees with the shape written in place, the reference is kept and the operation cites both trees; where a key both give differs, the key is left out with a must question that names the difference, as any disagreement is; where the shapes agree and only list different properties, the reference is kept and a should question says what each lists. A question the inline tree asked below the position (an `int64`'s width, a map's shape) follows the reference to the named schema, so the merged tree validates with 0 errors.

## BUG-017 — merge writes a field with no type when two trees give it different types, and validate refuses the merged tree

- Was: #17 (GitHub issue)
- Reported: 2026-10-14 by donnyhardyanto
- Closed: 2026-10-14, completed
- Labels: none
- Fixed by: f3c205cf
- Comments: 0

Two trees, each valid on its own, give an entity's field different types: one `{type: boolean, default: false}` with the field required, the other `{type: [boolean, "null"]}` with it optional.

`specarch merge --out m a b` asks the must question "the trees disagree" on `#/entities/Thing/properties/flag/type` and writes `flag: {default: false}`, with no type. `specarch validate m` then reports `/entities/Thing/properties/flag: schema: $ref is missing; add it here`, and `gaps` and `document` stop.

Cause: a disputed key is left out (ADR-045), and the must question blocking the key covers the missing-key error for every other required key, but a field with neither `$ref` nor `type` fails the schema's one-of, whose error names `$ref`, so the question does not cover it.

Expected: the field keeps a valid schema (the union where the types differ only in whether null is allowed, else the first tree's type), the disagreement stays the must question, and the merged tree validates. More generally, merge's output always validates: where leaving a key out would leave an entry the schema cannot read, one valid form is kept and the question says which. The conformance suite should validate every merge result.

Found with 0.6.2.

## BUG-018 — extract go misses routes registered in a callback passed to a local helper closure

- Was: #18 (GitHub issue)
- Reported: 2026-10-14 by donnyhardyanto
- Closed: 2026-10-14, completed
- Labels: none
- Fixed by: f3c205cf
- Comments: 0

A gin service registers routes through a local helper that takes a prefix and a function:

    v1 := r.Group("/v1")
    v1.GET("/things", listThings)

    group := func(prefix string, reg func(g *gin.RouterGroup)) {
        g := v1.Group(prefix)
        reg(g)
    }
    group("/callbacks", func(g *gin.RouterGroup) {
        g.POST("/thing-create", createThing)
    })

`specarch extract go` counts 2 registrations and writes 1 operation; the POST lands as a must question, "registers POST /thing-create on a router whose place the reader does not know (Group is given the prefix prefix, which is not a literal)". Expected: both operations, `GET /v1/things` and `POST /v1/callbacks/thing-create`.

Cause: the walk follows calls of the module's functions with the routers they pass, but not a call of a function literal held in a local name, and it binds neither the literal strings nor the function values a call passes, so the helper's prefix is not a literal to it and the callback is never walked with the group.

And as a rule, a counted registration the reader cannot place is never dropped silently: it becomes a question at the nearest element.

Found with 0.6.2.

## BUG-019 — derive names a draft from a requirement ID with a dot, and validate refuses the folder name

- Was: #19 (GitHub issue)
- Reported: 2026-10-14 by donnyhardyanto
- Closed: 2026-10-14, completed
- Labels: none
- Fixed by: f3c205cf
- Comments: 0

In 0.6.3, `specarch derive` names each acceptance-test draft from the requirement's ID as written, lower-cased. An ID may hold a dot or an underscore, so a requirement `AB-1.2` with a harm and one acceptance criterion gives `tests/ab-1.2-acceptance-1/test.yaml`, and `specarch validate` then refuses the tree: the test name is not kebab-case.

Expected: derive makes the subject's name a valid test name (every run of characters other than small letters and digits one hyphen: `ab-1-2-acceptance-1`), keeps the ID itself in the draft's keys and `verifies`, and where two IDs give the same name after that (`AB-1.2` and `AB-1_2`), gives the later one a suffix and asks about it. More generally, every command that writes a tree writes one that passes validate, and the conformance suite checks it for every such case.

## BUG-021 — validate: the Go build reports a type list that lacks null at each item, the Swift build once at the list

- Was: #21 (GitHub issue)
- Reported: 2026-10-14 by donnyhardyanto
- Closed: 2026-10-14, completed
- Labels: none
- Fixed by: da2f6238
- Comments: 0

A field whose type is a list without "null", such as `type: [object, string]`, gets two extra errors from the Go build that the Swift build does not print:

```
/type/0: schema: this must be null
/type/1: schema: this must be null
```

The Go schema library keeps a failed `contains` as one cause per item, and the Go build reported every cause. The Swift build reports a failed `contains` once, at the list. Both builds must print the same output for the same input.

## BUG-022 — extract go: routes on a group a helper returns are not read

- Reported: 2026-10-18
- Fixed: 2026-10-19
- Fixed by: dda47fdd

A gin service registers routes in three ways: on a plain group, through a local helper that takes a prefix and a callback (read since 0.6.4, BUG-018), and through a local helper that takes a prefix and returns the group:

    returned := func(path string) *gin.RouterGroup {
        g := v1.Group(path)
        return g
    }
    {
        g := returned("/d")
        g.POST("/x", h)
        g.PUT("/:id/y", h)
    }

Expected 5 operations: 1 under the plain group, 2 under the callback helper's group and 2 under the returned group. `specarch extract go` writes 3: both routes on the returned group are missing, and no question or warning is raised for them.

Cause: the walk followed a call of a function value held in a local name, or bound to a parameter, only as a statement, never as a value. The router a call gives was a library's maker, a group or subrouter, or what a function of the module returns, so `g := returned("/d")` gave `g` no router, and the routes on `g` reached no registration, counted or asked. Beside it, a literal's `return` set the return of the function it is written in, so a function that declared such a helper and then returned its own group could hand a caller the helper's group instead.

Found with 0.6.4.

## BUG-023 — extract database: a partition is read as a second table

- Reported: 2026-10-18, from reading a system whose database has partitioned tables
- Closed: 2026-10-19, completed
- Fixed by: the commit on `main` whose message says `Fixes BUG-023`; ADR-128

`tools/catalogue/catalogue.sql` listed every relation of relkind `r` or `p` as a table and did not mark a partition, so a catalogue dump of a partitioned table held the table and each of its partitions as tables with the same columns, each partition with the primary key and the indexes it takes from its table under the names PostgreSQL gives them. `extract database` wrote an entity for each table and each partition, `Events` and `EventsDefault` for a table `events` with a default partition `events_default`, and asked its could question for each generated default and each index once per entity. `validate` reported no error, so nothing blocked; `check` would have reported each partition as a table the specification does not hold. The migration in `spec/tests/extract-database-partitioned-table/service/migrations/001_init.sql` repeats it against a dump made by the earlier query.

Expected: one entity for the table, the partitions folded into it and named at it, and one question for each default and each index.

## BUG-024 — a history entry names a private organisation

- Reported: 2026-10-19
- Closed: 2026-10-19, completed
- Fixed by: the commit on `main` whose message says `Fixes BUG-024`

The entry for the merge item in `history/2026-10-19.md`, added by commit ddde375c, called the handoff mailbox by the short name of a private organisation. The repository is public and holds generic material only, so no such name may appear in it.

Expected: the mailbox named generically, as every other history entry does ("The handoff mailbox folder was empty."), and no other private name or path anywhere in the tree.

The fix rewords the sentence in a normal commit. The old text stays in the history of commit ddde375c; rewriting published history is the owner's decision.

## BUG-025 — extract go: the parameters and responses an endpoint declares where it is registered are missed, leaving empty schemas silently

- Reported: 2026-10-19, from converting a Go service on dxlib to SpecArch
- Closed: 2026-10-19, completed
- Fixed by: the commit on `main` whose message says `Fixes BUG-025`; ADR-130

The service registers each endpoint with `NewEndPoint`, which is given the endpoint's parameter list (names, types, required flags, limits) and its response possibilities; the handler then reads its parameters through dxlib, not by name. `specarch extract go` read only the names in a literal parameter list, said the handler "reads no parameter by name", and wrote each operation with no body or parameters and no responses but the refusals its handler answers. The OpenAPI document the running service dumped had no schemas either, since its endpoints declared none, so `extract openapi` added nothing. Every operation of such a service ended up with empty inputs and outputs, and nothing in validate or the questions said so.

Expected: the declared parameters written as the operation's body fields or query parameters with their types, required flags, bounds and enums, and the declared responses with their descriptions and bodies; what cannot be followed a question naming the operation and where to look; and an operation left with a body of any object or with no success response a validate warning.

## BUG-026 — validate (Go build): only one of two empty response keys is reported

- Reported: 2026-10-19, found while reviewing the response key checks
- Closed: 2026-10-19, completed
- Fixed by: the commit on `main` whose message says `Fixes BUG-026`

When two operations each have a response keyed by the empty string, the Go build of `validate` reports the schema error of only one of them. `propertyNameObject` in `internal/validate/schema.go` resolves the location of both `propertyNames` errors to the same object, because the schema library reuses the location slice it reports, so the `seen[path|name]` check takes the second error for the first and drops it. The Swift build reports both, so the two builds print different output for the same input.

Expected: one schema error at each operation's empty key (`/paths/~1loans~1close/post/responses/` and `/paths/~1loans~1open/post/responses/`), from both builds.

To repeat: copy `spec/tests/validate-interface-response-key-empty/specarch.yaml`, add a second operation (`openLoan` on `/loans/open`, post) whose responses are `"": { description: Odd. }` and `"200": { description: Opened. }`, and run `specarch validate` on it. The Go build prints 1 error, at `closeLoan`; `swift run --package-path swift specarch validate` prints 2.

## BUG-027 — merge: a schema and an entity of one name are written side by side, and validate refuses the merged tree

- Reported: 2026-10-19, from merging a service's OpenAPI document with its database
- Closed: 2026-10-19, completed
- Fixed by: the commit on `main` whose message says `Fixes BUG-027`; ADR-132

An OpenAPI document declares a schema that no operation creates, so `extract openapi` writes it under schemas; the database holds a table of the same name, which `extract database` writes under entities. Each tree passes `validate` alone. `specarch merge` wrote `/schemas/Note` beside `/entities/Note`, asked no question and reported no disagreement, and `validate` refused the merged tree: "Note is the name of an entity too; a schema, an entity and a view are one namespace". A merged tree is not edited by hand, so the extraction stopped there. Seen with 0.6.4.

Expected: the merge keeps both entries and asks a must question at the schema whether the name is one element or two, citing both, and the merged tree passes `validate` with the question open. The same for a view named like an entity, and a schema like a view.

## BUG-028 — extract go: a route whose path is a package-level constant, or whose Match method list is a package-level list, is a question instead of an operation

- Reported: 2026-10-19, from reading a gateway service whose routes are registered through the module's own constants
- Design: `docs/gateway.md`, section 3; ADR-143
- Closed: 2026-10-20, completed
- Fixed by: the commit on `main` whose message says `Fixes BUG-028`; ADR-143

A Go module registers its routes on a gin router with the paths and the method list declared at package level. The fixture is this one file, `main.go`, complete, as the only file of a module committed in a repository of its own and read with `specarch extract go --out <folder> <repository>`; the line numbers cited here and in BUG-029 are its:

    package main

    import (
    	"net/http"

    	"github.com/gin-gonic/gin"
    )

    const (
    	healthLivePath  = "/live"
    	healthReadyPath = "/ready"
    	metricsPath     = "/metrics"
    )

    var healthMethods = []string{http.MethodGet, http.MethodHead}

    func main() {
    	r := gin.New()
    	r.Match(healthMethods, healthLivePath, live)
    	r.Match([]string{"GET", "HEAD"}, healthReadyPath, ready)
    	r.Match([]string{"GET", "HEAD"}, "/started", started)
    	r.GET(metricsPath, metrics)
    	r.GET("/version", version)
    	r.NoRoute(proxy)
    	r.Run(":8080")
    }

    func live(c *gin.Context)    { c.Status(204) }
    func ready(c *gin.Context)   { c.Status(204) }
    func started(c *gin.Context) { c.Status(204) }
    func metrics(c *gin.Context) { c.String(200, "") }
    func version(c *gin.Context) { c.String(200, "") }

    // proxy forwards every request no route answers to the service the
    // token's audience names.
    func proxy(c *gin.Context) {
    	c.Status(502)
    }

Expected 5 operations. `specarch extract go` (0.7.0) writes 2, the two whose path is a literal, and asks a must question for each of the other three: "registers a route with Match on methods healthMethods, which is not a literal list" and "registers a route whose path healthReadyPath the reader does not read as a literal", at `main.go:19`, `main.go:20` and `main.go:22`.

Cause: `stringValue` in `internal/extract/golang.go` reads a string literal, literals joined by `+` and a name a call bound, and never a package-level `const`; `register` in `internal/extract/golang_routers.go` reads `Match`'s methods only from a composite literal written in the call, never from a package-level `var` holding one. Go gives a constant one value at compile time, and a list assigned once and never changed is as plain to syntax. The fix is ADR-143's rule: a package-level string constant is the literal it names, and a package-level list is a literal list when nothing in the package assigns to it, appends to it or takes its address.

## BUG-029 — extract go: a fallback handler is dropped with no question and no line

- Reported: 2026-10-19, from reading a gateway service whose forwarding lives in gin's NoRoute handler
- Design: `docs/gateway.md`, section 3; ADR-143
- Closed: 2026-10-20, completed
- Fixed by: the commit on `main` whose message says `Fixes BUG-029`; ADR-143

The same module registers `r.NoRoute(proxy)` (`main.go:24` of the fixture listed in BUG-028), the handler every request no route matches is given to, which is where the service forwards by the token's audience. `specarch extract go` (0.7.0) writes nothing for it: no operation, no question, no line, and the registration is not counted ("routers: counted 5 registrations" for five routes and the fallback). The same holds for gin's `NoMethod`, chi's `NotFound` and `MethodNotAllowed`, gorilla/mux's `NotFoundHandler` and `MethodNotAllowedHandler` and echo's `HTTPErrorHandler`, none of which `internal/extract/golang_routers.go` names.

Expected: a must question citing the call, as ADR-075 asks for every registration the reader knows on a surface it reads and cannot write, blocking the design stage until the `gateways` section exists and `gateways` after (the validator refuses a `blocks` entry that names no section), and the call counted as a registration. A route whose path ends in a catch-all (`/*path`), which prints a line today, is the other way the same handler is written and takes the same question.

## BUG-032 — validate (Swift build): an all-digit commit past 64 bits, written without quotes, passes as a string, and no check reads the fixture that holds one

- Reported: 2026-10-20, found by an outside schema check of this repository's own specifications
- Closed: 2026-10-20, completed
- Fixed by: the commit on `main` whose message says `Fixes BUG-032`

`spec/tests/extract-go-handlers-changed/implementation/lockers.go.specarch-implementation.yaml` gave two handler records `commit: 1111111111111111111111111111111111111111` without quotes. YAML reads an all-digit plain scalar as a number, and the implementation schema asks for a string that matches `^[0-9a-f]{7,64}$`, so a strict schema check refuses the value. Neither build of `validate` reported it, for two reasons. No check reads the file: `spec.Find` stops at a folder that holds `specarch.yaml`, so `validate spec examples` never descends into `spec/tests`, and the case runs `extract go` alone, which takes `commit` as raw text. And when the Swift build is pointed at the file it accepts the value: its resolver tags the forty digits `!!int`, `scalarValue` finds they fit neither `Int64` nor `UInt64` and returns them as a string, which matches the pattern. The Go build reports it, as yaml.v3 reads an integer past `uint64` as a float, a number; the two builds printed different output for the same input. A commit of seven digits was a number in both builds already.

Expected: the two values quoted; both builds report an all-digit `commit` of any width, written without quotes, as the schema type error at its line, "this is a number, but a string is expected here"; a case shows it.

To repeat: run either build's `validate` on a copy of the file as it was. The Go build printed the type error at lines 40 and 322; `swift/.build/release/specarch validate` printed only the `implements` error of the copied file.

## BUG-040 — extract go: a registration's largest request is not read, so an upload loses its ceiling

- Reported: 2026-10-20, found by the extract leg of the dxlib round trip (item 1006)
- Closed: 2026-10-20, completed
- Fixed by: the commit on `main` whose message says `Fixes BUG-040`

dxlib's `NewEndPoint` takes the request's largest content length as its thirteenth argument, `requestMaxContentLength`, and refuses a request whose declared Content-Length is above it. `specarch extract go` read every other argument and never that one, so the vault's upload, registered with 1073741824, came out with no `limits.maxRequestBytes`, and the service regenerated from it streamed an upload of any size: the generated `UploadStream` was handed 0 for no limit. The dxlib dialect writes that limit as `x-dxlib-max-content-length`, so the round trip lost it on the way in only.

Expected: an integer literal above 0 is the operation's `limits.maxRequestBytes`; 0 is no limit and writes nothing; any other expression is a should question at `limits/maxRequestBytes`. `writeLimits` in `internal/extract/golang_transport.go` does this, and the cases extract-go-dxlib-envelope-stream and extract-go-dxlib-vault show the limit.

To repeat: run `specarch extract go` on `spec/tests/extract-go-dxlib-envelope-stream/testdata/service` as its `case.yaml` does, before the fix: `POST /attachments/upload` has no `limits`.

## BUG-034 — version: both builds name meta-model 0.2 for specifications though 0.3 is built

- Reported: 2026-10-20, found while accepting ADR-133 to ADR-139
- Closed: 2026-10-20, completed
- Fixed by: the commit on `main` whose message says `Fixes BUG-034`

`specarch version` prints `specifications: meta-model 0.2, and 0.1 until 1.0` in both builds (`runVersion` in `cmd/specarch/main.go`, the `version` case in `swift/Sources/SpecArchKit/Command.swift`), and the case `spec/tests/version-prints-versions` expects that line. The validator reads meta-model 0.3 (`schema/specarch-design-0.3.schema.json`, `specarch: "0.3"` in SpecArch's own specification and the examples), and `specarch model` says `"metaModel":"0.3"`.

Expected: the line names 0.3 as the meta-model a specification is written in, and the older versions the validator still reads, in both builds and the case.

To repeat: run `specarch version` with either build.

Fix: both builds build the two lines from the schemas they embed, the `specarch` enum of the specification schema and the `specarchImplementation` value of the implementation schema, the last version being the current one and the earlier ones read until 1.0 with a warning. The Swift copy of the schemas is checked against `schema/` by its own test, so one file sets the line for both builds, and the case `version-prints-versions` pins it for both.

## BUG-041 — docs/roadmap.md: the meta-model 0.2 entries name the 0.3 schema files

- Reported: 2026-10-20, found while bringing the plain JavaScript screens level with meta-model 0.3
- Closed: 2026-10-20, completed
- Fixed by: the commit on `main` whose message says `Fixes BUG-041`

`docs/roadmap.md` says what meta-model 0.2 was built with, and three places there name the 0.3 schema files: the 0.6.0 row of the release table and the opening line of section 5 say `schema/specarch-design-0.3.schema.json`, and the bullet on the fragment schemas in section 5's list of what 0.2 built says `schema/specarch-fragment-*-0.3.schema.json`. The move to meta-model 0.3 renamed every 0.2 schema reference to 0.3, and that also changed these, which describe what 0.6.0 carried.

Expected: the three places name the 0.2 files, `schema/specarch-design-0.2.schema.json` and `schema/specarch-fragment-*-0.2.schema.json`. The 0.3 references elsewhere in `docs/` (`conventions.md`, `from-sources.md`, the generated documents' headers) describe the schema in use today and stay 0.3.

To repeat: `grep -n '0.3.schema' docs/roadmap.md`, and read the 0.6.0 row and section 5.

## BUG-042 — docs/roadmap.md: four statements do not match the code

- Reported: 2026-10-20, found in the roadmap audit, row by row against the code
- Closed: 2026-10-20, completed
- Fixed by: the commit on `main` whose message says `Fixes BUG-042`

Four statements in `docs/roadmap.md` said less or more than the code does. Section 1 said the Swift build has the `validate` and `version` verbs, and it also has `model` (`swift/Sources/SpecArchKit/Command.swift`, ADR-138), which section 5's diagram keywords rely on. Section 1 said CI runs `specarch validate spec examples` on every push, and `.github/workflows/ci.yml` runs on a push to `main` and on a pull request. Section 2 said CI runs `--check` on every document and code target the examples use, and the library lending example's `tests` target is left out, as the CI step's own comment says, since it writes into the service's own tree the example does not hold. Section 4 named ADR-155, ADR-156 and ADR-157 without their status, though they are proposed, while every other proposed decision in the roadmap says so.

Expected: the Swift build's three verbs; push to `main` and pull requests; the `tests` target named as the one left out of the regeneration gate; "proposed" beside ADR-155 to ADR-157. Section 2 item 7 also says now what is missing for a dead-letter item: what a person does with it, since the operations guide says only that it is kept apart.

To repeat: compare `docs/roadmap.md` sections 1, 2 and 4 with `swift/Sources/SpecArchKit/Command.swift`, the `on:` block and the "Generated code and data are current" step of `.github/workflows/ci.yml`, and the `status:` of `spec/design/decisions/ADR-155.yaml` to `ADR-157.yaml`.

## BUG-036 — generate ui (plain JavaScript): the screens use the design's field names on the wire whatever wireNames says

- Reported: 2026-10-20, found while writing forms and views for the plain JavaScript screens
- Closed: 2026-10-20, completed
- Fixed by: the commit on `main` whose message says `Fixes BUG-036`

`specarch-gen-ui` never reads `wireNames`: a list reads `row.cardNumber`, a filter is sent as `cardNumber`, a form sends `{ "fullName": ... }` and a view fills `data-field="fullName"`, all by the design's names. A specification whose `wireNames` is `snake_case` serves `card_number` and `full_name`, so the screens show empty cells and send bodies the service refuses, and nothing is reported. The Next.js generator maps every name through `internal/wirename`.

Expected: every name the screens read or send is the wire name `internal/wirename` gives, or, until that is built, a specification whose `wireNames` is not the design's own names is an error at the ui target saying so.

To repeat: in a copy of `spec/tests/generate-ui/project/specarch.yaml`, add `wireNames: snake_case` to `info` and rename the field `title` to `bookTitle` (in the entity, its `required` and the page's columns), then run `specarch generate ui --unapproved`; `books-list.js` lists `"bookTitle"` where the service sends `book_title`, and nothing is reported.

Fix: `specarch-gen-ui` maps every name its pages read from a record or send through `internal/wirename`, as the other generators do: a list's columns, row key, path parameters, `with`, `when`, reason and filters by `filterable`, a view's places, parts and conditions, and a form's controls, sent fields, checks, conditions, pickers, child rows, refusals and next page's parameters, with the paginated-list idiom's names mapped too (ADR-062). A query parameter keeps its own name, and a route's parameters theirs. The case `generate-ui-wire-names` pins it.

## BUG-043 — document operations: a job tried again once reads "tried again up to 1 times"

- Reported: 2026-10-20, found while writing what a person does with an item a job keeps apart (ADR-166)
- Closed: 2026-10-21, completed
- Fixed by: the commit on `main` whose message says `Fixes BUG-043`

The jobs table of the operations guide writes a job's retries as `tried again up to N times`, from `retriesText` in `internal/generate/operations.go`, whatever N is, so `retries: { limit: 1, then: ... }` reads "tried again up to 1 times".

Expected: "tried again up to 1 time" for a limit of 1, and "times" for any other.

To repeat: give a job `retries: { limit: 1, then: discard }` and run `specarch document operations`.

Fix: `retriesText` writes "time" when the limit is 1. The case `document-writes-operations` gives its job `countStock` a limit of 1 and pins it.

## BUG-038 — model: an operation's via and a role's through are not listed as references

- Reported: 2026-10-20, found while building the identity module
- Closed: 2026-10-21, completed
- Fixed by: the commit on `main` whose message says `Fixes BUG-038`

`specarch model` lists every name a reference keyword gives as a reference to the element it names (ADR-138). Its table of reference places (`referencePlaces` in `internal/validate/model.go`, and the Swift build's) has `callers` on an operation and a command, but not `via` on an operation, which names clients too, nor a role's `through`, whose keys name permissions and whose values name clients. An editor reading the model therefore draws no link from a hop to the service its `via` names, or from a narrowed grant to its applications.

Expected: `paths/*/*/via` listed as references to clients, and `roles/*/through` keys as references to permissions and their items as references to clients, in both builds.

To repeat: run `specarch model spec/tests/validate-0-3-tokens-valid` and look at `references`: `/paths/~1invoices~1{invoiceId}/delete/via/0` (naming `middleware`) and `/roles/invoice-manager/through` (naming `invoices.delete` and `admin-web`) are absent.

Fix: the table of reference places has `paths/*/*/via` and `roles/*/through/*`, both naming clients, and a table of key places has `roles/*/through/*`, whose key names a permission; a place's keyword is its last key that is not `*`, so all three are listed with `via` or `through` as their keyword, in both builds. A client or permission of the identity module (`identity#/clients/<name>`) is a reference into another specification and is left out, as `callers` leaves it out. The case `model-lists-via-and-through` pins it.

## BUG-035 — document handover: a deployment with links and no infrastructure gets no links, ports or network diagram

- Reported: 2026-10-20, found while building the network diagram (ADR-137)
- Closed: 2026-10-21, completed
- Fixed by: the commit on `main` whose message says `Fixes BUG-035`

ADR-137 says the handover document derives its network and ports paragraphs from a deployment's `links` and `listens` whenever it has them, and prints the infrastructure sentences after them. The handover writes them only inside "What the hosts must provide", which lists only deployments with `infrastructure` (`internal/generate/handover.go`), so a deployment with links and no `infrastructure` gets neither the paragraphs nor its network diagram.

Expected: every deployment with links or listens has its links, ports and network diagram in the handover, with or without `infrastructure`.

To repeat: copy `examples/library-lending/spec`, delete the `infrastructure` block of the production deployment in `implementation/go/library-lending.go.specarch-implementation.yaml`, and run `specarch document handover --out out spec`. `out/handover.md` has no "allowed connections" sentence and no port 5671, where the example's own handover has both.

Fix: "What the hosts must provide" lists every deployment with an `infrastructure`, `links` or `listens` (`hostNeeds` in `internal/generate/handover.go`), each with its links, ports and network diagram first and its infrastructure table when it has one. A deployment the customer runs without `infrastructure` is still a `handover` error. The case `document-handover-links-without-infrastructure` pins it.

## BUG-050 — generate ui (Next.js and SwiftUI): a query parameter's filter and the paginated-list idiom's names disagree with generate openapi under wireNames

- Reported: 2026-10-21, found while fixing BUG-036 for the plain JavaScript screens
- Closed: 2026-10-21, completed
- Fixed by: the commit on `main` whose message says `Fixes BUG-050`

`internal/genscreens`, which `specarch-gen-ui-typescript` and `specarch-gen-ui-swiftui` share, maps the fields its pages read and send by `info.wireNames`, but on two names it disagrees with ADR-062 in the standard dialect. A filter by a declared query parameter is sent under the field's wire name, so a parameter `shelfName` is sent as `shelf_name`, while `generate openapi` declares `shelfName`. The paginated-list idiom's names are not mapped, so the screens ask for `pageSize` and read `totalItems`, while `generate openapi` writes `page_size` and `total_items`. The service ignores the filter and the paging, and nothing is reported.

Expected: a parameter keeps its own name and the idiom's names go on the wire by the rule, as `generate openapi` and the plain JavaScript screens have them.

To repeat: give the ui target of `spec/tests/generate-ui-wire-names` platform `web` on `nextjs-carbon` with a session, run `specarch generate ui --unapproved`, and read `app/books/page.schema.ts`: the filter's `query` is `"shelf_name"` and `wire.pageSize` is `"pageSize"`.

Fix: `ListNames` maps the idiom's names for any stack through `internal/wirename`, and keeps the dxlib dialect's names as the idiom writes them (ADR-164); a filter by a query parameter is sent under the parameter's own name. The picker's page size and the Next.js server routes' paging read the same names. The cases `generate-ui-typescript-wire-names` and `generate-ui-swiftui-wire-names` pin it.

## BUG-044 — generate sql: a date-time field with generatedDefault currentTime gets the server's own text form on MariaDB

- Reported: 2026-10-21, found while writing the MariaDB scripts (ADR-167)
- Closed: 2026-10-21, completed
- Fixed by: the commit on `main` whose message says `Fixes BUG-044`

On MariaDB the type-rendering idiom renders a date-time as `VARCHAR(35)` holding UTC text with nine fractional digits, `2006-01-02T15:04:05.000000000Z`, the one form whose byte order is time order. Its `defaults` part names `CURRENT_TIMESTAMP` as `currentTime`, so a date-time field with `generatedDefault: currentTime` becomes `VARCHAR(35) DEFAULT CURRENT_TIMESTAMP`, and MariaDB 11.4 fills it with `2026-10-20 17:30:29`: no `T`, no fraction, no `Z`, in the server's time zone. Such a value sorts apart from the values written in the idiom's form.

Expected: the default writes the idiom's form, for instance `(DATE_FORMAT(UTC_TIMESTAMP(6), '%Y-%m-%dT%H:%i:%s.%f000Z'))`, as the MariaDB scripts write the audit instant; or the idiom says why not and a reader is warned.

To repeat: on MariaDB 11.4, `CREATE TABLE dt (id INT PRIMARY KEY, at VARCHAR(35) DEFAULT CURRENT_TIMESTAMP NOT NULL); INSERT INTO dt (id) VALUES (1); SELECT * FROM dt`.

Fix: the type-rendering idiom, version 1.4.1, names `(DATE_FORMAT(UTC_TIMESTAMP(6), '%Y-%m-%dT%H:%i:%s.%f000Z'))` as `currentTime` on MariaDB, with the reason beside it, so `generate sql` writes that default and the column fills `2026-10-20T21:07:05.301287000Z`, UTC whatever the session's time zone. The case `generate-sql-generated-default-mariadb` pins it, and a test of `internal/genscripts` keeps the scripts' audit instant equal to the idiom's expression. Tried on MariaDB 11.4.4.

## BUG-051 — generate sql: a MariaDB snapshot of type-rendering 1.4.0 gets no migration to the new currentTime default

- Reported: 2026-10-21, found in the review of the BUG-044 fix
- Closed: 2026-10-21, completed
- Fixed by: the commit on `main` whose message says `Fixes BUG-051`

The BUG-044 fix moved MariaDB's `currentTime` to `(DATE_FORMAT(UTC_TIMESTAMP(6), '%Y-%m-%dT%H:%i:%s.%f000Z'))` in type-rendering 1.4.1. The snapshot of the last migration keeps a generated default by its name, and `diffField` in `internal/gensql/diff.go` compares `generatedDefault` by that name, so a project whose snapshot was written at 1.4.0 got no migration and no problem on the next `generate sql`, and the snapshot was rewritten at 1.4.1. After that one run nothing on disk showed that the column still filled `CURRENT_TIMESTAMP`.

Expected: the next expand migration sets the column's default to the new expression, or a problem at the field says it does not.

To repeat: run `generate sql` on the project of `generate-sql-generated-default-mariadb` with that case's expected `0001_expand.sql` and `snapshot.yaml` turned back to 1.4.0 (`DEFAULT CURRENT_TIMESTAMP`, `typeRendering: 1.4.0`) in its output folder: it wrote only `snapshot.yaml`.

Fix: when the snapshot is of a type-rendering version before 1.4.1 and the dialect is MariaDB, each field still holding `generatedDefault: currentTime` gets `ALTER TABLE ... ALTER COLUMN ... SET DEFAULT (DATE_FORMAT(...))` in the next expand migration, and a warning at the field, marked above the statement, says the rows written before keep the engine's own form, in the session's time zone, and are converted in a migration a person writes. The case `generate-sql-generated-default-mariadb-upgrade` pins it; ADR-167 gives the reason. Tried on MariaDB 11.4.4: a row inserted before the step kept `2026-10-20 21:19:07`, one inserted after it filled `2026-10-20T21:19:08.005756000Z`.

## BUG-046 — derive: a client named in callers by reference to the identity module gets no signature, version or deputy case

- Reported: 2026-10-21, found while building the through cases across the identity module (ADR-171)
- Closed: 2026-10-21, completed
- Fixed by: the commit on `main` whose message says `Fixes BUG-046`

`callerCases` in `internal/validate/clients.go` (and its Swift twin) resolves each client in `callers` with `d.client(name)`, which looks only at the module's own clients, so a reference such as `identity#/clients/reporting` gives nil and the client is skipped. Three cases are then never derived for it, although the identity module given in the same run holds the client: "refused with a bad signature from <client>" when its proof is `signedRequest`, the version case at the edge, and the deputy case for a service's own token, since `hasService` looks at the module's clients and at `via` references, not at `callers`.

Expected: a client named in `callers` by reference resolves through the identity module given in the same run, as the roles' `through` does since ADR-171, and gets the same cases as a client of the module's own; checked alone, the module keeps a test of such a case as waiting.

To repeat: give an estate whose core names `identity#/clients/<name>` in an operation's `callers`, the identity module's client being a service with `proof: { kind: signedRequest }`, and no `via` in the core; `specarch derive` on the estate lists no bad-signature and no service's-own-token case for that operation, while the same client declared in the core gets both.

Fix: `callerCases` resolves each entry of `callers` through the identity module given in the same run when it is a reference, `<dependency>#/clients/<name>`, so such a client gets the bad-signature case, the version case at the edge, and, when the identity module says it is a service, `hasService` holds and the deputy's cases follow. Checked alone, the module keeps a test of those cases as waiting for the identity module. Both builds; ADR-171 says it. The cases `derive-caller-estate`, `validate-caller-identity-cases` and `validate-caller-identity-case-waits` give the core a list operation whose `callers` names `identity#/clients/reporting`, a service that signs each request and has an admitted version range.

## BUG-047 — derive: the token cases put "a" before the token's name with no article check

- Reported: 2026-10-21, found while recording the derive-caller-estate golden
- Closed: 2026-10-21, completed
- Fixed by: the commit on `main` whose message says `Fixes BUG-047`

`tokenCases` in `internal/validate/tokens.go` and `Tokens.swift` build the given text as `"a " + name + " token ..."`, so a token named `access` reads "a access token minted for another service, whose aud does not name this one". The same pattern is used in the other token cases of both builds.

Expected: "an" before a name that starts with a vowel sound, in both builds, the same text from each.

To repeat: run `specarch derive` on `spec/tests/derive-caller-estate` as its `case.yaml` does. The golden `expected/core/tests/delete-invoice-refused-with-a-token-for-another-audience/test.yaml`, and the same test in `validate-caller-identity-cases` and `validate-caller-identity-case-waits`, record this until the fix; those three goldens move with it.

Fix: the token cases put `indefinite(name)` before the token's name in both builds: "an" before a vowel, "a" before a consonant and before a "u" said as "you" ("a user token"). The three goldens now say "an access token minted for another service".

## BUG-052 — approve: changes, releases and html documents are never current, so approve refuses them right after document writes them

- Reported: 2026-10-21, found while approving a specification with change records and an html site
- Closed: 2026-10-21, completed
- Fixed by: the commit on `main` whose message says `Fixes BUG-052`

`approve` in `cmd/specarch/approve.go` made each document it compares with `l.state()` alone, while `runDocument` in `cmd/specarch/document.go` also sets the records' path from the output folder, the problem marks and, for the html target, the site's parts. The changes and releases documents link the records, and the site shows the problems, diagrams and screens, so each differed from what `document` had just written, and approve refused with "differs from what the specification says now" however often `document` was run.

Expected: a document that `specarch document` has just written is current for `approve`.

To repeat: in `examples/library-lending`, run `specarch document changes spec`, then `specarch approve --by librarian -- spec`. It printed that `docs/changes.md`, `docs/releases.md` and `site/index.html` differ, and exited 1.

Fix: `loaded.documentInputs` in `cmd/specarch/document.go` gathers the implementation files and the state a document is made with, and both `document` and `approve` use it, so they cannot drift apart. The Swift build has no approve verb. The case `approve-after-document-records-and-site` approves changes, releases and html documents that `document` wrote, and the design of `approve` says each document is made as `document` makes it.

## BUG-053 — docs/roadmap.md: caller context lists BUG-046 as left to build after its fix

- Reported: 2026-10-21, found while writing the 0.7.3 row of the release table
- Closed: 2026-10-21, completed
- Fixed by: the commit on `main` whose message says `Fixes BUG-053`

Section 5's caller context entry ends "Left to build: ... and BUG-046, the cases of a client named in `callers` by reference to the identity module", but the commit that fixed BUG-046 derives those cases, and BUG-046 is in this file.

Expected: the entry says that such a client gets the same cases as a client of the module's own (ADR-171), and its "Left to build" names only the `Txn-Token` header in the swift, javascript and dart readers and `through` on a pattern row of the permission table.

To repeat: `grep -n 'BUG-046' docs/roadmap.md`.

## BUG-033 — validate: the two builds kebab-case an operationId with a run of capitals differently in the test names they suggest

- Reported: 2026-10-20, found while writing the stream fixture of item 0979
- Closed: 2026-10-21, completed
- Fixed by: the commit on `main` whose message says `Fixes BUG-033`

For an operation `uploadJSON` with no golden test, the Go build's `test_golden_missing` suggests the test name `upload-j-s-o-n-succeeds` and the Swift build's `upload-json-succeeds`; the `test_red_missing` suggestion differs the same way. The conformance suite requires both builds to print the same output, so a fixture with such a name cannot have one golden. `kebab` in `internal/validate` splits before every capital, and its Swift twin keeps a run of capitals as one word.

Expected: one rule in both builds. `docs/conventions.md` says a test is named kebab-case after its subject; the derive command's design says a run of other characters is one hyphen and says nothing of a run of capitals, so the design decides which of the two is right and the other build follows.

To repeat: validate a specification with `operationId: uploadJSON` and no tests with both builds and compare the `test_golden_missing` lines.

Fix: the Go build's `kebab` in `internal/validate/derive.go` follows the rule the Swift build already had: a capital starts a word after a small letter or a digit, and in a run of capitals only the last starts one, when a small letter follows it. `uploadJSON` now gives `upload-json-succeeds` in both builds. The design of `derive` writes the rule down. The case `validate-test-name-capital-run` holds `uploadJSON` and `listJSONUploads` with no tests.

## BUG-048 — model (Swift): a hexadecimal integer in the document is printed as text

- Reported: 2026-10-21, found while comparing the two builds' `specarch model` on every specification in `spec/tests` and `examples`
- Closed: 2026-10-21, completed
- Fixed by: the commit on `main` whose message says `Fixes BUG-048`

YAML 1.2's core schema reads `0x1` as the integer 1. The Go build's `specarch model` prints it so in `document`; the Swift build prints the string `"0x1"`. The two builds print different bytes for the same specification, which the conformance suite does not see because no model case holds a hexadecimal number.

Expected: both builds print `1`, as YAML 1.2 reads it, or the same text from each with the reason written next to the choice.

To repeat: run `specarch model spec/tests/validate-theme` with each build and compare `/tokens/color/hexInt/$value/components/0`.

Fix: the Swift build reads a plain integer as yaml.v3 reads it (`parseInteger` and `parseUnsigned` in `swift/Sources/SpecArchKit/Source.swift`): underscores dropped, an optional sign, then `0x`, `0o` or `0b` in either case or a bare leading `0` for the base, so `0x1` is 1 in both builds and `017` is 15. The integer pattern also takes the capital prefixes, as yaml.v3 does. ADR-174 says why both builds follow yaml.v3 rather than the core schema's decimal reading of `017`. The case `model-integer-forms` prints hexadecimal, octal and binary integers from both builds.

## BUG-031 — extract go: the "wrote ... questions" line leaves out the questions the handler walk asks

- Reported: 2026-10-19, found while landing the handler work (ADR-144) on the dxlib handler shapes fixture
- Closed: 2026-10-21, completed
- Fixed by: the commit on `main` whose message says `Fixes BUG-031`

`specarch extract go` says `wrote N operations on M paths, P permissions, R problems and K questions` from `write()` in `internal/extract/golang.go` before the handler walk runs, and the walk then asks its findings against each operation (`findings()` in `internal/extract/golang_findings.go`) as questions of their own, which neither that line nor the `handlers: followed ...` line counts. On `spec/tests/extract-go-dxlib-handler-shapes` the line says 49 questions and `spec/design/questions.yaml` holds 53, Q-50 to Q-53 being the findings; on `spec/tests/extract-go-handlers` it says 29 and the file holds 40.

Expected: the count printed is the number of questions written, the handler findings included, or the `handlers:` line says how many findings it asked.

To repeat: run `specarch extract go` on either fixture as its `case.yaml` does and count the `Q-` entries in `spec/design/questions.yaml`.

Fix: the `handlers:` line of `extract go` now ends its count with the questions the handler walk asked, `and asked N questions on what they do, not counted in the questions written above`, from the questions numbered while `readHandlers` in `internal/extract/golang_handlers.go` runs (its findings, faults and transaction-token questions). On `extract-go-dxlib-handler-shapes` it says 4 beside the 49, and on `extract-go-handlers` 11 beside the 29, matching the 53 and 40 in `questions.yaml`. The 18 goldens that print the line record the count; the design of `extract` says it.

## BUG-030 — derive: a question that blocks one method of a path holds up the drafts of every method of that path

- Reported: 2026-10-19, found while reviewing the handler records' golden
- Closed: 2026-10-21, completed
- Fixed by: the commit on `main` whose message says `Fixes BUG-030`

When a question's `blocks` names an operation by its pointer, `#/paths/~1parcels/post`, `specarch derive` holds up the draft tests of every operation on that path, `GET /parcels` as well as `POST /parcels`, and the "not derived" line of each names the other's questions. `Block.Element()` in `internal/spec/spec.go` keeps the first two tokens of the pointer, `/paths/~1parcels`, and `blockedBy` in `internal/validate/drafts.go` compares that element with the operation's pointer by prefix, so a block on one method matches every method of the path.

Expected: a block on `#/paths/~1parcels/post` holds up the drafts of `POST /parcels` alone; `GET /parcels` is held up only by the questions that block it, or the path or stage as a whole.

To repeat: run `specarch extract go` on `spec/tests/extract-go-handlers/testdata/service` as its `case.yaml` does. The line for `listParcels` (`GET /parcels`) names the questions on `POST /parcels`'s body widths and on `storeParcel`'s dropped result and missing transaction, and the line for `storeParcel` names `listParcels`'s. The case's golden records this until the fix.

Fix: a block holds up the subject `Block.Holds` names (`internal/spec/spec.go`, and `holds` in the Swift build's `Spec.swift`): its element, or, for a pointer into an operation, the path and its method. `blockedBy` in `internal/validate/drafts.go` and `heldUp` in `Tests.swift` compare with it, so a block on `#/paths/~1parcels/post` holds up `POST /parcels` alone, and a block on the path holds up every method. The case `validate-question-holds-one-method` shows a 0.3 specification's GET reported as missing its tests (an error) while its POST is held up (a warning), in both builds; the "not derived" lines of 20 extract and merge goldens now name only the questions on each operation, `listParcels` held up by Q-12 and Q-13 and `storeParcel` by its own six.

## BUG-045 — extract javascript: a use() on the application before a mount does not reach the mounted router's routes

- Reported: 2026-10-21, found while adding a mounted gate to extract-javascript-caller-checks
- Closed: 2026-10-21, completed
- Fixed by: the commit on `main` whose message says `Fixes BUG-045`

Express runs the middleware an earlier `app.use(mw)` adds for every request after it, the routes of a router mounted later with `app.use('/admin', admin)` included. The reader adds only the middleware of an earlier `use` on the router that registers the route (`spec/design/commands/extract.yaml`: "of an earlier `use` on the same router"), so a permission check or a gate added by `use` on the application, or on any router above the mount, is not read for the mounted router's routes.

Expected: an earlier `use` without a path, on the router a mount is made on, is part of that mount's chain, cited where it is made, for every route under the mount.

To repeat: run `specarch extract javascript` on `spec/tests/extract-javascript-caller-checks` as its `case.yaml` does. `app.use(requireAudience(...))` at `server/src/app.ts:9` guards 5 operations, and `POST /admin/invoices/{invoiceId}/restore`, whose router is mounted at line 16, is not among them. The case's golden records this until the fix.

Fix: `mountsOfRouter` in `internal/extract/javascript_routes.go` adds to each mount's chain the middleware of every earlier `use` without a path on the router the mount is made on, in its file, cited at that `use`, before the mount's own middleware; `earlierUses` is shared with the route's own earlier uses. In `extract-javascript-caller-checks`, `requireAudience` from `app.use` at `server/src/app.ts:9` now guards 6 operations, `POST /admin/invoices/{invoiceId}/restore` among them, and that operation's cite says it is gated by requireAudience and requireClient. The design of `extract` says it.

## BUG-049 — document: a problem in an implementation file is never marked at the entry a document shows

- Reported: 2026-10-21, found while fixing BUG-035
- Closed: 2026-10-21, completed
- Fixed by: the commit on `main` whose message says `Fixes BUG-049`

A document marks each problem by a Problem paragraph at the element its pointer leads to (ADR-065), but a `Mark`'s pointer leads only into the merged specification and is empty for a problem in an implementation file (`internal/generate/problems.go`). The handover, the deployment guide and the technical specification show deployments of the implementation files, yet a problem at one of them is counted in the notice under the summary and never marked at the deployment. The handover of a deployment the customer runs without `infrastructure` says of its `handover` error "None of them is in what this document covers", although the error is at the deployment its "What the hosts must provide" section describes.

Expected: a problem in an implementation file is marked at the entry of that file a document shows, and the notice counts it as concerning the document.

To repeat: run `specarch document handover --out docs project` in a copy of `spec/tests/document-handover-links-without-infrastructure`; `docs/handover.md` has the notice "None of them is in what this document covers" and no Problem paragraph at "Shop in Go: production".

Fix: a `Mark` of a problem in an implementation file keeps the file's path and the pointer into it (`Implementation`, `InFile` in `internal/generate/problems.go`, set by `loaded.marks` in `cmd/specarch/document.go`), and every document finds the elements of the implementation files it is given by that file and pointer too (`newDoc`), so the mark stands at the deepest entry of the file the document shows and the notice counts it. The handover now marks a deployment under "What the hosts must provide"; the technical specification and the deployment guide already passed their deployments to the marking and now find them. Its problem line keeps the empty pointer it printed before. The cases `document-deployment-marks-implementation-problem` and `document-techspec-marks-implementation-problem` use the project of `document-handover-links-without-infrastructure`, whose handover now has the Problem paragraph at "Shop in Go: production".

## BUG-054 — model (Swift): a float written with underscores is printed as text

- Reported: 2026-10-21, found while fixing BUG-048
- Closed: 2026-10-21, completed
- Fixed by: the commit on `main` whose message says `Fixes BUG-054`

yaml.v3 drops the underscores of a plain scalar that starts with a digit or a sign before reading it as a number, so `0_8` is 8 and `1_0.5` is 10.5 in the Go build's `specarch model`. The Swift build matched its float pattern on the text as written and printed the strings `"0_8"` and `"1_0.5"`. The two builds print different bytes for the same specification, and no model case held such a float.

Expected: both builds print 8 and 10.5, as yaml.v3 reads them.

To repeat: run `specarch model` with each build on a specification whose theme holds `$value: [0_8, 1_0.5]` and compare the two values.

Fix: the Swift build reads a plain float as yaml.v3 does (`isFloat` and the `!!float` case of `scalarValue` in `swift/Sources/SpecArchKit/Source.swift`): a scalar that starts with a digit or a sign is matched with its underscores dropped; one that starts with a dot keeps the reading of Go's ParseFloat, an underscore only between two digits, so `.5_0` is 0.5 and `._5` stays text. The value is read with the underscores dropped. ADR-175 says why both builds follow yaml.v3 rather than the core schema, which has no underscores in a float. The case `model-float-underscores` prints these forms from both builds.

## BUG-055 — model and validate (Swift): a scalar under an explicit tag, a sign after 0b or 0o, and some timestamps are read unlike yaml.v3

- Reported: 2026-10-22, found while fixing BUG-054
- Closed: 2026-10-22, completed
- Fixed by: the commit on `main` whose message says `Fixes BUG-055`

The Swift build resolved a scalar by patterns of its own, where the Go build reads it through yaml.v3, so the two printed different bytes for the same specification. A table of every scalar form yaml.v3 tells apart, run through both builds' `model`, showed 35 values and 7 problems that differ, all from that one cause:

- `!!str 1`, `!!str 0x1F`, `!!str true`, `!!str ~` and `!<tag:yaml.org,2002:str> 5` were numbers, bools and nulls; a local tag such as `!custom 1` was read by its text;
- `!!bool` took any spelling, `yes`, `on`, `tRue`, `abc`, as false or true, where yaml.v3 takes only the core schema's six and leaves the rest text;
- `!!int _1` was 1, `!!float 017` was 17 (yaml.v3: octal, 15), `!!float 0b101` and `!!float 0o17` were text (5 and 15), `!!float 0x1p3` was 8 (text), and `!!float 18446744073709551615` a number (text);
- `0b+1`, `0b-1` and `0o+7` were text, where yaml.v3 reads 1, -1 and 7;
- `2001-12-14T21:59:43` with no zone, `2001-12-14 21:59:43Z`, `2001-13-01`, `2001-02-30` and `!!str 2001-12-14` were reported as unquoted dates, where yaml.v3 reads them as text;
- a worked example of a double field read its number with `Double(text)`, so `017` was 17 and `1_000.5`, `!!float 0b101` and `.inf` were refused.

Expected: both builds read every scalar as yaml.v3 does and print the same bytes.

To repeat: run `specarch model project` with each build in `spec/tests/model-scalar-forms`, and `specarch validate .` in `spec/tests/validate-example-double-forms`, and compare.

Fix: the Swift build reads a scalar in one place, a port of yaml.v3 (`resolveScalar`, `resolveTag`, `isTimestamp` and `scalarValue` in `swift/Sources/SpecArchKit/Source.swift`): the tag as yaml.v3's parser sets it, the text resolved as its resolve does (Go's ParseInt, ParseUint and ParseFloat rules, and time.Parse of its four timestamp layouts), and a value under an explicit tag only where the text fits that tag. The Yams parser is given a resolver that leaves an untagged scalar marked untagged, so its YAML 1.1 resolver never replaces the tag written. A double example reads its number through the same port (`decodeFloat`). ADR-176 says why; the cases `model-scalar-forms` (the table) and `validate-example-double-forms` hold both builds to the same bytes in the conformance suite.

## BUG-056 — validate and model: a YAML syntax error is worded by each build's parser, and some texts are read by one build and refused by the other

- Reported: 2026-10-22, found while fixing BUG-055
- Closed: 2026-10-22, completed
- Fixed by: the commit on `main` whose message says `Fixes BUG-056`

The problem line of a yaml_syntax error passed the parser's message through, and the two builds use different parsers (yaml.v3 and libyaml through Yams), so the same broken file printed different bytes:

- `enum: [!!null]` was "the [ that starts on this line is not closed" in the Go build and "did not find expected whitespace or line break" in the Swift build, at the same line and column; `[a, !!str]` the same;
- an alias with no anchor, a control character and an unknown parser error were printed by the Swift build as Yams describes them, with the offending line and a caret inside the problem line;
- the Swift build decoded a file with bytes that are not UTF-8 with replacement characters and reported nothing, where the Go build reports yaml_syntax;
- `[!!str, a]` was read by the Go build as the value a under the tag `!!str,`, and by the Swift build as an empty value and a, which the schema refused;
- `enums: {a: b, ? c ? d}`, `[a?b]` and `[?b]` were refused or read as a mapping by the Go build and read as plain values by the Swift build;
- an unclosed `[` or `{` on line 1 of a file was reported at line 2 in both builds.

Expected: both builds print the same bytes for a broken file, the error in words the validator owns with what to do, and the parser's own message still shown.

To repeat: run `specarch validate .` with each build in `spec/tests/validate-yaml-syntax-classes`, `validate-yaml-not-utf8`, `validate-yaml-tag-in-flow` and `validate-yaml-question-in-flow`, and compare.

Fix: a table the same in both builds (`internal/source/syntax.go`, `swift/Sources/SpecArchKit/Syntax.swift`, held equal by `TestSwiftSyntaxTablesMatch`) gives every parser message a class and the class a sentence; the parser's own message is the problem's detail, printed on standard error as `<file>:<line>:<column>: yaml parser (<parser>): <message>`. Both builds read each flow collection's text for a tag that runs into `,` `[` `]` `{` or `}`, and for a `?` that is not a key's, and refuse them as yaml_syntax at the line where the collection starts, after a parse or to classify a parser's refusal. The Swift build reads a byte that is not UTF-8 as U+FFFE, which libyaml refuses, and reads a collection's flow style from the text, since Yams keeps the style of a collection's end event. A flow collection on line 1 is reported at line 1. ADR-177 says why; the four cases hold both builds to the same bytes, and `validate-yaml-syntax-entries` now says "a value is missing" where it printed the parser's "did not find expected node content".
