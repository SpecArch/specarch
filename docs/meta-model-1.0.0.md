# Meta-model 1.0.0

Meta-model 1.0.0 is the first stable version of the format a
specification is written in: the design schema, its fragment schemas and
the meaning the validator gives every key. It adds almost nothing. What it
brings is a promise: a file valid on 1.0.0 stays valid, and means the
same, on every 1.x, and nothing a 1.0.0 file says changes meaning before a
2.0.0. This document says what is frozen, what may still be added, what a
2.0.0 would be, how a keyword is deprecated, what 0.3 leaves open that a
stable version must settle or leave out on purpose, the few changes
between 0.3 and 1.0.0, how existing specifications reach 1.0.0 without
being stranded, and a plan in phases. It changes no code, schema or
generator; it ends with the owner's questions, and nothing below is built
until they are answered.

In short:

- Two numbers, kept apart. The meta-model version is the `specarch:`
  line of a specification's root file: which rules the file is written
  against. The program version is the `specarch` command's release
  (`specarch version`, the git tag, 0.7.x today): which build reads it.
  Meta-model 1.0.0 ships in a program release of its own, 0.8.0, and the
  program stays below 1.0 until its command line, its plug-in request and
  its other file formats are stable too (question 1).
- 1.0.0 freezes the vocabulary of the design schema as 0.3 has it, with
  two possible renames of a word that means two things (question 4),
  and with the version line in three parts, after OpenAPI and AsyncAPI.
  Every keyword the documents leave for later is an addition, so none of
  them holds 1.0.0 back.
- The breaking choices are the owner's: whether 1.0.0 ends the reading of
  0.1 and 0.2, as ADR-055 and ADR-094 promise ("read until 1.0"), which
  touches 464 of the 732 conformance cases (question 2); and whether
  the reasons the decision-tracking design plans as errors for a
  meta-model 0.4 become errors in 1.0.0, stay warnings, or wait for a
  2.0.0 (question 3).
- No specification is stranded: 0.3 is read beside 1.0.0 through every
  1.x release with one warning that names the command that moves it, and
  `specarch upgrade` (question 5) rewrites a tree line by line, with a dry
  run that lists every change and changes nothing.

## Two versions and what each means

| Number | Where it is written | What it says | Who moves it |
|---|---|---|---|
| meta-model version | the `specarch:` line of a specification's root file; the design schema's file name and `$id`; the `specarch` key of the plug-in request; the header of every generated file ("meta-model 0.3") | which rules and which meaning of every key the file is written against | a new meta-model version, by the plan in a `docs/meta-model-<version>.md` |
| program version | `specarch version`, the git tag, `spec/specarch.yaml`'s `info.version`, the release records | which build of the toolchain wrote or read a file | every release (`docs/releases.md`) |

The two move independently. Program 0.6.0 brought meta-model 0.2 and
program 0.7.2 brought 0.3; many program releases carry no meta-model
change. A program release may read several meta-model versions, and
`specarch version` names them on its second line.

The other formats in `schema/` keep versions of their own, under keys of
their own: the implementation file (`specarchImplementation: "0.1"`), the
records (0.1), the idioms (0.1), the estate (0.1), a vocabulary
(`specarchVocabulary: "0.1"`) and the JSON `specarch model` prints
(`specarchModel: "0.1"`). 1.0.0 is the specification's format only. Each
of the others reaches its own 1.0 when its own item says it is stable,
since each has other readers and changes at its own pace: the
implementation file still gains a binding with nearly every reader item.
Until then each stays a 0.x format, and a 0.x format promises nothing
across its minor versions (Semantic Versioning 2.0.0, item 4). Whether
the implementation file should go to 1.0.0 together with the
specification is question 1.

## What 1.0.0 promises

The versioning follows Semantic Versioning 2.0.0, read for a file format
rather than a programming interface, as OpenAPI and AsyncAPI read it:
the version names the format, and the tooling reads the major and minor
and ignores the patch.

### Frozen for every 1.x

- **The vocabulary**: every key the 1.0.0 design schema declares, at the
  place it declares it, with its type, its allowed values and whether it
  is required. No key is renamed, moved, removed or made required, no
  allowed value is removed, and no type is narrowed.
- **The meaning**: what each key, each value and each expression means,
  as `docs/conventions.md` and the schema's descriptions state it: every
  verb (`validate`, `derive` and the rest, read as what they say about a
  file), every keyword, every reserved word (`caller`, `public`, `nobody`,
  `people`), the expression subset with its operators, functions and
  types, the fail-closed access rule, and the derived cases a keyword
  gives.
- **What is an error**: a file with no error on 1.0.0 gets no new error on
  a later 1.x. A later minor may add a warning or an open question, since
  neither changes whether the file is valid; a warning that points out a
  real mistake, as `value_object_unchecked` does, is the way a 1.x says
  something new about an old file.
- **The extension keys**: a key starting with `x-` is allowed everywhere,
  as today, and is never given a meaning by SpecArch.
- **The problems' identity**: a rule's name (`meta_model_version`,
  `test_case_missing`) is never given a second meaning; a message's
  wording may change, since nothing should parse it.

### What a 1.x may add

A minor version (1.1.0, 1.2.0) may add a key a file is free to leave out,
a value to an enumeration whose readers handle any value, a derived case,
a warning, an expression function, a page element, a step kind of a
workflow, an interface kind. A file that says `specarch: "1.0.0"` keeps
validating under a program that reads 1.1.0; a file that uses a 1.1.0
keyword says `specarch: "1.1.0"`, so that a program that reads only 1.0
finds the line before it finds the unknown key. A patch version (1.0.1)
changes no file's validity: a description, a typo, a schema bug where the
schema refused or accepted what `docs/conventions.md` says it does not.

A rule that turns a valid file invalid is not an addition, even when it
catches a real mistake (ADR-055: the version line is the file's promise
of which rules it is written against). Such a rule waits for 2.0.0, or is
written as a warning in a 1.x.

### What a 2.0.0 is

A version that does any of what the frozen list rules out: a rename, a
removal, a new required key, a rule that makes a valid 1.x file invalid,
a changed meaning. It is planned in its own `docs/meta-model-2.0.0.md`,
built behind the same side-by-side reading as below (every 1.x read
beside 2.0.0 for the whole 2.x, with a warning and `specarch upgrade`),
and it takes every deprecated keyword out at once.

### Deprecation

A keyword that a 1.x replaces is deprecated, not removed:

- the schema marks it with JSON Schema's `deprecated: true` annotation
  (draft 2020-12, the Meta-Data vocabulary, section 9.3), which editors
  already show as struck through, and its description names the
  replacement;
- the validator gives one warning at each use, under the rule
  `meta_model_deprecated`, naming the keyword, its replacement and
  `specarch upgrade`, and the file stays valid;
- `specarch upgrade` rewrites it to the replacement when the rewrite
  is mechanical, and lists it as a step for the author when it is not;
- it is removed only in the next major version, and never in the minor
  that deprecates it.

The standard's annotation is taken rather than a keyword of SpecArch's
own because the schema is draft 2020-12 already and the editors that load
it read `deprecated` without knowing SpecArch; the annotation alone does
not make a validator warn (JSON Schema leaves that to the application),
which is why the rule exists beside it. A specification's own elements
(a field, an operation) already carry `deprecated` from OpenAPI with that
meaning; this is the same word for the meta-model's own keys.

## What 0.3 and the documents leave open

Every open decision, deferred keyword and "until ..." in
`docs/meta-model-0.2.md`, `docs/meta-model-0.3.md`, `docs/principles.md`,
`docs/roadmap.md` and the documents they lead to, and what a stable
version does with each. An addition can come in any 1.x, so only a
breaking item has to be settled before 1.0.0.

| Left open | Where | 1.0.0 |
|---|---|---|
| 0.1 and 0.2 files "read until 1.0", with the `meta_model_version` warning | ADR-055, ADR-094, `docs/conventions.md` (Tests), `spec/design/commands/version.yaml`, the schema's description of `specarch` | Settled by question 2: kept or ended. Either way 0.3 is read through 1.x. |
| `why` required on an accepted decision, `decidedBy` on every accepted decision, `retired_reason` as an error, planned as a breaking meta-model 0.4 | `docs/decision-tracking-2026-10-25.md`, 3.5 and phase 5, questions 1 and 5 | Settled by question 3: in 1.0.0, never (warnings), or 2.0.0. 1.0.0 takes the place of the planned 0.4: there is no 0.4. |
| `alternatives`, `governs`, `proposedBy`, the reasons in `diff` and `gaps`, the approval's `commit` | the same document, phases 1 to 4 | Additions: any 1.x, after that document's questions 2 to 4. |
| seed data per profile | 0.3, Left out; the roadmap | Left out; an addition when a project asks. |
| data held in a cache with an expiry, tables kept but unused | 0.2 and 0.3, Left out | Left out of the design for good: the implementation file's, `validity`, `ownedBy` or a question. |
| more than one deliverable per specification | 0.2 and 0.3, Left out | Left out: a second implementation file; a design keyword is an addition. |
| the roles or the client in an expression; sets and lists in the expression language | 0.2 and 0.3, Left out | Left out; a set type would be an addition to the expression subset. |
| rows rules on a view's or a page's own fields | 0.3, Left out | Left out for good: a rule is over the entity. |
| a route both people and clients call | 0.3, Left out | Settled: `callers` is the explicit list, `[people, <client>]` (ADR-110). |
| a front end's signature beside the person's delegated token, "the step's item settles how the two are written together" | 0.3, Machine clients | Settled by caller context: the transaction token and `via` (ADR-106 to ADR-111). |
| parallel approvals, a number of approvals out of a pool, loops in a workflow | 0.2, Workflows | Left out; a step kind is an addition. |
| interfaces beyond HTTP, messaging and the command line | 0.2, Left out; the roadmap, Not planned | Left out; an interface kind is an addition. |
| a fixed expression grammar | 0.2, Left out | Settled: the table in `docs/conventions.md` and the conformance cases are the definition. |
| sender-constrained tokens for people (`cnf`, DPoP), SAML 2.0 sign-in | `docs/caller-context.md`, left out | Left out; additions. |
| a prefix or a method list on a gateway, routing by a header, rewriting the request | `docs/gateway.md`, left out | Left out; additions. |
| the schema's other conditionals that read a key in their `if` without requiring it (`page`, `source`, `constraint` and `workflowStep` on `kind`, `parameter` on `in`, `decision` on `status`) | the history of BUG-063 (item 1231), a side finding | A patch-level fix before 1.0.0: as for the flow step, each gives a second message where one is due and accepts or refuses nothing new; each fix proves that by comparing the old and new rule over every combination, as `TestFlowStepAcceptsWhatItDid` does. |
| a part typed by another module's entity inside a value kept in columns | ADR-184, Not changed | Settled by ADR-184: imported as a schema of the entity's properties; not refused. |
| a value kept in columns whose module the run does not give | ADR-184 | Settled: the warning `value_object_unchecked`, a list of another module's schema an error from the reference alone. |
| the 0.3 document's examples written before caller context (`callers: clients`, the `actor` claim) | `docs/meta-model-0.3.md` | Not part of the format: `docs/conventions.md` and the schema are the current truth; the 0.3 plan stays as written for its period, and the 1.0.0 reference names conventions, not the plans. |

## The changes from 0.3 to 1.0.0

Only what stability needs: the version line, and what would be a mistake
to freeze. Each change is marked breaking (a valid 0.3 file is invalid
on 1.0.0, or means something else) or additive.

| Change | Kind | Why |
|---|---|---|
| `specarch: "1.0.0"` accepted, and the schema files named for 1.0 | additive | The version line itself. |
| end the reading of 0.1 and 0.2 (if question 2 says so) | breaking for those files only | The promise of ADR-055 and ADR-094. A 0.3 file is not touched. |
| the reasons as errors (if question 3 says 1.0.0) | breaking | `why` and `decidedBy` required on an accepted decision and `retired_reason` an error: a valid 0.3 file without them fails. |
| `rows` on a role renamed `rowPolicies` (question 4) | breaking, mechanical | `rows` means two things: on a role, an expression per entity that limits the records the role reaches (after PostgreSQL's row security policies); on a view property, the records of a one-to-many relation carried in the view (ADR-070). One word with two meanings is the ambiguity the Low IQ Tax rules out, and freezing it keeps it for the whole 1.x. The role's is the newer and the rarer, and PostgreSQL names it a policy. |
| `source` on the handover renamed `sourceDelivery` (question 4) | breaking, mechanical | `source` is the key of a declared source on a citation and an operation's id on a page and a picker; on the handover it is a sentence on how the code reaches the customer, which is neither. |
| `via` on an operation and on a relation | kept | On a relation it is the field or join entity a relation goes through; on an operation, the services a call comes through, as HTTP's `Via` header (RFC 9110, section 7.6.3) lists the intermediaries a message came through. Both are "through what", in two objects no reader confuses; the relation's is in nearly every specification, and a rename would cost every file a change for no reader's gain. |
| `when` on an action and on a test | kept | The test's is Gherkin's given/when/then, a standard's own word in plain text; the action's is an expression. A reader of either knows which object they are in. |
| `flow` on `signIn` and on a test | kept | The sign-in's is OAuth 2.0's own name for a grant's exchange (RFC 6749, OpenID Connect Core 3.1); the test's names one of the specification's `flows`. |
| `client` under `signIn` and `clients` | kept | Both are OAuth clients (RFC 6749, section 2): the sign-in's is the client this system is at the identity provider, the others call this system. |
| the schema conditionals of the table above | patch | No file's validity changes; one message less where two were given. |

The names were checked by listing every key the design schema declares in
more than one definition and reading each description; the four kept and
the two proposed for renaming are every key with two meanings that list
holds. `kind`, `status`, `message`, `permission`, `format` and the rest
mean the same thing wherever they appear. Whether to rename at all is
question 4; with no rename and no answer of "1.0.0" to question 3, a 0.3
file becomes a 1.0.0 file by its version line alone.

## Existing specifications are not stranded

### Reading 0.3 and 1.0.0 side by side

A file declares its meta-model version on its root file's `specarch:`
line, and the validator reads it by that version:

- `"1.0.0"` (and every later 1.x the program knows): read as 1.0.0, no
  version warning.
- `"0.3"`: read as 0.3 through every 1.x release, until 2.0.0, with one
  warning at the line (`meta_model_version`): `the specification is
  written against meta-model 0.3, which is read until 2.0.0; run specarch
  upgrade --dry-run <root> to see what moves it to 1.0.0, then specarch
  upgrade <root>`. Generated output and the plug-in request carry the
  file's own version, so the generated headers say what the author wrote.
- `"0.1"` and `"0.2"`: by question 2, either read as today with the
  warning naming `specarch upgrade`, or given an error at the line naming
  `specarch upgrade` and read under 0.3's rules, so that every other
  problem in the file is still listed (the compiler principle: a version
  no longer read is an error at its line, never a file left unread).
- A version the program does not know (`"1.2.0"` before 1.2 exists, or
  `"1.0"`): an error at the line naming the versions it reads and, for a
  later version, that a later program reads it; the file is read under
  the newest version the program knows, so an unknown key it uses is the
  usual error at that key.

When 1.0.0 renames a key (question 4), the program keeps the 0.3 schema
beside the 1.0 one and checks a 0.3 file against its own, then reads it
into the 1.0.0 shape in memory, key for key, for the rules, the
derivation, the documents and the plug-ins, keeping every position. A
plug-in therefore reads one shape only, the newest; the `specarch` key of
its request still says what the file declared. With no rename, one schema
accepts both lines.

### `specarch upgrade`

The verb that rewrites a specification to the newest meta-model the
program reads. `upgrade`, not `migrate`: a specification already has
`migrations`, the database's, under `deployment`, and `generate sql`
writes migration files, so `specarch migrate` would read as a database
command to anyone who knows the rest (question 5).

    specarch upgrade [--dry-run] [--to <version>] <root>...

- It reads the tree as `validate` does, and rewrites only what the move
  between the two versions names: the `specarch:` line; the `$schema`
  line of every file that has one, to the 1.0 schema at the program's
  release tag; a renamed key, at its line; nothing else. It edits the
  text at the line and column the parser gives, so comments, order,
  quoting and blank lines stay byte for byte as they were; it never
  re-serialises a file.
- What it cannot do mechanically it does not do, and lists: a field,
  property or input named `caller` in a 0.2 file (the name is the
  author's); the tests a 0.1 file lacks, which `specarch derive` writes
  (one verb for one job); a decision without `why` if question 3 makes it
  an error. Each is a problem line at its entry, with the command or the
  edit that settles it.
- `--dry-run` writes nothing and prints every change as
  `<file>:<line>:<column>: <old> -> <new>` and every step left to the
  author as a problem line, in file and line order; it exits 0 when the
  tree would validate on the new version after the changes, and 1 when a
  step is left to the author. Without `--dry-run` it writes the changes,
  prints the same lines and exits the same way.
- `--to` names the version to move to, the newest by default; a version
  older than the file's is a usage error. A tree already on the target
  version prints nothing and exits 0, so a second run changes nothing.
- An estate's modules are upgraded one root at a time: each module
  declares its own version, and a module on 0.3 references one on 1.0.0
  as it references one on 0.3 today.

It is designed spec first, as `spec/design/commands/upgrade.yaml` with its
design tests, in both builds, because it reads a specification as the
validator does and the two must agree on where a key is.

### Who reads the versions and follows

| Reader | What changes | Done by |
|---|---|---|
| the validator, Go and Swift | the version enum, the version checks that compare strings (`caller.go`, `tests.go`, `Caller.swift`, `Tests.swift` test `!= "0.1"` and `!= "0.2"`: they become a comparison by version order), the warning texts, which today tell a 0.1 or 0.2 file to move to `"0.3"` and say "until 1.0", the schema id in `internal/validate/schema.go`, the embedded schemas (`schema/embed.go`, `swift/embed-schemas.sh` writing `Schemas.swift`) | step 1, and step 3 for the warnings |
| `specarch version` | the second line: `specifications: meta-model 1.0.0, and 0.3 until 2.0.0, with a warning` | step 1 |
| `specarch extract` and `merge` | the root's version (`internal/extract/tree.go`, `"0.3"`) and the `$schema` line it writes, pinned to the release tag | step 2 |
| the fragment schemas | `tools/fragment-schemas` (`designFile`), the seven `specarch-fragment-*` files | step 1 |
| the generators and the 12 plug-ins | the `specarch` key of the request and the header of every generated file ("meta-model 0.3" becomes the file's own version); no plug-in tests the version, and with the in-memory reading they read one shape | step 1 (headers follow the file), step 4 for a rename |
| the conformance cases and golden files | 732 cases: 449 on 0.1, 15 on 0.2, the rest on 0.3. 245 expect a `meta_model_version` warning; 161 of the 0.1 cases expect a missing-test warning that becomes an error on 0.2 and after; 49 have expected output naming meta-model 0.3. Cases that test a version on purpose (`validate-meta-model-0-2-missing-test`, `validate-0-3-meta-model-0-2`, `derive-meta-model-0-2` and its sibling) stay on their version; every other case moves to 1.0.0 by `specarch upgrade` itself, which is its largest proof | step 3 |
| the examples in this repository | library-lending, lending-desk, estate, notice-board, reading-list, room-booking: the version lines, the `$schema` lines, the trees each `extract.sh` writes, and 274 generated documents and files naming meta-model 0.3 | step 3, by `specarch upgrade` and `scripts/regenerate.sh` |
| `spec/`, SpecArch's own specification | its version line and `$schema` lines; the design of `version` and the new `upgrade` | steps 1 and 2 |
| the documents | `docs/conventions.md` (Tests, the version paragraph), `README.md`, `docs/manual.md` (generated), the roadmap, the release notes, a decision record that supersedes the "until 1.0" of ADR-055 and ADR-094 | each step its own part; the decision record with step 1 |
| the editor's vendored schemas (`apps/myowncodeeditor/Vendor/SpecArchSchemas`) | the 1.0 schemas beside the 0.3 ones for the whole 1.x, `VERSION` at the program release that ships 1.0.0, and the editor choosing the schema by a file's `$schema` line or its root's `specarch:` line | an item for the editor's repository, after step 1 is released; listed for the owner, not queued |
| specifications kept outside this repository, such as the owner's private example rebuilds in the umbrella repository | nothing until their owners choose: they keep validating on 0.3 with the warning; `specarch upgrade` moves each | an item per repository, listed for the owner, not queued |

## The version strings and files

| What | 0.3 | 1.0.0 |
|---|---|---|
| the root file's line | `specarch: "0.3"` | `specarch: "1.0.0"`, three parts, after OpenAPI's `openapi: 3.1.0` and AsyncAPI's `asyncapi: 3.0.0`; a later minor is `"1.1.0"`, and the patch never changes what a file means |
| the design schema | `schema/specarch-design-0.3.schema.json`, `$id` `https://raw.githubusercontent.com/SpecArch/specarch/main/schema/specarch-design-0.3.schema.json`, title `SpecArch specification, meta-model 0.3` | `schema/specarch-design-1.0.schema.json`, `$id` `.../schema/specarch-design-1.0.schema.json`, title `SpecArch specification, meta-model 1.0`: one file per major and minor, as OpenAPI publishes one schema per minor (`https://spec.openapis.org/oas/3.1/schema/`), since a patch changes no file's validity; a 1.1 adds `specarch-design-1.1.schema.json` |
| the fragment schemas | `specarch-fragment-<stage>-0.3.schema.json` | `specarch-fragment-<stage>-1.0.schema.json` |
| the `$schema` lines in a tree | `.../SpecArch/specarch/v0.7.5/schema/specarch-design-0.3.schema.json` or a relative path to `schema/` | the same form, at the release tag that ships 1.0.0 and the 1.0 file name |
| the 0.3 schema | the current schema | kept in `schema/` through the whole 1.x, for the 0.3 files and the editors that load it, with its description naming 2.0.0 as the end of its reading |
| the document | `docs/meta-model-0.3.md` | this file, `docs/meta-model-1.0.0.md` |
| the release notes | `docs/releases.md` from the release records | the release of 0.8.0 lists meta-model 1.0.0 under added, the end of 0.1 and 0.2 (if question 2 says so) under removed, and the compatibility statement below |

The compatibility statement, for the README and the release notes once
1.0.0 is built:

> SpecArch specifications are written against a meta-model version, the
> `specarch:` line of the root file. From 1.0.0 the meta-model follows
> Semantic Versioning: a file valid on 1.0.0 stays valid and means the
> same on every 1.x; a 1.x only adds. A keyword is deprecated with a
> warning before a 2.0.0 removes it, and every 1.x file is read beside
> 2.0.0 for the whole 2.x. A 0.3 file is read beside 1.0.0 until 2.0.0,
> with a warning; `specarch upgrade` moves it. The program's own version
> (`specarch version`) is a separate number.

## The plan

Each step is one item, built spec first in both builds where the
validator changes, with its conformance cases, the documents regenerated
and a history entry. Steps 1 to 3 hold whatever the questions answer;
step 4 exists only if question 3 or 4 asks for a breaking change.

1. **Accept 1.0.0, change nothing else.** `"1.0.0"` in the version enum,
   the 1.0 schema files beside the 0.3 ones (identical apart from the
   version enum, the title and the descriptions of `specarch`), the
   version checks made a comparison by version order in both builds, the
   decision record that supersedes "until 1.0", `specarch version`'s
   line, the fragment schemas. Done when a new case
   `validate-meta-model-1-0-0` holds that a tree validates on `"1.0.0"`
   with exactly the output it gives on `"0.3"` less the version warning,
   in both builds; a case holds an unknown version (`"1.0"`, `"1.2.0"`)
   as an error at the line with every other problem still listed; and
   every existing case passes unchanged. The smallest safe step: no file
   in the repository changes its version, so nothing a user has changes
   meaning.
2. **`specarch upgrade`.** Its design in `spec/design/commands/`, its
   design tests (a dry run listing every change, a run that writes them
   byte for byte, a second run that changes nothing, a 0.2 file with a
   field named `caller` left to the author, comments and quoting kept,
   an estate of two modules on two versions), both builds; `extract`
   and `merge` writing 1.0.0 trees. Done when the cases pass in both
   builds with the same output.
3. **Move the repository.** The 0.3 warning naming `specarch upgrade`;
   `spec/`, the examples and every case not on a version on purpose
   moved by `specarch upgrade` (and `specarch derive` for the 0.1 cases
   whose missing tests become errors, if question 2 ends 0.1), the
   documents regenerated. Done when `specarch validate spec examples`,
   `scripts/regenerate.sh` and `scripts/landing-check.sh` pass in both
   builds, the extract scripts are byte-identical on two runs, and the
   cases kept on 0.1, 0.2 and 0.3 still give their version's output.
4. **The breaking parts, if any** (questions 3 and 4): the renames with
   the 0.3 schema checked and the in-memory reading, `upgrade` rewriting
   them; the reasons as errors. Done when a 0.3 file using `rows` on a
   role validates with the same rules and output on 0.3 as before, and
   its upgraded copy validates on 1.0.0 with the same problems at the
   same entries.
5. **Release.** Program 0.8.0, with the release notes and the
   compatibility statement; then the editor's item and the external
   specifications, by their owners.

The proof that old specifications still work is the conformance suite
itself: every case keeps its expected output through steps 1 and 2; in
step 3 every moved case's output differs from before only in the
version warning and the generated headers, which the step's review reads
as a diff of the expected files; the cases kept on purpose on 0.1, 0.2 and
0.3 run in CI for the whole 1.x.

Rollback: step 1 is undone by removing `"1.0.0"` from the enum and the
1.0 files; no specification declares it yet. Step 2 adds a verb and
removes nothing. Step 3 is one commit per kind of tree (cases, examples,
`spec/`) and is undone by reverting that commit; `specarch upgrade`
never downgrades, so the way back is git's. Nothing is released before step 3
passes, and a released 0.8.0 still reads 0.3, so a user who meets a
problem stays on their 0.3 file and loses nothing.

## For the owner to decide

1. Is meta-model 1.0.0 the specification's format alone, with the
   program at 0.8.0 and the implementation file, records, idioms, estate,
   vocabulary and model JSON on their own 0.x versions? Recommended: yes;
   the program reaches 1.0 when its command line and plug-in request are
   stable, and the implementation file its own 1.0 in an item of its own.
   Or did you mean the program, `specarch` itself, to be 1.0.0 as well?
2. 0.1 and 0.2 at 1.0.0: end their reading as ADR-055 and ADR-094
   promise (an error at the line naming `specarch upgrade`, the file
   still read under 0.3's rules), or keep reading them with the warning
   through 1.x? Recommended: end it, as promised; it moves 464 cases, and
   161 of the 0.1 cases need `specarch derive` for tests that become
   required.
3. The reasons of `docs/decision-tracking-2026-10-25.md` (`why` and
   `decidedBy` on an accepted decision, `retired_reason`): errors in
   1.0.0, warnings for good, or errors only in a 2.0.0? Recommended:
   warnings for good; a missing reason is an incomplete specification,
   which the problems file shows and which is read like any other, and an
   error would make every 1.x file without one invalid at once.
4. The renames: `rows` on a role to `rowPolicies`, `source` on the
   handover to `sourceDelivery`, or neither. Recommended: both, now,
   since after 1.0.0 each would wait for 2.0.0; both are mechanical and
   `specarch upgrade` does them.
5. The verb: `specarch upgrade`, or `specarch migrate` as asked?
   Recommended: `upgrade`, since `migrate` reads as the database's
   migrations.
