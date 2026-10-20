# Outstanding bugs

Open bugs, one entry per bug, each as `BUG-NNN — <title>` with what was
seen, what was expected and how to repeat it. The next free number is
BUG-049. When the fix is on `main`, with `Fixes BUG-NNN` in its commit
message, the entry moves to `BUG_HISTORY.md` with the fixing commit.

## BUG-030 — derive: a question that blocks one method of a path holds up the drafts of every method of that path

- Reported: 2026-10-19, found while reviewing the handler records' golden

When a question's `blocks` names an operation by its pointer, `#/paths/~1parcels/post`, `specarch derive` holds up the draft tests of every operation on that path, `GET /parcels` as well as `POST /parcels`, and the "not derived" line of each names the other's questions. `Block.Element()` in `internal/spec/spec.go` keeps the first two tokens of the pointer, `/paths/~1parcels`, and `blockedBy` in `internal/validate/drafts.go` compares that element with the operation's pointer by prefix, so a block on one method matches every method of the path.

Expected: a block on `#/paths/~1parcels/post` holds up the drafts of `POST /parcels` alone; `GET /parcels` is held up only by the questions that block it, or the path or stage as a whole.

To repeat: run `specarch extract go` on `spec/tests/extract-go-handlers/testdata/service` as its `case.yaml` does. The line for `listParcels` (`GET /parcels`) names the questions on `POST /parcels`'s body widths and on `storeParcel`'s dropped result and missing transaction, and the line for `storeParcel` names `listParcels`'s. The case's golden records this until the fix.

## BUG-031 — extract go: the "wrote ... questions" line leaves out the questions the handler walk asks

- Reported: 2026-10-19, found while landing the handler work (ADR-144) on the dxlib handler shapes fixture

`specarch extract go` says `wrote N operations on M paths, P permissions, R problems and K questions` from `write()` in `internal/extract/golang.go` before the handler walk runs, and the walk then asks its findings against each operation (`findings()` in `internal/extract/golang_findings.go`) as questions of their own, which neither that line nor the `handlers: followed ...` line counts. On `spec/tests/extract-go-dxlib-handler-shapes` the line says 49 questions and `spec/design/questions.yaml` holds 53, Q-50 to Q-53 being the findings; on `spec/tests/extract-go-handlers` it says 29 and the file holds 40.

Expected: the count printed is the number of questions written, the handler findings included, or the `handlers:` line says how many findings it asked.

To repeat: run `specarch extract go` on either fixture as its `case.yaml` does and count the `Q-` entries in `spec/design/questions.yaml`.

## BUG-033 — validate: the two builds kebab-case an operationId with a run of capitals differently in the test names they suggest

- Reported: 2026-10-20, found while writing the stream fixture of item 0979

For an operation `uploadJSON` with no golden test, the Go build's `test_golden_missing` suggests the test name `upload-j-s-o-n-succeeds` and the Swift build's `upload-json-succeeds`; the `test_red_missing` suggestion differs the same way. The conformance suite requires both builds to print the same output, so a fixture with such a name cannot have one golden. `kebab` in `internal/validate` splits before every capital, and its Swift twin keeps a run of capitals as one word.

Expected: one rule in both builds. `docs/conventions.md` says a test is named kebab-case after its subject; the derive command's design says a run of other characters is one hyphen and says nothing of a run of capitals, so the design decides which of the two is right and the other build follows.

To repeat: validate a specification with `operationId: uploadJSON` and no tests with both builds and compare the `test_golden_missing` lines.

## BUG-035 — document handover: a deployment with links and no infrastructure gets no links, ports or network diagram

- Reported: 2026-10-20, found while building the network diagram (ADR-137)

ADR-137 says the handover document derives its network and ports paragraphs from a deployment's `links` and `listens` whenever it has them, and prints the infrastructure sentences after them. The handover writes them only inside "What the hosts must provide", which lists only deployments with `infrastructure` (`internal/generate/handover.go`), so a deployment with links and no `infrastructure` gets neither the paragraphs nor its network diagram.

Expected: every deployment with links or listens has its links, ports and network diagram in the handover, with or without `infrastructure`.

To repeat: copy `examples/library-lending/spec`, delete the `infrastructure` block of the production deployment in `implementation/go/library-lending.go.specarch-implementation.yaml`, and run `specarch document handover --out out spec`. `out/handover.md` has no "allowed connections" sentence and no port 5671, where the example's own handover has both.

## BUG-044 — generate sql: a date-time field with generatedDefault currentTime gets the server's own text form on MariaDB

- Reported: 2026-10-21, found while writing the MariaDB scripts (ADR-167)

On MariaDB the type-rendering idiom renders a date-time as `VARCHAR(35)` holding UTC text with nine fractional digits, `2006-01-02T15:04:05.000000000Z`, the one form whose byte order is time order. Its `defaults` part names `CURRENT_TIMESTAMP` as `currentTime`, so a date-time field with `generatedDefault: currentTime` becomes `VARCHAR(35) DEFAULT CURRENT_TIMESTAMP`, and MariaDB 11.4 fills it with `2026-10-20 17:30:29`: no `T`, no fraction, no `Z`, in the server's time zone. Such a value sorts apart from the values written in the idiom's form.

Expected: the default writes the idiom's form, for instance `(DATE_FORMAT(UTC_TIMESTAMP(6), '%Y-%m-%dT%H:%i:%s.%f000Z'))`, as the MariaDB scripts write the audit instant; or the idiom says why not and a reader is warned.

To repeat: on MariaDB 11.4, `CREATE TABLE dt (id INT PRIMARY KEY, at VARCHAR(35) DEFAULT CURRENT_TIMESTAMP NOT NULL); INSERT INTO dt (id) VALUES (1); SELECT * FROM dt`.

## BUG-045 — extract javascript: a use() on the application before a mount does not reach the mounted router's routes

- Reported: 2026-10-21, found while adding a mounted gate to extract-javascript-caller-checks

Express runs the middleware an earlier `app.use(mw)` adds for every request after it, the routes of a router mounted later with `app.use('/admin', admin)` included. The reader adds only the middleware of an earlier `use` on the router that registers the route (`spec/design/commands/extract.yaml`: "of an earlier `use` on the same router"), so a permission check or a gate added by `use` on the application, or on any router above the mount, is not read for the mounted router's routes.

Expected: an earlier `use` without a path, on the router a mount is made on, is part of that mount's chain, cited where it is made, for every route under the mount.

To repeat: run `specarch extract javascript` on `spec/tests/extract-javascript-caller-checks` as its `case.yaml` does. `app.use(requireAudience(...))` at `server/src/app.ts:9` guards 5 operations, and `POST /admin/invoices/{invoiceId}/restore`, whose router is mounted at line 16, is not among them. The case's golden records this until the fix.

## BUG-046 — derive: a client named in callers by reference to the identity module gets no signature, version or deputy case

- Reported: 2026-10-21, found while building the through cases across the identity module (ADR-171)

`callerCases` in `internal/validate/clients.go` (and its Swift twin) resolves each client in `callers` with `d.client(name)`, which looks only at the module's own clients, so a reference such as `identity#/clients/reporting` gives nil and the client is skipped. Three cases are then never derived for it, although the identity module given in the same run holds the client: "refused with a bad signature from <client>" when its proof is `signedRequest`, the version case at the edge, and the deputy case for a service's own token, since `hasService` looks at the module's clients and at `via` references, not at `callers`.

Expected: a client named in `callers` by reference resolves through the identity module given in the same run, as the roles' `through` does since ADR-171, and gets the same cases as a client of the module's own; checked alone, the module keeps a test of such a case as waiting.

To repeat: give an estate whose core names `identity#/clients/<name>` in an operation's `callers`, the identity module's client being a service with `proof: { kind: signedRequest }`, and no `via` in the core; `specarch derive` on the estate lists no bad-signature and no service's-own-token case for that operation, while the same client declared in the core gets both.

## BUG-047 — derive: the token cases put "a" before the token's name with no article check

- Reported: 2026-10-21, found while recording the derive-caller-estate golden

`tokenCases` in `internal/validate/tokens.go` and `Tokens.swift` build the given text as `"a " + name + " token ..."`, so a token named `access` reads "a access token minted for another service, whose aud does not name this one". The same pattern is used in the other token cases of both builds.

Expected: "an" before a name that starts with a vowel sound, in both builds, the same text from each.

To repeat: run `specarch derive` on `spec/tests/derive-caller-estate` as its `case.yaml` does. The golden `expected/core/tests/delete-invoice-refused-with-a-token-for-another-audience/test.yaml`, and the same test in `validate-caller-identity-cases` and `validate-caller-identity-case-waits`, record this until the fix; those three goldens move with it.

## BUG-048 — model (Swift): a hexadecimal integer in the document is printed as text

- Reported: 2026-10-21, found while comparing the two builds' `specarch model` on every specification in `spec/tests` and `examples`

YAML 1.2's core schema reads `0x1` as the integer 1. The Go build's `specarch model` prints it so in `document`; the Swift build prints the string `"0x1"`. The two builds print different bytes for the same specification, which the conformance suite does not see because no model case holds a hexadecimal number.

Expected: both builds print `1`, as YAML 1.2 reads it, or the same text from each with the reason written next to the choice.

To repeat: run `specarch model spec/tests/validate-theme` with each build and compare `/tokens/color/hexInt/$value/components/0`.
