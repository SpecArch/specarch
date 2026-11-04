# specarch gaps

```
specarch gaps [--next] <paths>...
```

List the open questions and what they hold up.

Validates its input first, prints the problems of each estate
given, one line each as validate prints them, since no module's
document holds them (ADR-113), and prints the open questions document
of a specification with errors too (ADR-065), the text
`document questions` writes, from its title: the counts by
priority, the decisions each stakeholder must take and the material
each must provide, a material question counted only as the second,
the elements by origin when the specification tracks it,
the errors, one problem line each, when it has any, the decision
questions of what could be read grouped by stage in
life-cycle order and within a stage by priority then ID, then the
material questions in a group of their own, Material to provide, in
the same order (ADR-120), each with
who decides, or for material whom it is needed from, what it blocks (and for a blocked element, which keys
are missing there), the name it gives for the key it blocks with
the file and line of the element that leaves the key out, its
options, the decision that opened it when one did, with the
questions that decision answered, and so on back along the chain,
its Insight and Notes; then the references into other
specifications that are not resolved, each with where it stands,
the specification and version it names, and what it waits on (the
specification not given, or the must question that blocks the
element there); then, when
a source lists its clauses, the coverage: per such source, the
elements each clause produced, how many clauses produced nothing,
and the citations outside the listed clauses; then the outputs, one row per document target and per code target the
implementation files name, each ready, a draft (the questions and
the errors that concern it) or waiting (the errors, the questions,
and the approval that is missing or void), the questions given per
stage as how many and the first three by priority then ID.

With `--next` it prints, after the same problem lines, for each
specification given only the one open question to put to a person
next (ADR-209): ranked first by priority, must, should, could; then
by how many outputs it holds up, the documents of the outputs table
whose sections it blocks and code generation, more first; then by
how many entries it blocks, more first; then by stage in life-cycle
order; then by ID as gaps orders IDs. A line names the
specification, the question with its priority and kind, and how
many questions are open; then the question; who decides it, or for
material whom it is needed from; what it blocks, with the keys
missing there; the outputs it holds up; its options; its Insight;
and the decide command that records the answer, with the question,
the stakeholder, the folder and `--source` naming the interview
source the root file declares, for material the document source,
or saying which to declare when there is none or which to choose
when there are several. A specification without an open question
prints that it has none. The errors of an invalid specification are
printed first as problem lines, and its next question is still
ranked from what could be read. With `--json` the output object
holds `next`, one entry per specification: `specification` (the
root file), `openQuestions`, and `question`, null when none is
open, with `id`, `question`, `kind`, `priority`, `decidedBy`,
`stage`, `blocks`, `missing` (the keys missing per blocked
pointer), `holdsUp`, `options`, `insight`, `certainty` and `names`
as the question has them, `decide` (the command as arguments, the
answer as `<answer>`) and `recommendation`, always null: the agent
fills it with the answer it recommends and why before it puts the
question (`specarch help asking`).

The intent comes before any open question, on every profile
(ADR-244): a specification with requirements and no problem, as the
validator's `problem_missing` sees it, and then the first accepted
goal with no measure or no target, is printed in its place, naming
the rule and the pointer, the question in plain words, who to ask
(the owner of the system), the keys the answer is written as, and
the open question ranked first that comes after it. With `--json`
each entry holds `intent`, null when there is none, with `rule`,
`pointer`, `question` and `keys`, beside `question` as before. The
exit status stays that of the open questions and the errors alone:
nothing refuses a specification for missing intent.

## Arguments

- `<paths>` (required; one or more): Folders holding specifications, searched as validate searches them.

## Options

- `--next`: Print only the one open question to ask next per specification, ranked by what its answer unblocks, with the decide command that records the answer.

## Reads

- `{paths}`: The specifications and their implementation files.
- `records/approvals/<version>.yaml`: The approval of the specification's version, beside its folder, when there is one.

## Standard output

The problems of each estate given, then the open questions document of each specification, with its errors listed in it; with --next the errors as problem lines and each specification's next question, and with --json that question as data under next.

## Standard error

A usage message on a usage error, and the reason on an unreadable path.

## Exit status

- 0: no must or should question is open and no specification has an error.
- 1: at least one must or should question is open, or a specification or an estate has an error. Then: Answer the open must and should questions it lists with specarch decide and settle the errors, then run it again.
- 2: usage error, or a path that could not be read. Then: Read the usage message or the reason on standard error, correct the command line or the path it names, and run it again.
