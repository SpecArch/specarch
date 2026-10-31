# specarch validate

```
specarch validate <paths>...
```

Check specifications and implementation files.

Checks every specification found under the paths given: a folder that
holds `specarch.yaml` is one specification, read as a whole, including
the implementation files under its `implementation/` folder; nothing
under it is searched further, so a test folder's data files are never
read as specifications. An implementation file outside any
specification is checked on its own against the specification its
`implements` names. A file given by name must be a root file or an
implementation file; a fragment of a tree is refused with the folder
to run on instead. The records in the `records/` folder beside a
specification's folder are checked with it.

Every specification is checked against the meta-model and against the
rules JSON Schema cannot express. It reports every problem in every
file, not only the first.

Every specification found is read before any is checked, so that a
reference into another specification (`<dependency>#/<pointer>`,
through a dependency with `specification` and `version`) resolves in
the one whose `info.name` the dependency names, when it is given in
the same run: its element is checked as a local one is, and the
dependency's version against that specification's `info.version`.
A reference whose specification is not given, or whose element is
blocked there by a must question, is not an error: it is counted as
unresolved, and `gaps` lists it (ADR-112).

An estate file, `specarch-estate.yaml`, found under a folder or
named, lists the modules' specifications by their `info.name` with
the folder each is in and the version the estate holds; it brings
each module's specification into the run, read once however often
it is named, so every reference between them resolves. Each module
is checked against its entry, as `module` at the module: its folder
holds a specification, whose `info.name` is the key and whose
`info.version` is the version listed, and its operations' and jobs'
`calls` do not come back to it through the other modules' (ADR-113).

## Arguments

- `<paths>` (required; one or more): Folders and files. A folder is searched for specifications, for implementation files outside one, and for estate files.

## Reads

- `{paths}`: Root files, the files under their stage folders, and implementation files.
- `records/ beside each specification's folder`: The change, defect, release, incident, commissioning, approval and idea records of that specification.
- `the specification named in each standalone implementation file's `implements``: Read to resolve that file's references.
- `each estate's modules, at their location`: The specification of every module an estate lists, read with the estate.

## Standard output

One line per diagnostic, `file:line:column: severity: /yaml/path: rule: message [id]`, sorted by file, then line, column, path, rule, message and id.

## Standard error

A usage message on a usage error, the reason on an unreadable path, and a one-line count at the end, with the number of open questions and of unresolved references when there are any.

## Exit status

- 0: every input is valid.
- 1: at least one input has an error. Then: Settle each error it prints at the file, line and path given, then run it again.
- 2: usage error, a path could not be read, or a folder holds no specification, no implementation file and no estate. Then: Read the usage message or the reason on standard error, correct the command line or the path it names, and run it again.
