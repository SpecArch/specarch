# Outstanding bugs

Open bugs, one entry per bug, each as `BUG-NNN — <title>` with what was
seen, what was expected and how to repeat it. The next free number is
BUG-054. When the fix is on `main`, with `Fixes BUG-NNN` in its commit
message, the entry moves to `BUG_HISTORY.md` with the fixing commit.

## BUG-049 — document: a problem in an implementation file is never marked at the entry a document shows

- Reported: 2026-10-21, found while fixing BUG-035

A document marks each problem by a Problem paragraph at the element its pointer leads to (ADR-065), but a `Mark`'s pointer leads only into the merged specification and is empty for a problem in an implementation file (`internal/generate/problems.go`). The handover, the deployment guide and the technical specification show deployments of the implementation files, yet a problem at one of them is counted in the notice under the summary and never marked at the deployment. The handover of a deployment the customer runs without `infrastructure` says of its `handover` error "None of them is in what this document covers", although the error is at the deployment its "What the hosts must provide" section describes.

Expected: a problem in an implementation file is marked at the entry of that file a document shows, and the notice counts it as concerning the document.

To repeat: run `specarch document handover --out docs project` in a copy of `spec/tests/document-handover-links-without-infrastructure`; `docs/handover.md` has the notice "None of them is in what this document covers" and no Problem paragraph at "Shop in Go: production".
