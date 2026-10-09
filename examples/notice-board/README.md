# Notice board

A small service on [dxlib](https://github.com/donnyhardyanto/dxlib), kept to
show how `specarch extract` reads a dxlib service from its two readings of
one surface:

- `sources/openapi/api.openapi.json` is the OpenAPI document dxlib emits
  (`OpenAPIAsJSON`) from the endpoints the service registers. It is what
  the running system printed, so `extract openapi` reads it as a code
  source with `reading: printed` (ADR-076).
- `sources/testdata/` is the service's Go source. `extract go` reads it by its
  syntax (`reading: parsed`, ADR-075): where each endpoint is registered,
  its handler and middlewares, the parameters the handler reads and the
  problems it answers, the notice table `NewModelDBTable` declares, the
  roles and grants its dxlib_module seed makes, the settings it reads and
  its configuration file `service/config/board.json`, read as data, and
  a gate on a setting in its session check, each citing its file and
  line.

`extract.sh <out folder>` reads both, merges them and validates the result.
Every operation ends with two citations, the document's and the source's,
and the merge leaves out the Go tree's question whether the running
system registers each one, which the document answers.

The source holds what a reader of it should notice: an endpoint open to
every caller, one that checks two privileges, privileges in dxlib_module's
capitals (`NOTICE.READ`, `GLOBAL.SET_MAINTENANCE_MODE`) mapped to
permission names by the rule of ADR-076, a handler that reads a parameter
its endpoint does not declare, a refusal whose reason is a constant, a
handler written as a function literal and a WebSocket endpoint; a role
granted `EVERYTHING`, a database password the configuration marks
sensitive, and a session check that lets every request through while
`NOTICE_BOARD_SESSIONS_ON` is false.

The board keeps no database and prints no permission table, so the
merge has no catalogue and no permission table to compare with: the
Go tree's questions whether the database holds the notice table and
whether the running system grants each seeded role stay open. A
service that has them reads them with `extract database` and `extract
permissions` and gives their trees to the merge, which answers those
questions and asks where the readings differ.

## The document and the code

`sources/testdata` holds the service's Go files and no `go.mod`: the
example is only read here, so this repository names, fetches and scans
no dependency for it, and the folder's name keeps the files out of this
repository's own module, as the go command leaves `testdata` out. A service has a `go.mod` that names its module and
requires dxlib; `extract go` reads its module path from it, to find a
handler in another package of the module. The document was made with
dxlib at commit 713e348 (after v1.126.6), the first that names each
operation's middlewares (`x-dxlib-middlewares`), by building
`sources/testdata/cmd/openapi` in a module `example.com/noticeboard` that
requires that dxlib, and running it:

    go run ./cmd/openapi > ../openapi/api.openapi.json

`cmd/openapi` is the small program dxlib's `api/OPENAPI.md`, section 6,
describes: it defines the API as the service does and prints its document.
A service built on `DXApp` gets the same with no program of its own, by
running under `DXLIB_OPENAPI_DUMP=<folder>`.
