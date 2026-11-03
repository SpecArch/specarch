# specarch run

```
specarch run <package> [--results <results>] [--base-url <base-url>] [--server <server>] [--callers <callers>] [--setup <setup>] [--dry-run]
```

Run a test package against its environment and write the results as JUnit XML.

Reads a test package, the folder `specarch generate tests --package`
wrote or its file `specarch-test-package.yaml`, and refuses with
exit 2 a file that is not one, or one of another format version;
it reads nothing of the specification, so the package runs where
the specification is not (ADR-233).

The requests go to `--base-url`, or to the server of the package
that `--server` names, or to the package's one server; with several
and neither option it refuses with exit 2, naming each server with
its deployment and address, and so does a `--server` that names
none. Each case runs in the package's order:

- A case the package marks skipped (no request the specification
  gives, a command, a page, a handshake, a streamed file, a caller
  with properties of its own, or an outcome only the state shows)
  is reported skipped with the package's reason.
- A case whose caller is a role signs in with the headers the file
  `--callers` gives that role, a YAML mapping of role to header
  name and value, such as `librarian: { Authorization: Bearer
  <token> }`; a role the file does not give, or no file, leaves the
  case skipped, naming the role and the option. The headers are
  never written anywhere.
- A case that needs records runs the program `--setup` names first,
  with the case's name as its one argument and on its standard
  input a JSON object with the case, the caller and the records as
  entity name to a list of records; exit 0 means the records are in
  place. A program that exits otherwise errors the case, with its
  exit status and standard error. Without `--setup` such a case is
  skipped, naming the records and the option.
- The request is the case's method and path, with its headers and,
  when it has one, its JSON body and `Content-Type:
  application/json`; `Accept: application/json` always. No answer
  errors the case with what the client said. An answer whose status
  is not the one expected fails the case, naming both and the start
  of the body; a body that lacks a field expected, or holds another
  value, fails it, naming the field and both values. Numbers compare
  by value, and a field the case marks numeric compares by value
  when both sides are text. Otherwise the case passes; the state
  after and the messages emitted, which a request from outside does
  not see, are named in its output as not checked.

Writes the results to `--results` as JUnit XML: one `testsuite`
named after the specification, its version and the environment,
with the environment, the server, the version and the digest as
properties, and one `testcase` per case named as the case is, so
`specarch log test-run` maps each back to its test or, for a
derived case, to its subject's element. A failure, an error or a
skip carries its message, and a case's output its request and the
answer. The file holds no time, timestamp or host name, which
Ant's JUnit schema asks for: the run's instant is `log test-run
--at`, with its offset, the host is no environment of the
specification, and the duration of each case is in its progress
line, so the file changes only with the outcomes and the answers. Prints one
line per case on standard error as it ends, step N of M, with its
outcome, its duration and its reason; one line per failed or
errored case on standard output; then the tests the package holds
back and the counts, with how to record the run. With `--dry-run`
it makes no request, runs no setup program and writes nothing: it
prints, for each case, the request it would make with the role and
the setup it needs, or that it would skip it and why, then the
results file it would write, and exits 0.

## Arguments

- `<package>` (required): The test package, its folder or its file specarch-test-package.yaml.

## Options

- `--results <results>`: The file the JUnit XML results are written to; a folder it names is made.
- `--base-url <base-url>`: The address the requests go to, such as https://staging.library.example, in place of the package's servers.
- `--server <server>`: The server of the package the requests go to, by its name in the deployment.
- `--callers <callers>`: A YAML file of the headers that sign each role in, such as librarian: { Authorization: Bearer <token> }; kept out of the package and the results.
- `--setup <setup>`: A program that puts a case's records in place, run before each case that needs records with the case's name as its argument and the records as JSON on its standard input.
- `--dry-run`: Make no request, run no setup program and write nothing; print what the run would do with each case, would request <method> <address> or would skip and why, and the results file it would write.

## Reads

- `{package}`: The test package.
- `{callers}`: The headers that sign each role in, with --callers.

## Writes

- `{results}`: The results, as JUnit XML.

## Standard output

One line per case that failed or errored, with the case's name and why; with --dry-run, one line per case saying what the run would do, and the results file it would write.

## Standard error

A usage message on a usage error, the reason a refusal, one progress line per case, the tests the package holds back, and the counts.

## Exit status

- 0: the results are written and no case failed or errored; a skipped case does not change the status.
- 1: the results are written and a case failed or errored. Then: Read the failed and errored lines, and each case's output in the results file: settle each failure with the people who own the system or its specification, and record the run with specarch log test-run.
- 2: usage error, a package, callers file or setup program that could not be read or found, no server chosen, or a results file that could not be written. Then: Read the usage message or the reason on standard error, correct the command line, the server or the path it names, and run it again.
