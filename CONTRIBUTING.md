# Contributing

SpecArch is developed and owned by one person. Anyone who wants to
contribute writes to the owner first and waits for an answer before writing
the change. The owner decides each contribution on its own, yes or no, and
may ask for a signed agreement before accepting one. A pull request or patch
that arrives without that first contact is closed, whatever its quality, and
so is one whose only grounds are that it fixes a listed bug: the owner fixes
listed bugs.

Agents working in this repository never merge or accept an outside pull
request or patch themselves. They report it to the owner, who decides.

## Bugs

The repository has no GitHub issues: an issue tracker open to anyone is a
way in for a pull request that claims to fix what an issue lists. Bugs are
kept in the repository instead, one entry per bug, numbered BUG-NNN:

- `BUG_OUTSTANDING.md` holds the open bugs: what was seen, what was
  expected and how to repeat it.
- The commit that fixes one names it with `Fixes BUG-NNN` in its message,
  and the entry moves to `BUG_HISTORY.md` with that commit.

BUG-001 to BUG-021 were GitHub issues #1 to #21 and keep the same number,
so an older commit's `Fixes #N` names BUG-N. Anyone outside who finds a bug
writes to the owner; a security weakness goes the way `SECURITY.md` says.

## What belongs here

The public repository holds generic material only: the meta-model, the
validator, the generators, the documentation and example projects. An example
must be free of any employer's or client's names, data, code or internal
detail. Any specification written for a particular organisation stays in that
organisation's own repository, where it points at this one as a dependency.

## Checks before a change is proposed

One script, run from the repository root, holds the checks a change must
pass; CI runs it too, so the two cannot differ:

    sh scripts/landing-check.sh

It builds, vets and tests the Go build, checks the fragment schemas,
validates `spec/` and `examples/`, checks that every generated document and
the library lending example's generated code and data are current, builds
and tests the Swift build, and compares the two builds' validate and model
output. It stops at the first failure and names the step. `sh
scripts/landing-check.sh go` runs the Go part alone and `swift` the Swift
part. CI also runs `govulncheck ./...`, the extraction examples and the
web screens' build.

    sh scripts/regenerate.sh

writes every file the script checks as generated. To settle a conflict in
one, take either side, run it on the merged tree, then run the checks.

While a change is in progress, `scripts/test-map.json` names the narrowest
check that covers each path: a list of rules, each a set of globs and the
command for them, the first matching rule taking a path. In a command,
`{paths}` is the changed paths the rule took, `{dirs}` their folders as
`./<folder>`. A Go file runs its own package and the `cmd/specarch` tests,
since the conformance suite there is what exercises most packages; a
conformance case runs only that case, in both builds; a schema or idiom
runs both parts of the landing check, since both builds embed them. A path
no rule takes (prose, history, CI, SBOM records) runs nothing. The landing
check stays the test before a change lands.

`go test` runs the conformance suite, which is the tests stage of SpecArch's
own specification, `spec/tests/`: every test folder holds `test.yaml` (the
design test), `case.yaml` with the arguments, the exit status and the exact
output expected, the input files of the case, and, when the case writes
files, an `expected/` folder with each file as it must be afterwards. A
case that needs a generator plug-in carries it as an executable under
`plugins/`, which the harness puts on PATH for that run; with no such
folder, PATH holds nothing. The Go and Swift builds of `specarch` both pass
the validate and version cases. A change to the validator is made in both
builds, and their output stays identical.

A new case is added together with its design test. Put the input files in a
new test folder with its `test.yaml`, run the program in a copy of it, and
record the arguments, the exit status, the standard output and every file it
wrote; read each line before it becomes the expectation.

A change to the program changes its specification first, in `spec/`, and the
validator must pass on `spec/` and `examples/` with no error. A change to a
specification is followed by `sh scripts/regenerate.sh`, and the regenerated
files are committed with it.

Dependencies are added only when their licence is OSI-approved and their SBOM
scan (syft, then grype and osv-scanner; govulncheck for Go) is clean, or when
the only finding is proven unreachable by govulncheck and the owner has
accepted it. The result goes in the commit message, and each library is
listed with its version and licence under `libraries` in
`spec/implementation/go/specarch.go.specarch-implementation.yaml`.

## Releases

A release is one commit on main that sets the version everywhere the
program and the specification name it, followed by the tag. The commit sets
the planned release record to released and names the change records it
includes, sets `info.version`, both implementation files'
`implements.version`, `Version` in `embed.go` and `programVersion` in the
Swift build, moves the editor hints whose schema changed since the last
tag and the install lines to the new tag, and passes
`sh scripts/landing-check.sh`. Once it is on origin's main, the tag
`vX.Y.Z` is made on it, annotated, and pushed. The GitHub release is
created from it as a draft (`gh release create vX.Y.Z --verify-tag
--draft`), never published at once: the repository's releases are
immutable, and GitHub refuses any asset on a published one. The tag's
tree is then scanned: syft writes the CycloneDX and the SPDX SBOM
(`specarch-X.Y.Z.cdx.json`, `specarch-X.Y.Z.spdx.json`), and grype,
osv-scanner and govulncheck check it. Both SBOMs, each scanner's report
and the tool versions are committed under `sbom/vX.Y.Z/`, both SBOMs are
attached to the draft, its notes end with the scan's summary (the tool
versions, the result and the licences), and only then is it published.
`sh scripts/sbom.sh X.Y.Z` runs the same scanners on the Go module and the
Swift package alone, before tagging. It writes the CycloneDX SBOM and each
scanner's report to `sbom/local/X.Y.Z/` (gitignored), never into the
release records in `sbom/vX.Y.Z/`. A finding stops the release until the
owner has decided on it.

The GitHub releases of v1.1.0 (no SBOM) and v1.2.0 (the CycloneDX SBOM
alone) cannot take an asset any more; both SBOMs of each and their scan
reports are in `sbom/v1.1.0/` and `sbom/v1.2.0/`.

## Style

Prose is plain and direct. Documentation explains the decision as well as the
rule. Identifiers in YAML follow `docs/conventions.md`. Commit messages say
what changed and why, in the imperative, without tool trailers.
