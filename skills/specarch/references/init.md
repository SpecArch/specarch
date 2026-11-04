# specarch init

```
specarch init <folder> [--title <title>] [--profile <profile>] [--problem <problem>] [--goal <goal>]... [--out-of-scope <out-of-scope>]... [--dry-run]
```

Write the smallest specification that validates into a new folder.

Writes `specarch.yaml` into the folder given, making the folder when
it does not exist: the `$schema` line pinned to this release, the
meta-model version, and `info` with the title and the version the
format requires, `0.1.0`. The title is `--title`, or the folder's
own name. With `--profile` the file names that profile under
`info.profile`; without it no profile is written and the
specification is on full (ADR-210). Nothing else is written: no
entity, operation or stage to rename or delete, since a placeholder
is a definition nobody made. `specarch help start` shows the next
step, a first entity and operation that validate.

The intent comes first (ADR-244): `--problem` writes
`problem.statement`, each `--goal` a goal `GOAL-n` with status
`proposed`, and each `--out-of-scope` an entry `OUT-n`, so the
analyst answers in sentences and the keys are written for them.
When standard input is a terminal and none of the three is given,
init asks the three questions there on standard error, an empty
answer skipping one; on a pipe or in a script it asks nothing.

A folder that already holds `specarch.yaml` is refused and nothing
is written: init never overwrites a specification.

## Arguments

- `<folder>` (required): The folder the specification goes in; made when it does not exist.

## Options

- `--title <title>`: The specification's title, written as info.title; the folder's own name when left out.
- `--profile <profile>` (one of `lean`, `full`): The profile written as info.profile, lean or full; left out, no profile is written and the specification is on full.
- `--problem <problem>`: The problem the system solves, in a sentence, written as problem.statement.
- `--goal <goal>` (given once per value): An outcome that counts as success, in a sentence, written as goal GOAL-n with status proposed; repeatable.
- `--out-of-scope <out-of-scope>` (given once per value): Something the system will not do, in a sentence, written as entry OUT-n; repeatable.
- `--dry-run`: Write nothing; print would write <path> and the text it would write.

## Reads

- `{folder}/specarch.yaml`: Only whether it exists, to refuse overwriting it.

## Writes

- `{folder}/specarch.yaml`: The smallest specification that validates: the $schema line, specarch and info with title, version and, when --profile is given, profile, then the problem, goals and out-of-scope entries given or answered.

## Standard output

The path written, or with --dry-run would write <path> and the text.

## Standard error

A usage message on a usage error; the file that is already there or the reason a path could not be written; after writing, the next step.

## Exit status

- 0: the specification was written, or with --dry-run would be.
- 2: usage error, a folder that already holds specarch.yaml, or a path that could not be written. Then: Read the reason on standard error: give a folder without specarch.yaml, or a profile of lean or full, or a path that can be written, and run it again.
