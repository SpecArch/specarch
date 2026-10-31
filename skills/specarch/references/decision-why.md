# specarch decision why

```
specarch decision why <decision> <folder>
```

Print the chain of decisions a decision rests on, earliest first.

Validates its input first and prints the errors of an invalid
specification, then works on what could be read (ADR-065). Finds
the decision by its ID among the specification's decisions; refuses
when there is none of that ID, naming the ones there are. An
implementation file's decisions are not walked.

Prints a summary line, how many decisions the decision rests on
through any number of `restsOn` links, then one line per decision
of the chain, each decision after the ones it rests on and the
decision asked about last: its ID, its title, its status and date,
the decision that supersedes it when one does, the decisions it
rests on, and the ideas in records/ideas/ beside the specification
whose became names it, as "from idea" and their IDs. A decision in
the chain is printed once, where it is first
reached. A decision that rests on nothing prints the summary and its
own line.

## Arguments

- `<decision>` (required): The ID of the decision, such as ADR-012.
- `<folder>` (required): The folder holding the specification.

## Reads

- `{folder}`: The specification and its implementation files, and the ideas in records/ideas/ beside it.

## Standard output

The errors of an invalid specification, then the summary line and one line per decision of the chain, earliest first.

## Standard error

A usage message on a usage error, and the reason a refusal.

## Exit status

- 0: the chain is printed.
- 1: refused, the decision is not one of the specification's; or printed, while the specification has errors. Then: Read the refusal on standard error and give the ID of a decision it names, or run specarch validate and settle the errors.
- 2: usage error, or a folder that could not be read. Then: Read the usage message or the reason on standard error, correct the command line or the path it names, and run it again.
