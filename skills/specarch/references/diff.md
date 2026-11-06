# specarch diff

```
specarch diff <old> <new>
```

Compare two versions of a specification and check the release between them.

Validates both specifications first and prints the errors of one
that has them; a specification with errors is compared as far as it
can be read, and the command then exits 1 (ADR-065). Then lists
every element added, changed
or removed between them, one line each in pointer order, as
`<impact> <added|changed|removed> <pointer>`, with the change that
decided the impact of a changed element of the public interface
after it. The impact follows Semantic Versioning: an element inside
the system is patch whatever happens to it; on the public interface
an addition is minor, a removal major, and a change the largest of
its changes, a change that cannot be shown safe counting as major.
After the change list, for each workflow of the new version that
keeps its as-is under asIs, one line per part the move from the
as-is to the to-be reaches, as `transition <pointer>: <part>:
<items>`, the items joined by semicolons: the parts workflow, steps,
approvers and performers, operations, pages and tests, the same
list the changes document writes (ADR-236).

Then it checks the release of the new version, read from the
records beside the new folder: that its version steps from the old
one by at least the largest impact (version), and that a change
request or defect it includes names every element in the list
(covered). When both folders carry the same `info.version`, as the
approved commit and the working tree do inside one pre-release
during a round of acceptance, the version check is left out and
only the coverage is checked, against the release record of that
version (ADR-224). That holds only for a version not yet released:
when a release record beside the old folder marks that exact
version released, the version check fails, naming the record and
the version to step to, since a specification that keeps the
version of a shipped release would ship the next one under the
same number. A version that is not a semantic version fails the
version check, naming the version and the folder, and the release
is not checked further. Each failed check is one line starting
`error:`; an
element the records cannot be shown to name because the release
includes an ID kept in a tracker is a line starting `warning:`.

## Arguments

- `<old>` (required): The folder of the earlier specification, normally the previous release's tag checked out on its own, or the approval record's commit checked out on its own for a second approval.
- `<new>` (required): The folder of the later specification, normally the working tree.

## Reads

- `{old}`: The earlier specification.
- `{new}`: The later specification.
- `records/ beside {new}`: The release record of the new version and the change and defect records it includes.

## Standard output

The errors of an invalid specification, one line each, then the change list, then the transition lines of each workflow that keeps an as-is, then one line per failed check.

## Standard error

A usage message on a usage error, and the reason on an unreadable path.

## Exit status

- 0: every check passes.
- 1: a check fails, there is no release record for the new version, or a specification has errors. Then: Read the checks it prints as failing; write the release record for the new version or settle the change a check names, then run it again.
- 2: usage error, or a path that could not be read. Then: Read the usage message or the reason on standard error, correct the command line or the path it names, and run it again.
