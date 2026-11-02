# specarch log

```
specarch log <folder> [--direction <direction>] [--from <from>] [--to <to>] [--at <at>] [--channel <channel>] [--type <type>] [--words <words>] [--form <form>] [--reply <reply>] [--person <person>] [--recorded-by <recorded-by>] [--session <session>] [--source <source>] [--evidence <evidence>]... [--question <question>]... [--decision <decision>]... [--element <element>]... [--change <change>]... [--defect <defect>]... [--idea <idea>]... [--dry-run]
```

Record one communication with a stakeholder, from what the analyst reports in chat, as a record beside the specification.

Validates its input first and prints the errors of an invalid
specification, then works on what could be read (ADR-065). Writes
one communication record, `records/communications/COM-<n>.yaml`
beside the specification's folder, under the next number after the
largest any communication record carries, from the options alone:
who said it and to whom (`--from`, `--to`, roles), when (`--at`, an
instant with its offset), how (`--channel`), what it is (`--type`),
the words (`--words`, with `--form` saying whether they are verbatim
or a summary), who wrote it down (`--recorded-by`), and what it
touches (`--question`, `--decision`, `--element`, `--change`,
`--defect`, `--idea`, each repeatable). The command guesses
nothing: every one of who, when, how, what and what was said that is
not given comes back as a line on standard output, one per missing
option, saying what to ask the analyst and the option to give it
with, and nothing is written, with exit 1; with `--json` the same
under `log.missing`, each with its option, the question to put and
the choices the specification offers (ADR-220).

Refuses, with exit 1 and nothing written, a role that is not a
stakeholder, an instant not written as RFC 3339 with its offset, a
channel, type, form or direction outside their values, a question
that is neither open nor answered by a decision, a decision,
element, change, defect, idea, session or source the specification
and its records do not have, an answer (`--type answer`) that names
no question, a reply on a record that is not a question, and an
evidence file that is not under `records/`. Each evidence file is
read and its SHA-256 and media type written into the record, the
media type from the file's extension or given after `=` as
`--evidence <path>=<media type>`; the validator later refuses a
record whose file no longer matches (`evidence`).

Prints on standard output the record's path, and for an answer the
`specarch decide` line that records it as the decision, citing the
record with `--record`. With `--dry-run` it prints the record it
would write and writes nothing.

## Arguments

- `<folder>` (required): The folder holding the specification; the record goes into records/communications/ beside it.

## Options

- `--direction <direction>` (one of `in`, `out`): in when the stakeholder said it to the project, out when the project said it to the stakeholder.
- `--from <from>`: Who said it, by role; a stakeholder of the specification, never a person.
- `--to <to>`: Who it was said to, by role.
- `--at <at>`: When it was said, an instant with its offset as RFC 3339 writes it, such as 2026-10-31T10:15:00+07:00; the date and the offset are asked for when the analyst gave a time alone.
- `--channel <channel>` (one of `meeting`, `chat`, `email`, `call`, `document`): How it was said: meeting, chat, email, call or document.
- `--type <type>` (one of `answer`, `question`, `request`, `information`, `objection`): What it is: answer (to a question of the specification), question (the stakeholder asked), request, information or objection.
- `--words <words>`: What was said, as given or summarised.
- `--form <form>` (one of `verbatim`, `summary`): verbatim when --words holds the words as they were said or written, summary when it is a summary of them.
- `--reply <reply>`: What was said back, for a question the stakeholder asked; in the same form as the words.
- `--person <person>`: A label for the person on the stakeholder's side when the role alone does not say who; left out of the documents for everyone.
- `--recorded-by <recorded-by>`: The role that writes it down, usually the analyst's.
- `--session <session>`: The session or test session record it was said in, by ID.
- `--source <source>`: The declared source it belongs to, by key: the interview it was part of.
- `--evidence <evidence>` (given once per value): A file under records/ that shows it, by its path, with the media type after = when the extension does not say it; repeatable.
- `--question <question>` (given once per value): A question of the specification it is about, open or answered, by ID; repeatable, and required for an answer.
- `--decision <decision>` (given once per value): A decision it is about, by ID; repeatable.
- `--element <element>` (given once per value): A requirement ID or a
- `--change <change>` (given once per value): A change request it is about, by ID; repeatable.
- `--defect <defect>` (given once per value): A defect it is about, by ID; repeatable.
- `--idea <idea>` (given once per value): An idea it is about, by ID; repeatable.
- `--dry-run`: Check everything and write nothing; print the record it would write as would write <path>.

## Reads

- `{folder}`: The specification, and the records beside it: the communication records for the next ID, the sessions and the records the links name.
- `records/<evidence path>`: Each evidence file, to take its SHA-256.

## Writes

- `records/communications/COM-<n>.yaml`: The communication record, under the next number; refused when a file of that name exists.

## Standard output

The errors of an invalid specification; then, when something is missing, one line per missing option saying what to ask; or the path written and, for an answer, the decide line to run next.

## Standard error

A usage message on a usage error, the reason a refusal, and one line naming the record written.

## Exit status

- 0: the record is written.
- 1: nothing written, since a required option is missing or a value names what the specification does not have; or written, while the specification has errors. Then: Read the lines on standard output: ask the analyst each question they put, and run the command again with the option each names; a refusal on standard error names the value to correct.
- 2: usage error, or a file that could not be read or written. Then: Read the usage message or the reason on standard error, correct the command line or the path it names, and run it again.
