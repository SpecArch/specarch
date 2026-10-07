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

Every `*.specarch-design.yaml` file in the repository must validate against the
meta-model. The check runs with a pinned, Apache-2.0 validator and needs only
a Go toolchain:

    go run github.com/santhosh-tekuri/jsonschema/cmd/jv@v0.7.0 -f \
      schema/specarch-design-0.1.schema.json examples/library-lending/library-lending.specarch-design.yaml

Implementation files (`*.specarch-implementation.yaml`) are checked the same way
against `schema/specarch-implementation-0.1.schema.json`. The schema itself must
compile, which the same command checks first.

Dependencies are added only when their licence is OSI-approved and their SBOM
scan (syft, then grype and osv-scanner; govulncheck for Go) is clean, or when
the only finding is proven unreachable by govulncheck and the owner has
accepted it. The result goes in the commit message. The dependency record for
the validator above, checked 2026-10-07:

| Module | Version | Licence |
|---|---|---|
| github.com/santhosh-tekuri/jsonschema/cmd/jv | v0.7.0 | Apache-2.0 |
| github.com/santhosh-tekuri/jsonschema/v6 | v6.0.1 | Apache-2.0 |
| github.com/spf13/pflag | v1.0.5 | BSD-3-Clause |
| gopkg.in/yaml.v3 | v3.0.1 | MIT and Apache-2.0 |
| golang.org/x/text | v0.14.0 | BSD-3-Clause |

grype and osv-scanner report one advisory, GO-2026-5970 (infinite loop on
invalid input in `golang.org/x/text/unicode/norm`, fixed in x/text 0.39.0).
govulncheck run on the jv module reports 0 affecting vulnerabilities: jv does
not call that package. The validator CLI on the roadmap replaces this
dependency with the repository's own module, pinned to a fixed x/text.

## Style

Prose is plain and direct. Documentation explains the decision as well as the
rule. Identifiers in YAML follow `docs/conventions.md`. Commit messages say
what changed and why, in the imperative, without tool trailers.
