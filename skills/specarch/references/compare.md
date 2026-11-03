# specarch compare

```
specarch compare <folder> [--test-run <test-run>]... [--results <results>]... [--scan <scan>]... [--measurements <measurements>]... [--out <out>] [--dry-run] <implementations>...
```

Compare alternative implementation files of one specification in one table, from their test results, scan findings and measurements.

Validates its input first and prints the errors of an invalid
specification, then works on what could be read (ADR-065). Reads
the implementation files given, each one of the specification's own
(`implementation/<stack>/<name>.<stack>.specarch-implementation.yaml`)
and named in the table and in the options by its name, the file's
name without `.specarch-implementation.yaml`, such as `lending.go`;
the columns follow the order they are given in. For each it reads
what the options give it, every option taking `<name>=<value>` and
given once per implementation at most (ADR-232):

- `--test-run <name>=TR-<n>`: a test-run record of
  `records/test-runs/`, whose results name their tests already; or
- `--results <name>=<path>`: a JUnit XML file under `records/`,
  each testcase mapped to its test as `specarch log test-run` maps
  it. Not both for one implementation.
- `--scan <name>=<path>`: the scan of its software bill of
  materials, under `records/`: grype's JSON (`matches`) or
  osv-scanner's (`results`). A grype finding keeps grype's
  severity; an osv-scanner group of aliases is one finding, its
  severity read from its highest CVSS score by the CVSS v3.1
  qualitative scale (9.0 and above critical, 7.0 high, 4.0 medium,
  above 0 low), unknown without one.
- `--measurements <name>=<path>`: a JSON object in a file under
  `records/` (JSON, since every YAML file there is read as a
  record), with what only a person or a CI job measures, every key optional
  and no other key read: `buildSeconds`, how long the build took,
  `linesOfCode` and `deploySteps`, the steps a person takes to
  deploy it, each a whole number from 0; and `requirements`, keyed
  by requirement ID, each with `latency` or `throughput` or both,
  each `{value, unit, how}`, the value a number, the unit and how
  it was measured in words (`how` optional).

Writes one table, the summary first: per implementation the
derived cases passed of all the case rows, the results by
outcome, the scan findings by severity, the libraries, the build
time, the lines of code and the deploy steps. Then the derived
cases, one row per case that is chosen or that a test covers, with
the tests that cover it, each cell failed when any result of those
tests failed or errored, else passed when one passed, else skipped
when one was skipped; the requirements a measurements file names,
one row per requirement and quantity, with the requirement's
statement beside the values so the reader judges each against it,
since a requirement states its target in words; the libraries of
the implementation files' `libraries`, one row each, with the
version, licence and scope each file gives, or not used; the scan
findings, one row per finding and package, with the severity and
the version in each scan, or not found; the inputs, each with its
SHA-256; and the gaps.

A result that is missing is a gap, never a stop: no test-run and no
results for an implementation, a case no test covers (no test), a
case whose tests have no result in the run (not run), no scan, no
measurements file, or one that leaves out a key or a quantity
another gives. Each is written `gap` in its cell, with the reason,
and listed under Gaps with the option that fills it.

Without `--out` it prints the table as Markdown on standard
output. With `--out <folder>` it writes `comparison.md` and
`comparison.json` there, prints their paths, and prints the entry
to declare under `sources` in `specarch.yaml` for a decision to
cite the comparison: kind document, its title, the SHA-256 of
`comparison.json` as its edition, and the path of `comparison.md`
from the specification's folder as its url. With `--json` the
comparison is under `compare`, in the form `comparison.json`
holds (`specarchComparison` 1.0.0).

Refuses, with exit 1 and nothing written, an implementation file
that is not one of the specification's, one given twice, an option
whose name is no implementation given or that names one twice,
`--test-run` and `--results` for one implementation, a test-run
record the records do not hold, a file that is not under
`records/`, cannot be read or is not of its format, a measurements
file with a key it does not read or given twice, a value that is
not a number, a number that is not a whole number from 0 where one
is asked, or a requirement the
specification does not have. With `--dry-run` it lists the files
it would write and writes nothing.

## Arguments

- `<folder>` (required): The folder holding the specification; the inputs are under records/ beside it.
- `<implementations>` (required; one or more): The implementation files to compare, each one of the specification's, in the order of the table's columns.

## Options

- `--test-run <test-run>` (given once per value): An implementation's test-run record, as <name>=TR-<n>, such as lending.go=TR-3; repeatable, once per implementation.
- `--results <results>` (given once per value): An implementation's JUnit XML results file under records/, as <name>=<path>, such as lending.go=records/evidence/go-junit.xml; repeatable, once per implementation.
- `--scan <scan>` (given once per value): An implementation's scan of its software bill of materials under records/, grype's or osv-scanner's JSON, as <name>=<path>; repeatable, once per implementation.
- `--measurements <measurements>` (given once per value): An implementation's measurements file under records/, as <name>=<path>: build time, lines of code, deploy steps and the latency and throughput per requirement; repeatable, once per implementation.
- `--out <out>`: The folder to write comparison.md and comparison.json into; left out, the Markdown goes to standard output.
- `--dry-run`: Check everything and write nothing; list the files it would write.

## Reads

- `{folder}`: The specification, its tests and implementation files, and the test-run records beside it.
- `{implementations}`: The implementation files, for their names and libraries.
- `records/<input path>`: The results, scan and measurements files the options name, for their content and their SHA-256.

## Writes

- `{out}/comparison.md`: The table as Markdown, with --out.
- `{out}/comparison.json`: The table as JSON, with --out.

## Standard output

The errors of an invalid specification; then the table as Markdown, or with --out the paths of the two files written and the sources entry to cite the comparison.

## Standard error

A usage message on a usage error, the reason a refusal, and one line counting the implementations, rows and gaps.

## Exit status

- 0: the table is written or printed, its gaps marked in their cells and listed.
- 1: nothing written, since an implementation file, an option's name or a test-run record is not the specification's, or an input file is refused; or written, while the specification has errors. Then: Read the reason on standard error: name the specification's own implementation files, give each option once per implementation by its name, move the input under records/ or correct its format, and run it again.
- 2: usage error, or a folder or file that could not be read or written. Then: Read the usage message or the reason on standard error, correct the command line or the path it names, and run it again.
