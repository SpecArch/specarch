# specarch log test-run

```
specarch log test-run <folder> [--results <results>] [--environment <environment>] [--version <version>] [--build <build>] [--digest <digest>] [--at <at>] [--by <by>] [--open-defect <open-defect>]... [--severity <severity>] [--phase <phase>] [--dry-run]
```

Record one run of the tests, from a JUnit XML results file, as a test-run record beside the specification, each result mapped to its test.

Validates its input first and prints the errors of an invalid
specification, then works on what could be read (ADR-065). Reads
the JUnit XML file `--results` names, which must be under
`records/` beside the specification's folder (such as
`records/evidence/`), a `testsuites` or a `testsuite` element
holding `testcase` elements, and writes one test-run record,
`records/test-runs/TR-<n>.yaml`, under the next number after the
largest any test-run record carries: the environment
(`--environment`), the version (`--version`, a release record), the
build (`--build`) and the specification's digest (`--digest`) when
known, the instant (`--at`, with its offset), the role that ran it
(`--by`), the results file as evidence with its media type and
SHA-256, and one result per testcase in the file's order, with the
name as the file writes it, the outcome (passed, failed, error or
skipped, from the testcase's failure, error or skipped child) and
the failure's message. Nothing else is read from the file: its
timestamp has no offset, and its hostname is no environment of the
specification. Every one of the results file, the environment, the
instant and the role that is not given comes back as a line on
standard output saying what to ask, and nothing is written, with
exit 1; with `--json` under `log.missing` (ADR-220).

Each result names the test of the specification it maps to: the
testcase's name, cut at the first `/` (a subtest) and with a
trailing argument list such as `()` taken off, is the test's name,
or `Test` or `test` followed by the name in Pascal case (the
functions specarch-gen-tests-go and specarch-gen-tests-swift
write), or begins with the name and a colon (the display name of
the Swift and Dart tests). A testcase that runs the worked examples
of an algorithm or of a call mapping, which no test names, names
that element under `element` instead: its cut name is a function
the generators write for them (`TestAlgorithm` or `testAlgorithm`
and the algorithm's name in Pascal case; `TestMapping` and the
operation and the call in Pascal case, or `testMapping` with them
and `Sent` or `Kept`), or its name is or begins with their display
name (`algorithm <name>`, or `mapping of <operation>'s call to
<call>`) and a colon or a space; the element is
`#/algorithms/<name>`, or the path of the operation,
`#/paths/<path>`. A testcase that maps to no test and no element,
or to two of them, is kept in the record without either and
printed as a line `unmapped: <name>: <why>` on standard output,
under `log.unmapped` with `--json`; the validator warns
`test_unmapped` at it until its test or element is named
(ADR-230).

`--open-defect <test>`, repeatable, opens a defect for a test that
failed or errored in the run, with `--severity` and `--phase`
given: `records/defects/DEF-<n>.yaml`, status reported, reported by
the run's role on the date of the run's instant, violating the
requirements the test verifies, or the element the test is about
when it verifies none (the path of its operation, or the command,
page, job, process, route, gateway, workflow, flow, algorithm or
entity it names, the element the testruns document counts it for), naming the test, observed from the first
failure's message and expected from the test's then; for the
acceptance and production phases its environment is the run's, and
for acceptance its build is the run's version, digest and build.
Each failed result of that test links the defect. A failure no
`--open-defect` names opens nothing, and a worked example's
failure, which names no test, opens none.

Refuses, with exit 1 and nothing written, a results file that is
not under `records/`, cannot be read, is not JUnit XML, holds no
testcase or holds a testcase without a name; an environment, a version or a role the specification
and its records do not have; a digest not written as sha256: and 64
hexadecimal digits; an instant not written as RFC 3339 with its
offset; an `--open-defect` for a test that is not one of the
specification's or that did not fail in the run, one without
`--severity` or `--phase`, a severity or phase outside their values,
an acceptance defect without `--version`, and a test that verifies
nothing and is about no element. Prints on standard output the
unmapped lines, then the record's path and the path of each defect
opened; with `--json` the record under `log.record` and `log.id`,
the unmapped testcases under `log.unmapped` and the defects under
`log.defects`. With `--dry-run` it prints the records it would write and
writes nothing.

## Arguments

- `<folder>` (required): The folder holding the specification; the record goes into records/test-runs/ beside it.

## Options

- `--results <results>`: The JUnit XML file of the run, by its path under records/, such as records/evidence/TR-3-junit.xml; it becomes the record's evidence.
- `--environment <environment>`: The environment of the specification the tests ran in.
- `--version <version>`: The version of the build, a release record, planned or released, when the build is one.
- `--build <build>`: The build that was tested, such as a commit or an image tag with its digest.
- `--digest <digest>`: The digest of the specification the build was generated from, as sha256: and 64 hexadecimal digits, as the approval record keeps it.
- `--at <at>`: When the run started, an instant with its offset as RFC 3339 writes it, such as 2026-11-03T14:05:00+07:00.
- `--by <by>`: The role that ran the tests, a stakeholder of the specification.
- `--open-defect <open-defect>` (given once per value): A test that failed or errored in the run, by name, to open a defect for; repeatable.
- `--severity <severity>` (one of `critical`, `major`, `minor`, `cosmetic`): How badly the defects opened stop the system from meeting its requirements: critical, major, minor or cosmetic.
- `--phase <phase>` (one of `development`, `acceptance`, `production`): Where the defects opened were found: development, acceptance or production.
- `--dry-run`: Check everything and write nothing; print each record it would write.

## Reads

- `{folder}`: The specification and its tests, and the records beside it: the test-run and defect records for the next IDs, and the releases.
- `records/<results path>`: The JUnit XML file, for its testcases and its SHA-256.

## Writes

- `records/test-runs/TR-<n>.yaml`: The test-run record, under the next number; refused when a file of that name exists.
- `records/defects/DEF-<n>.yaml`: One defect for each --open-defect, under the next numbers; refused when a file of that name exists.

## Standard output

The errors of an invalid specification; then, when something is missing, one line per missing option saying what to ask; or one unmapped line per testcase that maps to no test, the path of the record written and the path of each defect opened.

## Standard error

A usage message on a usage error, the reason a refusal, and one line naming the record written with its counts.

## Exit status

- 0: the record is written; a testcase that maps to no test is printed and kept, and does not change the status.
- 1: nothing written, since a required option is missing or a value names what the specification and its records do not have, or the results file is refused; or written, while the specification has errors. Then: Read the lines on standard output and the reason on standard error: give each missing option, move the results file under records/, or correct the value named, and run it again.
- 2: usage error, or a file that could not be read or written. Then: Read the usage message or the reason on standard error, correct the command line or the path it names, and run it again.
