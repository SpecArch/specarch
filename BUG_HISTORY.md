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
