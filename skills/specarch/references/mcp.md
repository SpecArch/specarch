# specarch mcp

```
specarch mcp
```

Serve every command as a tool of the Model Context Protocol, over standard input and output.

Runs a server of the Model Context Protocol (MCP), revision
2025-11-25, on the stdio transport: it reads JSON-RPC 2.0 messages
from standard input, one per line, and writes each answer on
standard output, one per line, and nothing else there. It runs until
standard input ends, then exits 0. An agent's client starts it in
the folder the paths are relative to (ADR-214).

It answers `initialize` with the revision the client asked for when
it is one of 2025-11-25, 2025-06-18, 2025-03-26 and 2024-11-05, and
with 2025-11-25 otherwise; the tools capability; the server's name
and version; and short instructions naming the guide tool. It
answers `ping`, `tools/list` and `tools/call`; takes the
notifications and answers none; and answers any other method,
`server/discover` among them, with the JSON-RPC error -32601, so a
client that tries the stateless 2026-07-28 revision first falls back
to `initialize`.

`tools/list` lists the tools `specarch capabilities --json` lists,
this command's own entry left out, each with its `name`, `title`,
`description`, `inputSchema` and `annotations`: the same entries,
made from the same command definitions, so the server can offer no
input the program does not take. The plug-ins on PATH are found once,
when the server starts: a plug-in installed later is listed after
the client starts the server again.

`tools/call` runs the tool's command in this program, as `specarch
<command> <inputs> --json` would run, and answers with the JSON
object that run prints on standard output as `structuredContent`
and as text, and, on exit status 2, the error object of standard
error as a second text. Exit status 0 and 1 are results, 1 being a
command that ran and found what it reports; exit status 2 sets
`isError`. The inputs are given in the order of the command's
synopsis: the arguments that are not repeated, each option as
`--<name> <value>` (a boolean only when true, a list once per
value), then the repeated arguments. An input the tool does not
take, a required input left out, a value of the wrong type, or an
argument that starts with `-` is answered with `isError` and the
reason, before anything runs; a tool that is not listed is the
JSON-RPC error -32602. A line that is not JSON is the error -32700,
and a message that is not a JSON-RPC 2.0 request, a batch included,
-32600.

## Reads

- `standard input`: The JSON-RPC 2.0 messages of the client, one per line.
- `PATH`: The folders searched for specarch-gen-* plug-ins, for tools/list and for a generate tool; each is run with --capabilities.
- `the paths of each call`: What the command called reads.

## Writes

- `the paths of each call`: What the command called writes; a command that writes is listed with destructiveHint, and takes --dry-run.

## Standard output

The JSON-RPC 2.0 answers, one per line, and nothing else.

## Standard error

A usage message on a usage error; otherwise nothing.

## Exit status

- 0: standard input ended.
- 2: usage error, --json given, or standard input or output that could not be read or written. Then: Run specarch mcp with no argument and without --json: it speaks JSON-RPC on standard output itself. Configure the agent's client to start it as a stdio server.
