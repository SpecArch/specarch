# specarch skill

```
specarch skill --out <out> [--check] [--dry-run]
```

Write an agent skill for SpecArch, in the Agent Skills format, from the program's own command definitions.

Writes the folder `{out}/specarch` in the Agent Skills format
(agentskills.io), which agents read from their skills folder:
`SKILL.md`, whose front matter names the skill and says when to use
it and whose body is the agent guide (`specarch guide --agent` up to
the plug-ins found) with a list of the references; and
`references/<command>.md`, one file per command, with its synopsis,
what it does, its arguments and options, what it reads, writes and
prints, and each exit status with its meaning and remedy. Everything
is made from the command definitions and the facts built into the
program, so the skill names exactly the commands and options this
release takes. The plug-ins found on PATH are left out, so the same
release writes the same skill on every machine; the skill says that
`specarch capabilities --json` lists them (ADR-214).

Put it where the agent reads skills, such as `.agents/skills` in a
repository or the skills folder the agent's own documentation names,
and write it again after an upgrade. A reference in `{out}/specarch/references` the skill does
not hold, a `.md` file, is removed, so a command a later release
drops leaves no reference behind; no other file is removed. A
folder `{out}/specarch` that holds files but no `SKILL.md` naming
the skill specarch is not one this command wrote, and is refused
with nothing written. With `--check` nothing is written: the skill
is made in memory and compared with what is on disk.

## Options

- `--out <out>` (required): The folder the skill's folder, specarch, is written in, such as .agents/skills.
- `--check`: Make the skill in memory and fail when the files on disk differ. Writes nothing.
- `--dry-run`: Write nothing; print would write <path> or would remove <path> for each file that would change. Not with --check.

## Reads

- `{out}/specarch`: The skill on disk, to write only the files that change, to remove the files it no longer holds, and with --check to compare.

## Writes

- `{out}/specarch/SKILL.md`: The skill: its name and when to use it, then the agent guide and the list of references. Nothing is written with --check.
- `{out}/specarch/references/<command>.md`: One command's reference, a command of two words joined by a hyphen. Nothing is written with --check.

## Standard output

The paths written, or with --dry-run would write <path> and would remove <path>, or with --check one line per file that differs, is missing or is not the skill's.

## Standard error

A usage message on a usage error, the reason a file could not be read or written, and a one-line count at the end.

## Exit status

- 0: written, or with --check the skill on disk is current.
- 1: with --check, a file differs, is missing or is not the skill's. Then: Run specarch skill --out with the same folder and without --check to write the current skill, then run the check again.
- 2: usage error, a folder specarch under --out that this command did not write, or a file that could not be read or written. Then: Read the usage message or the reason on standard error, give --out a folder whose specarch folder is the skill's or does not exist, correct the path it names, and run it again.
