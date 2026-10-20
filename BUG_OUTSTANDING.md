# Outstanding bugs

Open bugs, one entry per bug, each as `BUG-NNN — <title>` with what was
seen, what was expected and how to repeat it. The next free number is
BUG-043. When the fix is on `main`, with `Fixes BUG-NNN` in its commit
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

## BUG-036 — generate ui (plain JavaScript): the screens use the design's field names on the wire whatever wireNames says

- Reported: 2026-10-20, found while writing forms and views for the plain JavaScript screens (item 1007)

`specarch-gen-ui` never reads `wireNames`: a list reads `row.cardNumber`, a filter is sent as `cardNumber`, a form sends `{ "fullName": ... }` and a view fills `data-field="fullName"`, all by the design's names. A specification whose `wireNames` is `snake_case` serves `card_number` and `full_name`, so the screens show empty cells and send bodies the service refuses, and nothing is reported. The Next.js generator maps every name through `internal/wirename`.

Expected: every name the screens read or send is the wire name `internal/wirename` gives, or, until that is built, a specification whose `wireNames` is not the design's own names is an error at the ui target saying so.

To repeat: in a copy of `spec/tests/generate-ui/project/specarch.yaml`, add `wireNames: snake_case` to `info` and rename the field `title` to `bookTitle` (in the entity, its `required` and the page's columns), then run `specarch generate ui --unapproved`; `books-list.js` lists `"bookTitle"` where the service sends `book_title`, and nothing is reported.

## BUG-038 — model: an operation's via and a role's through are not listed as references

- Reported: 2026-10-20, found while building the identity module (item 1004)

`specarch model` lists every name a reference keyword gives as a reference to the element it names (ADR-138). Its table of reference places (`referencePlaces` in `internal/validate/model.go`, and the Swift build's) has `callers` on an operation and a command, but not `via` on an operation, which names clients too, nor a role's `through`, whose keys name permissions and whose values name clients. An editor reading the model therefore draws no link from a hop to the service its `via` names, or from a narrowed grant to its applications.

Expected: `paths/*/*/via` listed as references to clients, and `roles/*/through` keys as references to permissions and their items as references to clients, in both builds.

To repeat: run `specarch model spec/tests/validate-0-3-tokens-valid` and look at `references`: `/paths/~1invoices~1{invoiceId}/delete/via/0` (naming `middleware`) and `/roles/invoice-manager/through` (naming `invoices.delete` and `admin-web`) are absent.
