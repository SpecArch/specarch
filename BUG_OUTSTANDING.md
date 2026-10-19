# Outstanding bugs

Open bugs, one entry per bug, each as `BUG-NNN — <title>` with what was
seen, what was expected and how to repeat it. The next free number is
BUG-033. When the fix is on `main`, with `Fixes BUG-NNN` in its commit
message, the entry moves to `BUG_HISTORY.md` with the fixing commit.

## BUG-028 — extract go: a route whose path is a package-level constant, or whose Match method list is a package-level list, is a question instead of an operation

- Reported: 2026-10-19, from reading a gateway service whose routes are registered through the module's own constants
- Design: `docs/gateway.md`, section 3; ADR-143

A Go module registers its routes on a gin router with the paths and the method list declared at package level. The fixture is this one file, `main.go`, complete, as the only file of a module committed in a repository of its own and read with `specarch extract go --out <folder> <repository>`; the line numbers cited here and in BUG-029 are its:

    package main

    import (
    	"net/http"

    	"github.com/gin-gonic/gin"
    )

    const (
    	healthLivePath  = "/live"
    	healthReadyPath = "/ready"
    	metricsPath     = "/metrics"
    )

    var healthMethods = []string{http.MethodGet, http.MethodHead}

    func main() {
    	r := gin.New()
    	r.Match(healthMethods, healthLivePath, live)
    	r.Match([]string{"GET", "HEAD"}, healthReadyPath, ready)
    	r.Match([]string{"GET", "HEAD"}, "/started", started)
    	r.GET(metricsPath, metrics)
    	r.GET("/version", version)
    	r.NoRoute(proxy)
    	r.Run(":8080")
    }

    func live(c *gin.Context)    { c.Status(204) }
    func ready(c *gin.Context)   { c.Status(204) }
    func started(c *gin.Context) { c.Status(204) }
    func metrics(c *gin.Context) { c.String(200, "") }
    func version(c *gin.Context) { c.String(200, "") }

    // proxy forwards every request no route answers to the service the
    // token's audience names.
    func proxy(c *gin.Context) {
    	c.Status(502)
    }

Expected 5 operations. `specarch extract go` (0.7.0) writes 2, the two whose path is a literal, and asks a must question for each of the other three: "registers a route with Match on methods healthMethods, which is not a literal list" and "registers a route whose path healthReadyPath the reader does not read as a literal", at `main.go:19`, `main.go:20` and `main.go:22`.

Cause: `stringValue` in `internal/extract/golang.go` reads a string literal, literals joined by `+` and a name a call bound, and never a package-level `const`; `register` in `internal/extract/golang_routers.go` reads `Match`'s methods only from a composite literal written in the call, never from a package-level `var` holding one. Go gives a constant one value at compile time, and a list assigned once and never changed is as plain to syntax. The fix is ADR-143's rule: a package-level string constant is the literal it names, and a package-level list is a literal list when nothing in the package assigns to it, appends to it or takes its address.

## BUG-029 — extract go: a fallback handler is dropped with no question and no line

- Reported: 2026-10-19, from reading a gateway service whose forwarding lives in gin's NoRoute handler
- Design: `docs/gateway.md`, section 3; ADR-143

The same module registers `r.NoRoute(proxy)` (`main.go:24` of the fixture listed in BUG-028), the handler every request no route matches is given to, which is where the service forwards by the token's audience. `specarch extract go` (0.7.0) writes nothing for it: no operation, no question, no line, and the registration is not counted ("routers: counted 5 registrations" for five routes and the fallback). The same holds for gin's `NoMethod`, chi's `NotFound` and `MethodNotAllowed`, gorilla/mux's `NotFoundHandler` and `MethodNotAllowedHandler` and echo's `HTTPErrorHandler`, none of which `internal/extract/golang_routers.go` names.

Expected: a must question citing the call, as ADR-075 asks for every registration the reader knows on a surface it reads and cannot write, blocking the design stage until the `gateways` section exists and `gateways` after (the validator refuses a `blocks` entry that names no section), and the call counted as a registration. A route whose path ends in a catch-all (`/*path`), which prints a line today, is the other way the same handler is written and takes the same question.

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
