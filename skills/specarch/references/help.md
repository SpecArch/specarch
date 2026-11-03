# specarch help

```
specarch help [<topic>...]
```

Explain an area of SpecArch or one command, from the text built into the program.

Without a topic it prints the usage and the list of topics. A topic
is an area, with a short guide of its own (format, extract,
validate, generate, upgrade, gaps, asking, start, plugins,
conventions, examples, agents, situations (one entry per situation:
when it happens and what to run)),
or a command's name, `idioms-diff` or `idioms diff` for the
subcommand. A command's topic prints the command's reference made
from its definition in this specification: the synopsis, the
summary, each argument and option, what it prints, what it writes,
and each exit status with what to do about it; after the area's
guide when the command has one. The plugins topic also lists the
plug-ins found on PATH now and what each says it writes and does
not write.

The text is built into the program from docs/help and from
spec/design/commands, so an installed program explains exactly the
commands it was built with, with no source checkout or docs folder.
It is plain text, wrapped for an 80-column terminal, with no colour
or terminal codes. `-h` and `--help` given as the command are help
with no topic.

## Arguments

- `<topic>` (one or more): An area or a command; the list is printed by help with no topic.

## Reads

- `PATH`: The folders searched for specarch-gen-* plug-ins, for the plugins topic only; each is run with --capabilities.

## Standard output

The usage and the topics, or the topic's text.

## Standard error

A topic that does not exist, with the topics there are.

## Exit status

- 0: printed.
- 2: no such topic. Then: Run specarch help for the list of topics and give one of them.
