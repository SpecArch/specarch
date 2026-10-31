# specarch idioms diff

```
specarch idioms diff <idiom> <paths>...
```

Print a shipped idiom's parts beside each override's.

Validates its input first, as idioms does. Then, for every
implementation file that overrides the named idiom, prints the
override's file, the version it was copied from and the version this
program ships, and for each part it replaces and each stack the
override renders, the shipped rendering and then the override's, as
YAML. With no override of the idiom, it says so.

## Arguments

- `<idiom>` (required): The name of a shipped idiom.
- `<paths>` (required; one or more): Folders holding specifications, searched as validate searches them.

## Reads

- `{paths}`: The specifications, their implementation files, and the idioms folder beside each.

## Standard output

The errors of an invalid specification, one line each; otherwise each override with the shipped and the overriding renderings of what it replaces.

## Standard error

A usage message on a usage error, the reason on an unreadable path, and that the idiom is not shipped.

## Exit status

- 0: the overrides were printed, or there was none.
- 2: usage error, an idiom this program does not ship, a path that could not be read, or a specification with errors. Then: Run specarch validate and settle its errors, or correct the idiom's name or the path it names, then run it again.
