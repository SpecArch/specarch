# specarch guide

```
specarch guide [--agent]
```

Print the order of work, for a person or, with --agent, for an agent.

Without --agent it prints the steps of the work in order, from a
specification written first to code generated and checked, then
each command's synopsis and summary.

With --agent it prints the guide for an agent, as Markdown: what
SpecArch is; the formats it writes and reads; the workflow (pin the
program, specification first, validate after every edit, gaps and
decide for the open questions, document and approve, generate and
generate --check, extract and merge for a system that exists,
upgrade when the formats move); how to read a problem line and the
problems file; when to stop and ask the owner; the limits (no
promise of complete generation, hand-written islands); the
conventions of every command (--json, --dry-run, exit statuses, no
prompts) and the error codes of --json; every command with its exit
statuses; then the plug-ins found on PATH. Each fact is the one
capabilities --json lists under the same name, and the repository's
AGENTS.md holds the same text up to the plug-ins.

## Options

- `--agent`: Print the guide for an agent, as Markdown, in place of the order of work for a person.

## Reads

- `PATH`: The folders searched for specarch-gen-* plug-ins, for --agent; each is run with --capabilities.

## Standard output

The order of work and the commands, or with --agent the agent's guide in Markdown.

## Standard error

A usage message on a usage error.

## Exit status

- 0: printed.
- 2: usage error. Then: Run specarch guide with no argument or with --agent only.
