# specarch approve

```
specarch approve --by <by> [--date <date>] [--dry-run] <paths>...
```

Record that the documents were read and the specification is approved.

Validates its input first and approves nothing with errors. Then
refuses while a must or should question is open, when `--by` is not a
stakeholder of the specification, when no document target is
configured, and when any configured document on disk is not what the
specification generates now, so that what was read is what is
approved. Each document is made as `document` makes it, with the
records beside the specification, the problem marks and, for the
html site, its problems, diagrams and screens, so a document that
`document` has just written is current; the questions, problems and comparison targets are the
refinement's own documents and are not among them (ADR-157). Otherwise writes `records/approvals/<version>.yaml` beside
the specification's folder: the record kind, the version, who
approved, the date, the documents that were current, and the digest
of the specification's files (the SHA-256 of every `.yaml` file under
its folder, in byte order of their relative paths, each as its path,
a zero byte, its bytes without its problem marks, and a zero byte).
A later change to any of those files voids the approval, and
`generate` says so; a problem mark, a line that starts with
`# specarch-problem:` (ADR-066), is not a change.

## Arguments

- `<paths>` (required; one or more): Folders holding specifications, searched as validate searches them.

## Options

- `--by <by>` (required): The stakeholder who read the documents and approves, by key; a role, never a person.
- `--date <date>`: The date of the approval, as YYYY-MM-DD. Today when not given.
- `--dry-run`: Check everything and write nothing; print the approval record it would write, as would write <path>.

## Reads

- `{paths}`: The specifications and their implementation files.
- `<output>/<target>.md`: Every configured document, compared with what the specification generates now.

## Writes

- `records/approvals/<version>.yaml`: The approval record, beside the specification's folder; an earlier approval of the same version is replaced.

## Standard output

The errors of an invalid specification, and one line per configured document that is missing or differs.

## Standard error

A usage message on a usage error, the reason a refusal, and one line naming the record written.

## Exit status

- 0: the approval was recorded.
- 1: refused; the specification has errors or open questions, the stakeholder is unknown, no document is configured, or a document is not current. Then: Run specarch gaps and specarch validate on the same paths and settle what they print, run specarch document for each document not current, then approve again.
- 2: usage error, or a file that could not be read or written. Then: Read the usage message or the reason on standard error, correct the command line or the path it names, and run it again.
