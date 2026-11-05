# specarch plan

```
specarch plan <folder> [--release <release>] [--next] [--brief <brief>] [--export <export>] [--import <import>] [--dry-run]
```

Propose, order and brief the work packages, and round-trip them with a tracker.

Validates its input first and prints the errors of an invalid
specification, then works on what could be read (ADR-065), and exits
1 then. Reads the work-package records beside the specification as
validate does (ADR-245, ADR-246).

With no mode it proposes packages. Per release, earliest first, it
takes the accepted requirements of that release that no package not
withdrawn holds: each planned release, or the one `--release` names,
which must have a record and then takes the accepted requirements
that name no release as well. Requirements met by a common design
element, a decision left aside, go into one group; groups that need
each other through the reference graph are merged; the rest are
ordered so a group comes after the package that builds what its
elements need. Each is written as a record with status proposed
under the next free WP number, with its release, a title, its
requirements, after and why. It prints the requirements it left out
and why, then one line per package proposed: its ID, requirements,
after and title. A release with nothing to plan says so.

With `--next` it prints the first package by ID that is planned or
ready, whose after packages are each done or withdrawn and whose
readiness check (the reasons work_package_not_ready names and
work_package_order) finds nothing: its title, release, status,
assignee, requirements and after, and the brief command. When none
can start it prints each candidate with what it waits on.

With `--brief <package>` it prints one page in Markdown: whether the
package can start and why not, its requirements with their acceptance
criteria, the design elements that meet them, the tests that verify
them, the goals the needs of its requirements serve, the out-of-scope
entries that rule out one of those needs and the others by ID, the
quality goals in rank order, and what it changes in operation: the
settings, migrations, jobs, processes and monitors it builds or
reaches.

With `--export csv` or `--export json` it prints every package for a
tracker: id, title, release, status, assignedTo, estimatedHours,
spentHours, after and requirements; a CSV is quoted as RFC 4180 quotes,
with LF line breaks, a list of IDs in one cell separated by a comma
and a space; a JSON list is an array.

With `--import <file>`, a .csv or .json file as the export writes it,
it writes into each record what the tracker owns and the file
changes: status, assignedTo and effort, an empty cell taking the key
out, the record's other keys and comments kept; it prints one line
per record written with each change. It warns, one line each with the
file and row first, at a title, release, after or requirements the
file changes (naming both values; the record keeps its own), at a
status that is not a work package's, at hours that are not a positive
number, at a row with no ID, a repeated ID or an ID with no record,
and at a column it does not read.

## Arguments

- `<folder>` (required): The folder holding the specification.

## Options

- `--release <release>`: The release to propose packages for, one with a record; its accepted requirements and those naming no release are planned.
- `--next`: Print the first planned or ready package that can start now, or what each waits on.
- `--brief <brief>`: Print one package's brief, by its ID, for whoever takes it.
- `--export <export>` (one of `csv`, `json`): Print every package for a tracker, as csv or json.
- `--import <import>`: Read a tracker file, .csv or .json as the export writes it, and write back the status, assignee and effort it changes.
- `--dry-run`: Propose or import as usual and write nothing; list each record it would write as would write <path>.

## Reads

- `{folder}`: The specification and its implementation files, and the records beside it: work packages, releases and test runs.
- `{import}`: The tracker file, for --import.

## Writes

- `records/work-packages/WP-<n>.yaml`: Each package proposed, under the next numbers, refused when a file of that name exists; with --import, each record whose status, assignee or effort the file changes.

## Standard output

The errors of an invalid specification; then the requirements left out and one line per package proposed, or the next package or what each waits on, or the brief, or the export, or one line per record the import writes and one per warning.

## Standard error

A usage message on a usage error, the reason a refusal, and one line saying how many records were written.

## Exit status

- 0: done; for --next, a package can start.
- 1: refused (a release with no record, a package with no record), no package can start for --next, or the specification has errors. Then: Read what standard output or standard error names: record the release, give the ID of a package it lists, settle what each package waits on, or run specarch validate and settle the errors.
- 2: usage error, a folder or tracker file that could not be read, or a record that could not be written. Then: Read the usage message or the reason on standard error, correct the command line or the path it names, and run it again.
