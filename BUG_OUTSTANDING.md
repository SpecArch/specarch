# Outstanding bugs

Open bugs, one entry per bug, each as `BUG-NNN — <title>` with what was
seen, what was expected and how to repeat it. The next free number is
BUG-030. When the fix is on `main`, with `Fixes BUG-NNN` in its commit
message, the entry moves to `BUG_HISTORY.md` with the fixing commit.

## BUG-028 — extract go: a route whose path is a package-level constant, or whose Match method list is a package-level list, is a question instead of an operation

- Reported: 2026-10-19, from reading a gateway service whose routes are registered through the module's own constants
- Design: `docs/gateway.md`, section 3; ADR-143

A Go module registers its routes on a gin router with the paths and the method list declared at package level:

    const (
        healthLivePath  = "/live"
        healthReadyPath = "/ready"
        metricsPath     = "/metrics"
    )

    var healthMethods = []string{http.MethodGet, http.MethodHead}

    r.Match(healthMethods, healthLivePath, live)
    r.Match([]string{"GET", "HEAD"}, healthReadyPath, ready)
    r.Match([]string{"GET", "HEAD"}, "/started", started)
    r.GET(metricsPath, metrics)
    r.GET("/version", version)

Expected 5 operations. `specarch extract go` (0.7.0) writes 2, the two whose path is a literal, and asks a must question for each of the other three: "registers a route with Match on methods healthMethods, which is not a literal list" and "registers a route whose path healthReadyPath the reader does not read as a literal", at `main.go:19`, `main.go:20` and `main.go:22`.

Cause: `stringValue` in `internal/extract/golang.go` reads a string literal, literals joined by `+` and a name a call bound, and never a package-level `const`; `register` in `internal/extract/golang_routers.go` reads `Match`'s methods only from a composite literal written in the call, never from a package-level `var` holding one. Go gives a constant one value at compile time, and a list assigned once and never changed is as plain to syntax. The fix is ADR-143's rule: a package-level string constant is the literal it names, and a package-level list is a literal list when nothing in the package assigns to it, appends to it or takes its address.

## BUG-029 — extract go: a fallback handler is dropped with no question and no line

- Reported: 2026-10-19, from reading a gateway service whose forwarding lives in gin's NoRoute handler
- Design: `docs/gateway.md`, section 3; ADR-143

The same module registers `r.NoRoute(proxy)`, the handler every request no route matches is given to, which is where the service forwards by the token's audience. `specarch extract go` (0.7.0) writes nothing for it: no operation, no question, no line, and the registration is not counted ("routers: counted 5 registrations" for five routes and the fallback). The same holds for gin's `NoMethod`, chi's `NotFound` and `MethodNotAllowed`, gorilla/mux's `NotFoundHandler` and `MethodNotAllowedHandler` and echo's `HTTPErrorHandler`, none of which `internal/extract/golang_routers.go` names.

Expected: a must question citing the call, as ADR-075 asks for every registration the reader knows on a surface it reads and cannot write, blocking the design stage until the `gateways` section exists and `gateways` after (the validator refuses a `blocks` entry that names no section), and the call counted as a registration. A route whose path ends in a catch-all (`/*path`), which prints a line today, is the other way the same handler is written and takes the same question.
