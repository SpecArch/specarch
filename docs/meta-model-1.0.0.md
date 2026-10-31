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
being stranded, the steps it is built in, and the owner's decisions of
2026-10-25 that settle its questions (ADR-186).

In short:

- Two numbers, kept apart. The meta-model version is the `specarch:`
  line of a specification's root file: which rules the file is written
  against. The program version is the `specarch` command's release
  (`specarch version`, the git tag): which build reads it. Meta-model
  1.0.0 ships in program release 1.0.0, and every other format SpecArch
  reads or writes goes to 1.0.0 with it (the owner's first decision).
- 1.0.0 freezes the vocabulary of the design schema as 0.3 has it, with
  two renames of a word that meant two things, a role's `rows` to
  `rowPolicies` and the handover's `source` to `sourceDelivery`, the old
  spellings deprecated and still read, and with the version line in three
  parts, after OpenAPI and AsyncAPI. Every keyword the documents leave for
  later is an addition, so none of them holds 1.0.0 back.
- Nothing breaks. Every earlier version, 0.3, 0.2 and 0.1, is read beside
  1.0.0, none dropped, so the 463 conformance cases on 0.1 and 0.2 stay
  on their versions; and the reasons the decision-tracking design planned
  as errors for a meta-model 0.4 stay warnings, so there is no 0.4.
- No specification is stranded: an earlier version is read beside 1.0.0
  with one warning that names the command that moves it, and `specarch
  upgrade` rewrites a tree line by line, with a dry run that lists every
  change and changes nothing.

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
their own: the implementation file (`specarchImplementation`), the records
(`specarchRecord`), the idioms (`specarchIdiom`), the estate
(`specarchEstate`), a vocabulary (`specarchVocabulary`) and the JSON
`specarch model` prints (`specarchModel`). Each was 0.1 and goes to 1.0.0
with the meta-model, the same in shape, so each makes the promise below
from 1.0.0 on; from then each version moves at its own pace, since each
has other readers. A file on 0.1 is checked against the `-0.1` schema of
its kind and read beside 1.0.0 with a warning, a vocabulary with none, as
it is taken whole or refused.

## What 1.0.0 promises

The versioning follows Semantic Versioning 2.0.0, read for a file format
rather than a programming interface, as OpenAPI and AsyncAPI read it:
the version names the format, and the tooling chooses the rules by the
major and minor and ignores the patch. A program that reads 1.0.0 reads
a file that says `"1.0.1"` or `"1.0.9"` as 1.0, whether or not it knew
that patch, since no patch changes what is valid or what a file means
(below). The line is still written in three parts: `"1.0"` is refused
(the version table below), as OpenAPI 3.1 has `openapi` hold the full
`major.minor.patch` and its published schema's pattern
(`^3\.1\.\d+(-.+)?$`) refuses a shorter one, and
because two spellings of one version are two ways to say one thing,
which the Low IQ Tax rules out. 0.3, 0.2 and 0.1 keep their two parts:
they are written in the files that exist, and 1.0.0 changes no file it
does not have to.

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
  a later 1.x, patch or minor. A later version may add a warning or an
  open question, since neither changes whether the file is valid; a
  warning that points out a real mistake, as `value_object_unchecked`
  does, is the way a 1.x says something new about an old file. This holds
  without exception, even where the 1.0.0 schema or validator accepts
  what `docs/conventions.md` rules out: what 1.0.0 accepts is the floor of
  what is valid for the whole 1.x, and such a bug is fixed as a warning
  (below).
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
makes no valid file invalid and changes no meaning: a description, a
typo, a message's wording, one message less where two were given for one
mistake, and a bug
where the schema or the validator refused what `docs/conventions.md`
allows (a file loses a wrong error, which is never a new one).

A rule that turns a valid file invalid is not an addition, even when it
catches a real mistake (ADR-055: the version line is the file's promise
of which rules it is written against). Such a rule waits for 2.0.0, or is
written as a warning in a 1.x. That includes the bug the other way, where
the 1.0.0 schema or validator accepts what `docs/conventions.md` rules
out: `docs/conventions.md` stays the truth of what a key means, but the
fix in a 1.x is a warning at the entry, under a rule name of its own,
saying what conventions rules out and that 2.0.0 makes it an error; the
2.0.0 plan lists every such warning and turns each into an error.

### What a 2.0.0 is

A version that does any of what the frozen list rules out: a rename, a
removal, a new required key, a rule that makes a valid 1.x file invalid
(a 1.x warning made an error among them), a changed meaning. It is
planned in its own `docs/meta-model-2.0.0.md`,
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
| 0.1 and 0.2 files "read until 1.0", with the `meta_model_version` warning | ADR-055, ADR-094, `docs/conventions.md` (Tests), `spec/design/commands/version.yaml`, the schema's description of `specarch` | Settled by the owner's second decision: 0.1, 0.2 and 0.3 are all read beside 1.0.0, each with the warning, now naming `specarch upgrade` (ADR-186). |
| `why` required on an accepted decision, `decidedBy` on every accepted decision, `retired_reason` as an error, planned as a breaking meta-model 0.4 | `docs/decision-tracking-2026-10-25.md`, 3.5 and phase 5, questions 1 and 5 | Settled by the owner's third decision: warnings, never errors in a 1.x. 1.0.0 takes the place of the planned 0.4: there is no 0.4. |
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
| the schema's other conditionals that read a key in their `if` without requiring it (`page`, `source`, `constraint` and `workflowStep` on `kind`, `parameter` on `in`, `decision` on `status`) | the history of BUG-063 (item 1231), a side finding | Fixed in the schema 1.0.0 freezes: each `if` also requires the key it reads, so an object without the key is told only that it is missing. `TestConditionalsAcceptWhatTheyDid` compares each 1.0 rule with the 0.3 one over every such object of the library lending example and SpecArch's own specification, with the key absent or given each value and each key its branches name kept or dropped, and finds no object one accepts and the other refuses. A conditional found after 1.0.0 that refuses too little is a warning in a 1.x and an error in 2.0.0. |
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
| end the reading of 0.1 and 0.2 | not taken | The owner keeps every earlier version read beside 1.0.0. |
| the reasons as errors | not taken | A missing reason stays a warning: an error would make every file without one invalid at once. |
| `rows` on a role renamed `rowPolicies` | mechanical, the old key deprecated and still read | `rows` means two things: on a role, an expression per entity that limits the records the role reaches (after PostgreSQL's row security policies); on a view property, the records of a one-to-many relation carried in the view (ADR-070). One word with two meanings is the ambiguity the Low IQ Tax rules out, and freezing it keeps it for the whole 1.x. The role's is the newer and the rarer, and PostgreSQL names it a policy. The validator's rule `rows` (`meta-model-0.3.md`, step 1) is renamed `row_policies` in the same change, for files of both versions: a rule's name is frozen from 1.0.0 (What 1.0.0 promises), and a frozen rule named for a key 1.0.0 no longer has would keep the two meanings in the problems file. The problem's pointer stays the file's own key (`/roles/member/rows/Loan` in a 0.3 file, `/roles/member/rowPolicies/Loan` in a 1.0.0 file); the four cases that expect `[rows@...]` change in the rule name only. |
| `source` on the handover renamed `sourceDelivery` | mechanical, the old key deprecated and still read | `source` is the key of a declared source on a citation and an operation's id on a page and a picker; on the handover it is a sentence on how the code reaches the customer, which is neither. |
| `via` on an operation and on a relation | kept | On a relation it is the field or join entity a relation goes through; on an operation, the services a call comes through, as HTTP's `Via` header (RFC 9110, section 7.6.3) lists the intermediaries a message came through. Both are "through what", in two objects no reader confuses; the relation's is in nearly every specification, and a rename would cost every file a change for no reader's gain. |
| `when` on an action and on a test | kept | The test's is Gherkin's given/when/then, a standard's own word in plain text; the action's is an expression. A reader of either knows which object they are in. |
| `flow` on `signIn` and on a test | kept | The sign-in's is OAuth 2.0's own name for a grant's exchange (RFC 6749, OpenID Connect Core 3.1); the test's names one of the specification's `flows`. |
| `client` under `signIn` and `clients` | kept | Both are OAuth clients (RFC 6749, section 2): the sign-in's is the client this system is at the identity provider, the others call this system. |
| the schema conditionals of the table above | none for a file | Made before 1.0.0 freezes; each proven to accept and refuse what it did, so no file's validity changes, one message where several were given. |

The names were checked by listing every key the design schema declares in
more than one definition and reading each description; the four kept and
the two renamed are every key with two meanings that list holds. `kind`,
`status`, `message`, `permission`, `format` and the rest mean the same
thing wherever they appear. Since a deprecated key is still read, a 0.3
file becomes a valid 1.0.0 file by its version line alone, and `specarch
upgrade` renames the keys besides.

## Existing specifications are not stranded

### Reading 0.3 and 1.0.0 side by side

A file declares its meta-model version on its root file's `specarch:`
line, and the validator reads it by that version:

- `"1.0.0"`, any `"1.0.<patch>"`, and every later 1.x minor the program
  knows: read as that minor, no version warning; the patch is not looked
  at, so the schema's `specarch` is a pattern per minor
  (`^1\.0\.(0|[1-9][0-9]*)$`) rather than a list of releases.
- `"0.3"`, `"0.2"` and `"0.1"`: each read under its own version's
  rules, against `specarch-design-0.3.schema.json`, with one warning at
  the line (`meta_model_version`): `the specification is written against
  meta-model 0.3, an earlier version read beside 1.0.0 with this warning;
  run specarch upgrade --dry-run on the specification to see what moving
  it to 1.0.0 changes, and specarch upgrade to move it`. A 0.1 file's
  warning names `specarch derive` first for its missing tests, and a 0.2
  file's says that 0.3 and later reserve the name `caller`. Generated
  output and the plug-in request carry the file's own version, so the
  generated headers say what the author wrote.
- A version the program does not know (`"1.2.0"` before 1.2 exists), or
  one written in fewer than three parts (`"1.0"`, `"1"`): an error at the
  line naming the versions it reads and, for a later minor, that a later
  program reads it, or, for a short one, the three-part spelling
  (`"1.0.0"`); the file is read under
  the newest version the program knows, so an unknown key it uses is the
  usual error at that key.

For the renamed keys, the program keeps the 0.3 schema beside the 1.0 one
and checks a 0.3 file against its own; the rules, the derivation and the
documents read either spelling as the 1.0.0 key, and a problem's pointer
names the key the file gives. A plug-in reads one shape only, the newest:
its request carries every renamed key under its new name, and a problem
it reports at a new key is placed at the key the file gives; the
`specarch` key of its request still says what the file declared.

### `specarch upgrade`

The verb that rewrites a specification and every file it is read from
to the newest version of each format the program reads. `upgrade`, not
`migrate`, as the owner decided: a specification already has
`migrations`, the database's, under `deployment`, and `generate sql`
writes migration files, so `specarch migrate` would read as a database
command to anyone who knows the rest.

    specarch upgrade [--dry-run] [--to <version>] <root>...

- It reads the tree as `validate` does, with the implementation files,
  the idioms and the records beside it and an estate named, and rewrites
  only what the move names: the version line of each file's format; the
  `$schema` line of every file that has one, to the 1.0 schema of its
  kind, a release URL at the program's release tag; a renamed key, at its
  line; nothing else. It edits the
  text at the line and column the parser gives, so comments, order,
  quoting and blank lines stay byte for byte as they were; it never
  re-serialises a file.
- What it cannot do mechanically it does not do, and lists: every error
  the specification would have on 1.0.0 that it does not have on its own
  version, such as a field, property or input named `caller` in a 0.2
  file (the name is the author's) or a test a 0.1 file lacks, which
  `specarch derive` writes (one verb for one job); a key given under both
  spellings; and a file the parser cannot read. Each is a problem line at
  its entry, with the command or the edit that settles it.
- `--dry-run` writes nothing and prints every change as
  `<file>:<line>:<column>: <old> -> <new>` and every step left to the
  author as a problem line, in file and line order. The exit code is the
  move's own: 0 when every change the move names is made (or, with
  `--dry-run`, can be made) and no step is left to the author, 1 when a
  step is left to the author, 2 for a usage error or a path that cannot
  be read, as `validate`'s. Problems the tree has apart from the move
  (an unknown entity, a missing test on 0.3) are neither printed nor
  counted: they are `validate`'s, and an incomplete specification, the
  usual kind, upgrades cleanly and keeps them. A file the parser cannot
  read is a step left to the author at the parser's position, since the
  move cannot be checked there. Without `--dry-run` it writes the
  changes, prints the same lines and exits the same way.
- `--to` names the version to move to, the newest by default and the
  only one this program writes, 1.0.0; another is a usage error. A tree
  already on 1.0.0 prints nothing and exits 0, so a second run changes
  nothing.
- An estate's modules are upgraded one root at a time: each module
  declares its own version, and a module on 0.3 references one on 1.0.0
  as it references one on 0.3 today.

It is designed spec first, as `spec/design/commands/upgrade.yaml` with its
design tests, in both builds, because it reads a specification as the
validator does and the two must agree on where a key is.

### Who reads the versions and follows

| Reader | What changes | Done by |
|---|---|---|
| the validator, Go and Swift | the schema chosen by the version line, the version checks made a comparison by version order (`internal/metamodel`, `MetaModel.swift`), the warning texts naming `specarch upgrade`, the embedded schemas (`schema/embed.go`, `swift/embed-schemas.sh` writing `Schemas.swift`) | step 1, and step 4 for the warnings of 0.3 and of the other formats' 0.1 |
| `specarch version` | a line per kind of file: `specifications: meta-model 1.0.0 and any later 1.0 patch, and 0.3, 0.2 and 0.1 with a warning` | steps 1 and 2 |
| `specarch extract`, `merge` and `init` | the root's version (`internal/extract/tree.go`) and the `$schema` line it writes, pinned to the release tag | step 3 |
| the fragment schemas | `tools/fragment-schemas` (`designFile`), the seven `specarch-fragment-*` files | step 1 |
| the generators and the 12 plug-ins | the `specarch` key of the request and the header of every generated file, which follows the file's own version; no plug-in tests the version, and the request carries the renamed keys under their new names | step 1 |
| the conformance cases and golden files | 732 cases before 1.0.0, counted by every line-anchored `specarch:` line in a case's files outside `expected/` (the inputs), then in `expected/` (the outputs): 448 cases had inputs on 0.1, 14 on 0.2, one on both (`merge-role-names-disagree`), 136 on 0.3; 79 `extract-*` cases had none in their inputs and a 0.3 tree in their outputs; 54 had none at all. The 463 cases on 0.1 and 0.2 stay on their versions, each now with the warning naming `specarch upgrade`, and with the warning of every implementation file, record, idiom or estate of theirs on 0.1. The cases on 0.3 are moved by `specarch upgrade`, its largest proof, apart from those kept on 0.3 on purpose: `validate-0-3-rows`, `validate-tenant-rows-overridden`, `validate-schema-conditionals-0-3`, `generate-sql-0-3-rows-mariadb` and the inputs of the `upgrade-*` cases. The expected trees of the `extract-*` and `merge-*` cases are on 1.0.0, since both write 1.0.0. Every expected output was recorded again from a run and read as a diff: it differs only in the version lines and hints, the version warnings, the headers naming the meta-model, the renamed keys' pointers and the single message of a fixed conditional | steps 1 to 4 |
| the examples in this repository | library-lending, lending-desk, estate, notice-board, reading-list, room-booking: the version lines, the `$schema` lines, the trees each `extract.sh` writes, and the generated documents and files naming the meta-model | step 4, by `specarch upgrade` and `scripts/regenerate.sh` |
| `spec/`, SpecArch's own specification, and its records | its version line and `$schema` lines; the design of `version` and the new `upgrade`; the records beside it | steps 1, 3 and 4 |
| the shipped idioms | the `specarchIdiom` line of each, and its hint | step 4 |
| the documents | `docs/conventions.md` (Tests, Meta-model versions), `README.md` with the compatibility statement, `docs/manual.md` (generated), the roadmap, the release notes, ADR-186, which settles the "until 1.0" of ADR-055 and ADR-094 | each step its own part |
| the editor's vendored schemas (`apps/myowncodeeditor/Vendor/SpecArchSchemas`) | the 1.0 schemas beside the 0.3 and 0.1 ones for the whole 1.x, `VERSION` at the program release that ships 1.0.0, and the editor choosing the schema by a file's `$schema` line or its root's `specarch:` line; the `specarchModel` it reads is 1.0.0 | an item for the editor's repository, after the release; listed for the owner, not queued |
| specifications kept outside this repository, such as the owner's private example rebuilds in the umbrella repository | nothing until their owners choose: they keep validating on their version with the warning; `specarch upgrade` moves each | an item per repository, listed for the owner, not queued |

## The version strings and files

| What | 0.3 | 1.0.0 |
|---|---|---|
| the root file's line | `specarch: "0.3"` | `specarch: "1.0.0"`, three parts, after OpenAPI's `openapi: 3.1.0` and AsyncAPI's `asyncapi: 3.0.0`; a later minor is `"1.1.0"`, and the patch never changes what a file means |
| the design schema | `schema/specarch-design-0.3.schema.json`, `$id` `https://raw.githubusercontent.com/SpecArch/specarch/main/schema/specarch-design-0.3.schema.json`, title `SpecArch specification, meta-model 0.3` | `schema/specarch-design-1.0.schema.json`, `$id` `.../schema/specarch-design-1.0.schema.json`, title `SpecArch specification, meta-model 1.0`: one file per major and minor, as OpenAPI publishes one schema per minor (`https://spec.openapis.org/oas/3.1/schema/`), since a patch changes no file's validity; a 1.1 adds `specarch-design-1.1.schema.json` |
| the fragment schemas | `specarch-fragment-<stage>-0.3.schema.json` | `specarch-fragment-<stage>-1.0.schema.json` |
| the other formats' schemas | `specarch-<kind>-0.1.schema.json` for the implementation file, the record, the idiom and the estate | `specarch-<kind>-1.0.schema.json`, the same in shape, its version line a pattern per minor |
| the `$schema` lines in a tree | `.../SpecArch/specarch/v0.7.5/schema/specarch-design-0.3.schema.json` or a relative path to `schema/` | the same form, at the release tag that ships 1.0.0 and the 1.0 file name |
| the 0.3 schema | the current schema | kept in `schema/` for the files on 0.3, 0.2 and 0.1 and the editors that load it, with its description naming the 1.0 schema and `specarch upgrade` |
| the document | `docs/meta-model-0.3.md` | this file, `docs/meta-model-1.0.0.md` |
| the release notes | `docs/releases.md` from the release records | the release of 1.0.0 lists meta-model 1.0.0 under added, and the compatibility statement below |

The compatibility statement, in the README and for the release notes:

> SpecArch specifications are written against a meta-model version, the
> `specarch:` line of the root file. From 1.0.0 the meta-model follows
> Semantic Versioning: a file valid on 1.0.0 stays valid and means the
> same on every 1.x; a 1.x only adds. A keyword is deprecated with a
> warning before a 2.0.0 removes it. Every earlier version, 0.3, 0.2 and
> 0.1, is read beside 1.0.0 with a warning; `specarch upgrade` moves a
> file on one of them. The program's own version (`specarch version`) is
> a separate number.

## The steps it is built in

Each step is built spec first, in both builds where the validator
changes, with its conformance cases, the documents regenerated and a
history entry, and passes `scripts/landing-check.sh` on its own.

1. **Accept 1.0.0 beside every earlier version.** The 1.0 design schema
   beside the 0.3 one, chosen by the version line; a version line the
   program does not read an error at the line; the two renames with the
   old keys deprecated and the rule `row_policies`; the conditionals
   fixed with their proof; the version checks made a comparison by
   version order; the 0.1 and 0.2 warnings naming `specarch upgrade`;
   ADR-186; `specarch version`'s line; the fragment schemas of both
   versions. `validate-meta-model-1-0-0` holds that a tree gives on
   `"1.0.0"` exactly the output it gives on `"0.3"`.
2. **Every other format at 1.0.0.** The `-1.0` schemas of the
   implementation file, the record, the idiom and the estate beside their
   `-0.1` ones, a vocabulary read on 0.1 and 1.0.x, `approve` writing
   records on 1.0.0, `specarchModel` 1.0.0.
3. **`specarch upgrade`.** Its design in `spec/design/commands/`, its
   design tests (a dry run listing every change, a run that writes them
   byte for byte, a second run that changes nothing, a 0.2 file with a
   field named `caller` left to the author, a 0.1 file's missing tests,
   comments and quoting kept, an estate of modules on two versions, a key
   given twice, a file the parser cannot read), both builds; `extract`
   and `merge` writing 1.0.0 trees.
4. **Move the repository.** The warning of a 0.3 specification and of the
   other formats' 0.1, each naming `specarch upgrade`; `spec/`, its
   records, the examples, the shipped idioms and every case not kept on
   its version moved by `specarch upgrade`; the documents regenerated.
   `specarch validate spec examples`, `scripts/regenerate.sh` and
   `scripts/landing-check.sh` pass in both builds, and the extract
   scripts of the examples run clean.
5. **Release.** Program 1.0.0, with the release notes and the
   compatibility statement; then the editor's item and the external
   specifications, by their owners.

The proof that old specifications still work is the conformance suite
itself: the cases kept on 0.1, 0.2 and 0.3 give their version's output,
with the version warning, and run in CI for the whole 1.x; every moved
case's output differs from before only in what the move names.

Rollback: each step is one commit and is undone by reverting it;
`specarch upgrade` never downgrades, so the way back is git's. A
released 1.0.0 still reads 0.3, 0.2 and 0.1, so a user who meets a
problem stays on their file's version and loses nothing.

## The owner's decisions

The owner answered the five questions on 2026-10-25:

1. Scope: every file kind goes to 1.0.0, not the specification's format
   alone, and the program is 1.0.0. The implementation file, the
   records, the idioms, the estate, the vocabulary and the model JSON are
   each 1.0.0, each still under a key and a version of its own.
2. Earlier versions: the program keeps reading every one of them, 0.1,
   0.2 and 0.3, beside 1.0.0; none is dropped, and the 463 cases on 0.1
   and 0.2 stay on their versions.
3. The reasons of `docs/decision-tracking-2026-10-25.md` (`why` and
   `decidedBy` on an accepted decision, `retired_reason`): a missing one
   is a warning in 1.0.0, not an error.
4. The renames: `rows` on a role to `rowPolicies` and `source` on the
   handover to `sourceDelivery`, before the freeze; the old keys are
   deprecated (`meta_model_deprecated`) and still read.
5. The verb: `specarch upgrade`, with `--dry-run`, not `migrate`.
