# specarch generate

```
specarch generate <target> [--out <out>] [--check] [--estate <estate>]... [--environment <environment>] [--unapproved] [--dry-run] [--as-is] [--package <package>] <paths>...
```

Write code or data from a specification.

Validates its input first and writes nothing from an invalid
specification. Without `--out`, passes over a specification whose
implementation files name no output folder for the target, with a
warning of rule output_folder at their targets, as document does
(ADR-073), and exits 2 when every specification given is passed
over. Then refuses, with status 1, while a must or should question
blocks a section the target reads: the sections the implementation
file names under `targets.<target>.reads`, or every section when it
names none. Then refuses, with status 1, when
`records/approvals/<version>.yaml` is missing or its digest is not the
digest of the specification's files now. `--unapproved` lets the run
through both refusals and makes the output a draft: every file says
it is one, naming the questions that block what the target reads and
the approval's state, and a draft exits 0 (ADR-066, ADR-086). Then
writes the target into the folder it owns, and nothing outside it.
With `--check` nothing is written: the output is made in memory and
compared with what is on disk. A generated file, one whose text has
the line that starts the generated header, is compared byte for
byte. A file with no such line was built by hand: it matches when it
equals what the target writes without the header, the draft notice
and the problem marks, and a match is named in a line of its own and
not counted as differing. A file the plug-in marks as an object
literal is, when it differs, read on both sides in the subset of
TypeScript that is also JSON5 (ADR-093) and compared by value: the
imports, the exported constant's name, the type it satisfies and the
literal. A hand-built one with equal values matches; for any other,
each differing value is printed below the file's line, by its path
in the literal, with the value on disk and the one generated; any
other file built by hand that differs names its first differing
line, both ways. A file
on disk outside the subset is not comparable, named with its line
and what was found there, and counts as differing.

A target this program does not build in is produced by a plug-in found
on PATH. For each implementation file that names the target under
`targets` (every implementation file when none does), the plug-in is
`specarch-gen-<target>-<stack>` when there is one, where the stack is
first the target's `framework` in lower case, given under
`targets.<target>.framework` or the platform's default when the
target names a platform and no framework (plain-javascript on web,
swiftui on iphone), then the file's `stack.language.name` in lower
case with a dash for a space, and `specarch-gen-<target>` otherwise;
none found is status 2. A framework is a stack of the file, as a
dialect is, so one language may have a plug-in per framework. Files
that find the same plug-in and name the same output folder run it
together, a file that names none joining the others of its plug-in,
and each run writes into the folder its files name, so `--out` is
refused when the files reach more than one plug-in or folder. The program runs each plug-in once per
specification with a request on its standard input, as JSON:
`specarch` (the meta-model version), `digest` (the specification's
digest, as the approval record keeps it, which every generated file
names in its header beside the version, so a remark from acceptance
can name the build; ADR-083), `environment` and `trace` (the
environment given with `--environment`, and whether its entry says
the build shows the trace; ADR-222), `target`, `root` (the root
file's path), `specification` (the merged, validated specification as plain
values, each workflow its to-be with no asIs, or its as-is with
`--as-is`), `implementations` (each implementation file as `file`,
`content`, the target's `settings` from it, and `idioms`, the idioms
it uses, each with its `name`, `version`, how it applies (`as`:
shipped, overridden, project or excluded), the `content` of the idiom
that applies and the project's `override`; the ui target's request
carries the paginated-list idiom whether or not the design has a
`listOf`, since its server routes page a list answered whole in the
idiom's envelope and its screens page another module's list by the
idiom's names, so no name is left empty for a design that lists
nothing itself, and the file's exclusion or override of the idiom
applies to it as to any other) and `output` (the folder
the files are for), and `existing`, the text files already in that
folder by their path in it, so a plug-in that adds files, such as a
migration beside a snapshot, knows what is there without reading the
disk; `draft`, the notice every file of a draft carries, from
"Draft:" to "this file is not the approved output", or empty for the
approved output; and `problems`, the specification's warnings and
open questions in the order of its problems file, each with its
`id`, `severity`, `rule`, `message`, the `pointer` into the merged
specification it is marked at (empty for a problem outside it, such
as in an implementation file) and, for a question, what it
`blocks`, so the output marks each one at the entry it shows for it
(docs/diagnostics.md); and `check`, true when the run only compares,
so a plug-in that refuses to write over a file built by hand answers
its files for the comparison; and `dependencies`, for the targets
that read another module (openapi, sql, seed, go-dxlib and ui), the
specifications of the run that the specification's dependencies
name, keyed by the dependency, each with the module's
`specification` and `version` (its info.name and info.version), its
`content` as plain values, the `apiDialect` it is served in and the
`idioms` of the implementation file that serves its API (the shipped
ones when it has none), so a list it answers is paged by its own
names, and its own `dependencies` that name a specification, by
their key there, each with its `specification` and `version`; and
`modules`, for the same targets, the specifications of the run that
a dependency reaches through its own dependencies, at any depth, and
this specification's dependencies do not name, by their info.name,
each with its `specification`, `version`, `content` and
`dependencies` (ADR-185). The plug-in answers on its standard output,
as JSON, with `files` (each a `path` relative to the output folder,
its `content`, and `comparison: object-literal` for a file that is
one object literal of data, which `--check` compares by value) and `diagnostics` (each with the validator's fields:
file, line, severity, path, rule, message, and a column when it
knows one), and exits 0. The program places each diagnostic at its
entry: one at the root file, whose path leads to an entry of the
merged specification, is put at the fragment, line and column that
entry was read from; a column not given is derived as validate
derives it; and the id is made as validate makes it. It prints the
diagnostics in validate's line, refuses a path outside the output folder,
and writes or checks the files itself. When the plug-in answers
with diagnostics, the program runs it once more with them added to
`problems`, each with its id and `target: true`, so the files mark
the target's own problems as well; it writes what that run answers.
A plug-in that cannot express an entry answers the rest of its
output with that entry marked, and an error makes the run exit 1
after the files are written or checked; a plug-in that can write
nothing answers no file (ADR-086). A plug-in that exits with
another status, or answers with something else, fails the run with
status 2; its standard error is passed through.

A reference into another specification through a dependency,
`<dependency>#/<pointer>`, is read where the run gives that
specification: in the same folders, or through an estate given as a
folder or a file, or with `--estate`, whose modules the run reads and
checks with the estate and does not generate, the folders naming
those it does (ADR-184). Before the plug-in runs, a reference the run
does not give is refused as reference at its entry, with status 1
and nothing written: a value typed by another specification's element
(for sql and seed, a field of an entity or a view), and for ui a list page or
a picker reading another specification's list; every other target
refuses those whatever the run gives; tests reads the specifications given
for the integration tests below. A relation to another
specification's entity is not refused: its via field is this
specification's. Given the specification, openapi writes the
element a field is typed by inline where the field is, with every
element it names there, each marked `x-specarch-ref` with the
module's name and the pointer, and no component for it (the dxlib
dialect, whose reader takes a $ref into the components only, writes
it as a component, marked); sql takes the columns of a field typed
by another module's schema or entity, and the check of one typed by
its enum, from that element, as from its own, and seed draws the
rows of such a column as for one of its own; go-dxlib writes a
field typed by another module's schema or entity as a value object
of it, commented with the module and the pointer; ui draws such a
field as its own, and writes a list page or a picker that reads
another module's list with that module's entity, wire names and
paging names, called at the address the target's settings give the
dependency: on Next.js, under the server routes after the module's
name, whose route forwards it with the session's token to the address
the environment variable `settings.server.services.<dependency>`
names; in plain JavaScript, at the URL or the path of the screens'
own origin `settings.services.<dependency>` gives. A value typed by
another module's view, an element whose name the specification
already gives in a section the generators write under one name, a
schema that holds itself through the other module, a call to another
module in the dxlib dialect, a call with no address in the settings,
and on SwiftUI, which calls one service, any call to another module,
are errors at the entry naming the module and the pointer; the rest
is written with them marked. A reference inside an element imported
from another module resolves through that module's own dependencies,
so a specification declares a module only where it names the module
itself; a module there that the run does not give is an error at the
entry that led to it, naming the module chain and the dependency
that names the module, with its version (ADR-185).

Given the specifications of the modules its dependencies name,
tests writes, beside the design tests, one integration test per
operation of another module the specification calls, through a
calls entry of an operation, a job or a gateway, or the source of a
list page or a picker, naming every place that calls it, the
calling and the called module with their versions, the dependency,
the method and path and the caller's time limit. Each test runs the
golden path, with the operation's permission and the input of the
called module's golden design test of the operation, the harness
filling every required value it does not give, checking the
success status and every field the answer requires, and the red
paths the operation implies: denied without its permission, each
required query, header or body field left out, each other answer
it declares, the module failing, and the module not answering
within the time limit; each checks the status the operation
declares for it (403 denied, 400 or 422 for a missing field), or a
refusal when it declares none. The tests of the calls into one
module go into one file named after the module, and the harness
they share into the file of the design tests, so a change in one
module's contract changes that module's file only. A call into a
module the run does not give is named, with its place and the
module, in a comment of the design tests' file (ADR-226).

An element whose mapping in the implementation file names a
stakeholder under `ownedBy` is owned by that stakeholder: a plug-in
that builds code or data writes nothing for it or for anything under
its pointer, and an element that refers to it still refers to it; the
tests target, which checks rather than builds, keeps it (ADR-046). The gate does not look
at the mark, so a question that blocks an owned element still holds
the target up.

`generate tests --package <environment>` writes a test package in
place of a stack's tests, built in and through no plug-in, since it
belongs to no stack (ADR-233). It takes one specification, `--out`
and the key of one of its environments, and refuses with status 2
another target, no `--out`, an environment the specification does
not have (naming the ones it has), several specifications, and
`--estate` or `--environment`. It writes one file,
`specarch-test-package.yaml`, with the generated header and, for a
draft, the draft notice as comment lines, then: the format version;
the specification's title, version, meta-model, digest and draft
notice; the environment; the servers that answer requests in it,
each server of a deployment of an implementation file whose
`environment` is that one, of kind service or of no kind, no
dependency, with an http or https address, by deployment, name and
address; and the cases. Each test of the specification is a case,
named after it, with its element `#/tests/<name>`; each chosen case
no test covers is a case too, drafted as `specarch derive` drafts it,
named as derive names it, marked derived, with the element of its
subject. A case carries its subject, scenario, covers, given, when
and then, and either the reason it cannot run from outside, or its
request: the method, the path with the path parameters of its input
written in, the query parameters as the query, the header and cookie
parameters as headers, the other fields as the JSON body when the
operation takes one, and named as not sent when it takes none, each property at every depth named as
`info.wireNames` puts it on the wire while a parameter keeps its
name (ADR-062); the caller's role; the fixture's records as
JSON; the status and the body fields expected, with the fields to
compare as numbers; and the state and messages expected, named as
not checked. A case cannot run from outside when its test does not
apply, gives no request (a derived case whose call derive does not
draft says so), leaves a path parameter of its path without a value
(its answer would be the server's for a path it does not serve), is about a command or a page, travels in a
handshake, streams a file, gives the caller's properties, expects
only what is left behind, covers a denied case with no caller (a
request with none is refused as not signed in), or calls an API
served in the dxlib dialect, whose envelope a package does not
write. No credential and no address
other than the deployment's goes into the package.

An open must or should question that blocks nothing but tests holds
back those tests alone: each is left out of the cases and listed
under `held` with the questions, and on standard error, and joins
the package once they are answered. The derived cases of rank
other no test covers, which the test plan lists as left out, are
listed under `leftOut` with their subject, case and reason, and not
run. Every other open question, and
the approval, gate the package as they gate every target, and
`--unapproved` makes it a draft. Prints one line per test held back
and the counts: the cases, how many are derived, how many run, how
many are skipped with a reason, how many tests are held back and how
many derived cases are left out.

## Arguments

- `<target>` (required): A code or data target, built in or offered by a plug-in; kebab-case.
- `<paths>` (required; one or more): Folders holding specifications, searched as validate searches them.

## Options

- `--out <out>`: The folder the target owns. Without it, the folder the implementation files' `targets` name for the target, relative to each implementation file; they must agree.
- `--check`: Make the output in memory and fail when the files on disk differ. Writes nothing.
- `--estate <estate>` (given once per value): An estate file whose modules the run reads beside the folders given, so a reference into one of them is written rather than refused; the modules are checked with the estate and none is generated unless a folder names it; repeated, one per estate.
- `--environment <environment>`: The key of the environment the build is for. When that environment's entry says `trace: true`, the output shows the trace to its testers (ADR-222): the web UI carries each page's, section's, field's, column's and action's pointer into the specification in `data-specarch-element` attributes and loads an overlay showing the pointer under the focus, the build's version and digest, and a remark to copy as the start of a change request; the Go server sends `Specarch-Element` on every answer. A key that is not an environment of a specification given is refused with status 2. Without it nothing shows the trace, and that untraced output is what the implementation file's output folder holds: a build for an environment that shows the trace is refused with status 2 unless `--out` names its folder, since the two cannot share one folder under `--check`; the refusal names the environment and the folder it would have written into.
- `--unapproved`: Write a draft from a specification that is not approved yet, because no approval record covers its files as they are or an open must or should question blocks what the target reads. The refusal is the default; with this, every file says it is a draft and why.
- `--dry-run`: Run the plug-in and write nothing; print each file it would write, as would write <path>. Not with --check.
- `--as-is`: Give the plug-in each workflow's as-is in the workflow's place, for a build of the process the business runs today while it moves to the to-be. Without it every plug-in gets each workflow's to-be, with no asIs; a workflow that keeps no as-is is the same either way (ADR-236).
- `--package <package>`: With the tests target and --out: write a test package for this environment of the specification, every test and every derived case no test covers as a request to its servers, in place of a stack's tests (ADR-233). Not with --estate, --environment or --as-is.

## Reads

- `{paths}`: The specifications and their implementation files.
- `records/approvals/<version>.yaml`: The approval of the specification's version, beside its folder.
- `{estate}`: The estate whose modules the run reads, with `--estate`, and each module's specification and implementation files.
- `{out}`: The current output, with `--check`.
- `specarch-gen-{target} on PATH`: The plug-in for a target that is not built in.

## Writes

- `{out}/`: The files the target produces, as the plug-in answers them. Nothing is written with `--check`.

## Standard output

The diagnostics of an invalid specification, the output_folder
warning of each specification passed over, and the diagnostics the
plug-in answers with, one line each. With `--check`, one line per
file that differs, is missing or is not comparable, each differing
value of an object literal on an indented line below it, and for
any other file built by hand its first differing line, and one line
per file built by hand that matches.

## Standard error

A usage message on a usage error, the reason a plug-in was not found or failed (its path and exit status, or the start of an answer that is not JSON), each file the plug-in is not given because it cannot be read and why, one line per step when a run takes more than one plug-in or output folder (N of M), the plug-in's own standard error, and a one-line count at the end.

## Exit status

- 0: written, a draft included, or with `--check` the output is current.
- 1: a specification has an error, an open question blocks what the target reads or the specification is not approved and `--unapproved` was not given, the plug-in reported an error (what it answered is written or checked), or with `--check` the output differs. Then: Settle the errors, answer the questions that block the target, or approve the specification with specarch approve; with --check, run it once without --check to write the current output.
- 2: usage error, no generator for the target (not built in and no plug-in on PATH), the plug-in failed or answered badly, no output folder in any specification given, a build for an environment that shows the trace without `--out`, or a file that could not be read or written. Then: Put the plug-in specarch-gen-<target> on PATH, name an output folder for the target in an implementation file, give --out for a build that shows the trace, or correct the command line, and run it again.
