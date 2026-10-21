# Outstanding bugs

Open bugs, one entry per bug, each as `BUG-NNN — <title>` with what was
seen, what was expected and how to repeat it. The next free number is
BUG-054. When the fix is on `main`, with `Fixes BUG-NNN` in its commit
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

## BUG-045 — extract javascript: a use() on the application before a mount does not reach the mounted router's routes

- Reported: 2026-10-21, found while adding a mounted gate to extract-javascript-caller-checks

Express runs the middleware an earlier `app.use(mw)` adds for every request after it, the routes of a router mounted later with `app.use('/admin', admin)` included. The reader adds only the middleware of an earlier `use` on the router that registers the route (`spec/design/commands/extract.yaml`: "of an earlier `use` on the same router"), so a permission check or a gate added by `use` on the application, or on any router above the mount, is not read for the mounted router's routes.

Expected: an earlier `use` without a path, on the router a mount is made on, is part of that mount's chain, cited where it is made, for every route under the mount.

To repeat: run `specarch extract javascript` on `spec/tests/extract-javascript-caller-checks` as its `case.yaml` does. `app.use(requireAudience(...))` at `server/src/app.ts:9` guards 5 operations, and `POST /admin/invoices/{invoiceId}/restore`, whose router is mounted at line 16, is not among them. The case's golden records this until the fix.

## BUG-048 — model (Swift): a hexadecimal integer in the document is printed as text

- Reported: 2026-10-21, found while comparing the two builds' `specarch model` on every specification in `spec/tests` and `examples`

YAML 1.2's core schema reads `0x1` as the integer 1. The Go build's `specarch model` prints it so in `document`; the Swift build prints the string `"0x1"`. The two builds print different bytes for the same specification, which the conformance suite does not see because no model case holds a hexadecimal number.

Expected: both builds print `1`, as YAML 1.2 reads it, or the same text from each with the reason written next to the choice.

To repeat: run `specarch model spec/tests/validate-theme` with each build and compare `/tokens/color/hexInt/$value/components/0`.

## BUG-049 — document: a problem in an implementation file is never marked at the entry a document shows

- Reported: 2026-10-21, found while fixing BUG-035

A document marks each problem by a Problem paragraph at the element its pointer leads to (ADR-065), but a `Mark`'s pointer leads only into the merged specification and is empty for a problem in an implementation file (`internal/generate/problems.go`). The handover, the deployment guide and the technical specification show deployments of the implementation files, yet a problem at one of them is counted in the notice under the summary and never marked at the deployment. The handover of a deployment the customer runs without `infrastructure` says of its `handover` error "None of them is in what this document covers", although the error is at the deployment its "What the hosts must provide" section describes.

Expected: a problem in an implementation file is marked at the entry of that file a document shows, and the notice counts it as concerning the document.

To repeat: run `specarch document handover --out docs project` in a copy of `spec/tests/document-handover-links-without-infrastructure`; `docs/handover.md` has the notice "None of them is in what this document covers" and no Problem paragraph at "Shop in Go: production".
