# Outstanding bugs

Open bugs, one entry per bug, each as `BUG-NNN — <title>` with what was
seen, what was expected and how to repeat it. The next free number is
BUG-027. When the fix is on `main`, with `Fixes BUG-NNN` in its commit
message, the entry moves to `BUG_HISTORY.md` with the fixing commit.

## BUG-026 — validate (Go build): only one of two empty response keys is reported

- Reported: 2026-10-19, found while reviewing the response key checks

When two operations each have a response keyed by the empty string, the Go build of `validate` reports the schema error of only one of them. `propertyNameObject` in `internal/validate/schema.go` resolves the location of both `propertyNames` errors to the same object, because the schema library reuses the location slice it reports, so the `seen[path|name]` check takes the second error for the first and drops it. The Swift build reports both, so the two builds print different output for the same input.

Expected: one schema error at each operation's empty key (`/paths/~1loans~1close/post/responses/` and `/paths/~1loans~1open/post/responses/`), from both builds.

To repeat: copy `spec/tests/validate-interface-response-key-empty/specarch.yaml`, add a second operation (`openLoan` on `/loans/open`, post) whose responses are `"": { description: Odd. }` and `"200": { description: Opened. }`, and run `specarch validate` on it. The Go build prints 1 error, at `closeLoan`; `swift run --package-path swift specarch validate` prints 2.
