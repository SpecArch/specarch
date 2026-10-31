# specarch facts

```
specarch facts <language> <folder> <dump> [--dry-run]
```

Write the code-facts dump extract reads of a committed folder.

Reads a committed folder of code with the reader of its language and
writes the code-facts dump that specarch extract of that language
reads (docs/reading-code.md, ADR-116): the version of the format,
the reader's name and version, the folder's path from its
repository's root, the commit that last changed it, every tracked
file the reader reads with the number of its syntax errors, and
every fact the reader gives. It is what tools/code-facts/dump-*.sh
run in a checkout, so a team with the installed binary and no
checkout makes the same dump.

The folder is read in its git repository, run without the user's or
the system's configuration; a folder with changes not committed or
files neither tracked nor ignored is refused, naming them, and so is
one no commit has changed, since no commit would name what the facts
were read from. A dump file in another repository than the code's,
one holding the code's as a submodule, names that submodule's folder
as its repository, so extract reads the code at the commit the
gitlink pins; a dump file in a repository that does not hold the
code's is refused.

javascript reads the tracked JavaScript, TypeScript and Vue files
and the tsconfig.json and jsconfig.json files under the folder, not
the .d.ts files, with the TypeScript compiler inside this binary
(third_party/typescript-go) and its Vue reader, and reads no file
outside them: no node_modules, no Node. swift and dart read the
tracked .swift or .dart files with the reader of readers/swift or
readers/dart, which this binary holds with the lock file that pins
every dependency: it is written once into the user's cache
(`$XDG_CACHE_HOME/specarch/readers/<language>-<hash>`, else the
operating system's cache folder), named by a hash of its files, and
built there with the language's toolchain (swift build, or dart pub
get --enforce-lockfile), whose progress is printed on standard
error. Nothing is written into the folder read.

## Arguments

- `<language>` (required; one of `javascript`, `swift`, `dart`): The reader to run.
- `<folder>` (required): The committed folder to read.
- `<dump>` (required): The dump file to write, in an existing folder.

## Options

- `--dry-run`: Read the folder and write nothing; print the dump it would write, and for the Swift or the Dart reader the cache folder it would write the reader into when it is not there yet, as would write <path>, without building or running that reader.

## Reads

- `{folder}`: The tracked files of the folder's language, at the commit that last changed it.

## Writes

- `{dump}`: The code-facts dump, whose header names the reader's version; specarch extract refuses a dump another version made.

## Standard output

One line naming the dump written, the folder, the commit and the repository, and the submodule read through when the dump file is in another repository.

## Standard error

A usage message on a usage error; the reason a folder or a dump file was refused, with git's own message; and for swift and dart, each step of writing and building the reader in the cache, with the toolchain's output.

## Exit status

- 0: the dump was written.
- 1: the folder could not be read as committed code: it is in no git repository, it has changes not committed or files not tracked, no commit has changed it, the dump file is in a repository that does not hold the code's, or the reader's toolchain failed to build or to run it. Then: Commit the folder's changes and track its files, or install the toolchain of the reader named on standard error, then run it again.
- 2: usage error, a language this build does not read, a folder that does not exist, a dump file whose folder does not exist or that could not be written, or a submodule folder with a quote or a backslash. Then: Read the usage message or the reason on standard error, correct the command line or the path it names, and run it again.
