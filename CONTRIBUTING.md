# Contributing

SpecArch is developed and owned by one person. Anyone who wants to
contribute contacts the owner first, by opening an issue or by writing to the
owner, and waits for an answer before writing the change. The owner decides
each contribution on its own, yes or no, and may ask for a signed agreement
before accepting one. A pull request or patch that arrives without that
first contact is closed, whatever its quality.

Agents working in this repository never merge or accept an outside pull
request or patch themselves. They report it to the owner, who decides.

Issues, questions and suggestions are welcome at any time.

## What belongs here

The public repository holds generic material only: the meta-model, the
validator, the generators, the documentation and example projects. An example
must be free of any employer's or client's names, data, code or internal
detail. Any specification written for a particular organisation stays in that
organisation's own repository, where it points at this one as a dependency.

## Checks before a change is proposed

These are the checks CI runs on every change:

    go build ./...
    go test ./...
    govulncheck ./...
    go run ./cmd/specarch validate spec examples
    for target in techspec requirements testplan traceability deployment commissioning; do go run ./cmd/specarch document $target --check spec examples || exit 1; done
    swift build --package-path swift && swift test --package-path swift

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
specification is followed by `specarch document <target> spec examples` for
every built document target, and the regenerated documents are committed
with it.

Dependencies are added only when their licence is OSI-approved and their SBOM
scan (syft, then grype and osv-scanner; govulncheck for Go) is clean, or when
the only finding is proven unreachable by govulncheck and the owner has
accepted it. The result goes in the commit message, and each library is
listed with its version and licence under `libraries` in
`spec/implementation/go/specarch.go.specarch-implementation.yaml`.

## Style

Prose is plain and direct. Documentation explains the decision as well as the
rule. Identifiers in YAML follow `docs/conventions.md`. Commit messages say
what changed and why, in the imperative, without tool trailers.
