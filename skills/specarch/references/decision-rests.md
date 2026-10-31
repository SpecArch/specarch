# specarch decision rests

```
specarch decision rests <decision> <folder>
```

Print the decisions that rest on a decision, nearest first.

Validates its input first and prints the errors of an invalid
specification, then works on what could be read (ADR-065). Finds
the decision by its ID among the specification's decisions; refuses
when there is none of that ID, naming the ones there are. An
implementation file's decisions are not walked.

Prints a summary line, how many decisions rest on the decision
through any number of `restsOn` links, then the decision's own line
and one line per decision that rests on it, the ones resting on it
directly first, then the ones resting on those, each line as
`decision why` prints it, with the ideas it came from. A decision is printed once, where it is
first reached. A decision nothing rests on prints the summary and
its own line.

## Arguments

- `<decision>` (required): The ID of the decision, such as ADR-012.
- `<folder>` (required): The folder holding the specification.

## Reads

- `{folder}`: The specification and its implementation files, and the ideas in records/ideas/ beside it.

## Standard output

The errors of an invalid specification, then the summary line, the decision's line and one line per decision that rests on it, nearest first.

## Standard error

A usage message on a usage error, and the reason a refusal.

## Exit status

- 0: the decisions are printed.
- 1: refused, the decision is not one of the specification's; or printed, while the specification has errors. Then: Read the refusal on standard error and give the ID of a decision it names, or run specarch validate and settle the errors.
- 2: usage error, or a folder that could not be read. Then: Read the usage message or the reason on standard error, correct the command line or the path it names, and run it again.
