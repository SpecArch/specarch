# specarch approve

```
specarch approve --by <by> [--session <session>] [--date <date>] [--dry-run] <paths>...
```

Record that the documents were read and the specification is approved.

Validates its input first and approves nothing with errors. Then
refuses while a must or should question is open, when `--by` is not a
stakeholder of the specification, when `--by` was never heard (the
rule unheard: no communication record names it as from or to, no
session or test session lists it under present, no interview lists it
among its participants and no decision names it under decidedBy,
whether or not anything relies on it; ADR-239), when `--session` names
no session, test session or communication record, or one held under
two of those kinds (record_ref), or one that does not list `--by`
(approval_session), when no document target is
configured, and when any configured document on disk is not what the
specification generates now, so that what was read is what is
approved. Each document is made as `document` makes it, with the
records beside the specification, the problem marks and, for the
html site, its problems, diagrams and screens, so a document that
`document` has just written is current; the questions, problems and comparison targets are the
refinement's own documents and are not among them (ADR-157). Otherwise writes `records/approvals/<version>.yaml` beside
the specification's folder: the record kind, the version, who
approved, the date, the session given with `--session`, the
documents that were current, the digest
of the specification's files (the SHA-256 of every `.yaml` file under
its folder, in byte order of their relative paths, each as its path,
a zero byte, its bytes without its problem marks, and a zero byte),
and the commit the folder is at when it is inside a git repository
and every file under it is committed, so that `diff` between that
commit and the working tree later shows the approver only what
changed (ADR-224); a folder with changes not committed gets no
commit, with a line on standard error saying so. Without `--session`
it warns on standard error that the approval names no session, and
still writes the record.
A later change to any of those files voids the approval, and
`generate` says so; a problem mark, a line that starts with
`# specarch-problem:` (ADR-066), is not a change.

## Arguments

- `<paths>` (required; one or more): Folders holding specifications, searched as validate searches them.

## Options

- `--by <by>` (required): The stakeholder who read the documents and approves, by key; a role, never a person.
- `--session <session>`: The session, test session or communication record in which the role read the documents, by ID; it must list the role under present, or as from or to of a communication.
- `--date <date>`: The date of the approval, as YYYY-MM-DD. Today when not given.
- `--dry-run`: Check everything and write nothing; print the approval record it would write, as would write <path>.

## Reads

- `{paths}`: The specifications and their implementation files.
- `<output>/<target>.md`: Every configured document, compared with what the specification generates now.
- `records/`: The records beside the specification: the communications, sessions and test sessions that show the role was heard, and the one --session names.

## Writes

- `records/approvals/<version>.yaml`: The approval record, beside the specification's folder, with the session --session names and the commit the folder is at when it is committed in a git repository; an earlier approval of the same version is replaced.

## Standard output

The errors of an invalid specification, and one line per configured document that is missing or differs.

## Standard error

A usage message on a usage error, the reason a refusal, one line when the folder has changes not committed and so no commit is recorded, one warning when no role has confirmed that the stakeholder list is complete (stakeholdersComplete), one warning when no session is named (--session), and one line naming the record written.

## Exit status

- 0: the approval was recorded; a warning on standard error when no role has confirmed that the stakeholder list is complete (stakeholdersComplete), and one when no session is named.
- 1: refused; the specification has errors or open questions, the stakeholder is unknown or was never heard, the session is not a record that lists it, no document is configured, or a document is not current. Then: Run specarch gaps and specarch validate on the same paths and settle what they print; for a role never heard, record the session in which it read the documents under records/sessions/ with the role under present, or what it said with specarch log; run specarch document for each document not current, then approve again with --session naming that record.
- 2: usage error, or a file that could not be read or written. Then: Read the usage message or the reason on standard error, correct the command line or the path it names, and run it again.
