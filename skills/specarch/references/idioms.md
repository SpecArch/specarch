# specarch idioms

```
specarch idioms <paths>...
```

List the idioms each implementation file uses, and how.

Validates its input first and prints nothing but the errors from a
specification that has them. Then prints, for every implementation
file of every specification, its path and one line per idiom: every
shipped idiom and every one of the project's own that applies to the
file, by its stacks and by the keywords the specification uses, with
its version and how it applies: shipped, overridden (by which file,
copied from which version, replacing which parts), the project's own,
or excluded; an override's or an exclusion's reason follows.

## Arguments

- `<paths>` (required; one or more): Folders holding specifications, searched as validate searches them.

## Reads

- `{paths}`: The specifications, their implementation files, and the idioms folder beside each.

## Standard output

The errors of an invalid specification, one line each; otherwise each implementation file's path and its idioms, one per line.

## Standard error

A usage message on a usage error, and the reason on an unreadable path.

## Exit status

- 0: the idioms were listed.
- 2: usage error, a path that could not be read, or a specification with errors. Then: Run specarch validate and settle its errors, or correct the path it names, then run it again.
