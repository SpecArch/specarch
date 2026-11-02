# specarch decide

```
specarch decide <question> <folder> --by <by> --answer <answer> [--source <source>] [--record <record>] [--date <date>] [--dry-run]
```

Answer an open question with a decision record and take the question out.

Validates its input first and prints the errors of an invalid
specification, then works on what could be read (ADR-065). Finds the
question by its ID in the merged `questions` section; refuses when
there is none of that ID, naming the open ones, when `--by` is not a
stakeholder of the specification, when `--source` names a source the
root file does not declare, when `--record` names a record that is
not a communication record, not an answer, one that does not link
the question or one said by another role than `--by`, since the
decision that answers a question is decided by the role that
answered it (ADR-220), when the specification tracks origin and
neither `--source` nor `--record` is given, since the answer must
then be citable, and when `stages` does not list design, where
decisions live (ADR-155). `--source` and `--record` together are a
usage error: a decision cites the one or the other.

Writes the decision record as `design/decisions/<ID>.yaml`, a file of
its own, under the next ID: `ADR-` and the largest number any
decision carries plus one, at least three digits, `ADR-001` for the
first. The record is accepted, dated `--date` or today, titled with
the question, its context the question with its options, what it
blocked and its Insight, its decision the answer, its consequences
the entries the question blocked, which are to be written from the
answer, `answers` the question's ID and `decidedBy` the stakeholder;
with `--source` it is stated, citing that source with the answer as
what it says; with `--record` it is stated, citing the communication
record with the answer as what it says, which `specarch log` prints
as the line to run after recording an answer (ADR-220). When the question blocked a decision the
specification holds, the record rests on it: `restsOn` names each
such decision once, in the order of `blocks`, on meta-model 1.0 and
later, which have the key, and the consequences say so (ADR-208).

Takes the question's entry out of the file that holds it by editing
the text, as the marks are written (ADR-066): the lines from the
entry's key to the last line of its value, with the problem marks
above it, and the `questions` key with them when the entry was its
only one; a file left with nothing but blank and comment lines is
removed. An entry not written in block style, whose key does not
start its line, is refused with the line, to be taken out by hand.
The edited text is read again before it is written, and refused
when it does not hold every other entry the file held.

Prints one line per entry the question blocked, so that the agent
writes them: for a pointer, the file and line of the deepest entry
that exists, the pointer, and the keys missing there; for a stage or
a section, its name; each ending with the origin and decision the
entry is to carry. For a pointer into a decision it prints instead
that the new record rests on it, that the decision keeps its own
origin, since no decision is decided by another (origin_decision),
or that it is superseded by the new record when the answer replaces
it, and the keys to write only when keys of it are missing; before
meta-model 1.0 it says to add `restsOn` after `specarch upgrade`
(ADR-208). A must question's covering ends with the question,
so the keys it covered are errors until they are written; `validate`
holds the agent to it.

## Arguments

- `<question>` (required): The ID of the open question, such as Q-12.
- `<folder>` (required): The folder holding the specification.

## Options

- `--by <by>` (required): The stakeholder who decided, by key; a role, never a person. With --record, the role the record names as from.
- `--answer <answer>` (required): The answer, as the stakeholder gave it; the record's decision.
- `--source <source>`: The key of the source the answer rests on, declared in the root file; an interview for an answer given in words, a document for material provided. One of --source and --record is required when the specification tracks origin.
- `--record <record>`: The communication record of the answer given, by ID, written by specarch log with type answer and the question under its links; the decision cites it. Not with --source.
- `--date <date>`: The date of the decision, as YYYY-MM-DD. Today when not given.
- `--dry-run`: Check the question and the answer and write nothing; print the blocked entries, then the decision record it would write and the question file it would edit or remove, as would write <path> and would remove <path>.

## Reads

- `{folder}`: The specification and its implementation files.

## Writes

- `design/decisions/<ID>.yaml`: The decision record, under the next ID; refused when a file of that name exists.
- `the file that holds the question`: Without the question's entry, and without the questions key when the entry was its only one; removed when nothing but blank and comment lines is left.

## Standard output

The errors of an invalid specification, then one line per entry the question blocked, with the file and line, the pointer and the keys missing there.

## Standard error

A usage message on a usage error, the reason a refusal, and one line naming the record written, the file the question left and what to do next.

## Exit status

- 0: the record is written and the question taken out.
- 1: refused; the question, the stakeholder or the source is unknown, the record is not an answer record linking the question and said by the stakeholder, the specification tracks origin and neither a source nor a record is given, design is not a stage, or the entry cannot be edited as text; or written, while the specification has errors. Then: Read the refusal on standard error and correct the question, stakeholder, source or record it names, then run it again; when the record was written while the specification has errors, run specarch validate and settle them.
- 2: usage error, or a file that could not be read or written. Then: Read the usage message or the reason on standard error, correct the command line or the path it names, and run it again.
