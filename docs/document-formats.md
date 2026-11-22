# Document formats: Markdown, Word and PDF

Every document `specarch document` writes as Markdown can also be
written as Word (`.docx`) or PDF (`.pdf`), and the four books a project
files and a customer signs are written as PDF or Word (ADR-248, ADR-249).
The Word and PDF forms are made from the Markdown, so they never say
anything the Markdown does not, and a specification with gaps still gets
every format, with every gap shown where it is.

```sh
specarch document requirements --out docs spec                 # requirements.md
specarch document requirements --format docx --out docs spec   # requirements.docx
specarch document requirements --format md,pdf --out docs spec # .md and .pdf
specarch document requirements --format all --out docs spec    # .md, .docx and .pdf
specarch document books --out docs/books spec                  # the four books as PDF
specarch document books --format docx --out docs/books spec    # the four books as Word
specarch document requirements --format all --check --out docs spec
```

## Choosing the formats

| Target | Formats | Without `--format` |
|---|---|---|
| every Markdown document (`intent`, `techspec`, `requirements`, `testplan`, `traceability`, `brd`, ...) | `md`, `docx`, `pdf` | `md` |
| `books` | `pdf`, `docx` | `pdf` |
| `html`, `problems` | their own files only | `index.html`; `problems.txt` and `problems.sarif` |

`--format` takes one format, several separated by commas, or `all`, every
format the target writes. They are written in the order md, docx, pdf,
each as `<target>.<format>` in the target's folder. `--format` for
`html` or `problems`, or `md` for `books`, is a usage error (exit 2).

Without `--format`, the formats come from the target's entry under
`targets` in the implementation file, beside its output folder:

```yaml
targets:
  requirements:
    output: ../../docs
    formats: [md, pdf]
  books:
    output: ../../docs/books
    formats: [pdf, docx]
```

Without `--out` the folder comes from `output` there, as for every
document; the implementation files that name a folder or formats for one
target must agree. Without `formats`, a document is written as Markdown
and the books as PDF. A format the target does not write, or `formats` on
`html`, `problems` or a target that is not a document's, is an error at
that entry (`document_format`); the file is still read, and the formats
of the list the target has are written. `specarch approve` compares every
file of the formats the entry lists.

## What each format holds

The Markdown is the source form. The Word and PDF forms of a document are
a book of that document alone, rendered from its Markdown:

1. **Front matter**: the document's title, the specification's title,
   version, date and status, its owners, the root file and meta-model it
   was made from, what the book contains, and the change history, one row
   per release record, newest first.
2. **Contents**: every heading to the third level, each a link, and last
   the Problems and gaps list.
3. **The document**, starting on a new page.
4. **Problems and gaps**: every mark of the book, grouped by document (the
   front matter's under Front matter), each linking to its mark, and each
   mark's label linking back.

The three carry the same text, ids, links, tables and marks: the Word and
the PDF writer read one model of the Markdown (`internal/mdbook`), so
their anchors, their id links and the marks they list are the same.

| | md | pdf | docx |
|---|---|---|---|
| Front matter, contents, Problems and gaps list | no: the marks are at their entries, and `document problems` lists them | yes | yes |
| Page numbers in the contents | no pages | yes | added by Word when the reader updates the field (Update Field) |
| Problems and gaps list | | each entry gives its page | each number links to its mark |
| Headings | Markdown headings | tagged H1 to H6, bookmarks | Word's heading 1 to heading 6, so the navigation pane finds them |
| Fonts | | the Go fonts, embedded as subsets | Calibri, and Courier New for code, named, not embedded |
| Mermaid diagrams | the source, drawn by a Markdown viewer | the source, labelled | the source, labelled |
| Same bytes for the same input | yes | yes | yes |

The `html` document draws the entity diagram and the workflows.

## The four books

`specarch document books` writes four books, each made from the Markdown
the other document targets write for the same specification:

| File | Book | Made from |
|---|---|---|
| `business-requirements` | business requirements document | `brd` |
| `requirements-specification` | requirements specification | `requirements`, then `traceability` |
| `technical-specification` | technical specification | `techspec`, then `deployment` and `operations` |
| `test-document` | test document | `testplan`, then `traceability` |

Each is `.pdf`, or `.docx` with `--format docx`. The business
requirements document (`specarch document brd` writes it as `brd.md`
too) follows the business requirements specification of ISO/IEC/IEEE
29148: the purpose, the intent (problem, goals, out of scope, quality
goals), the stakeholders, the business needs with the goals each serves,
each requirement at business level (statement, priority, status, the
needs it refines and the goals those serve), the constraints, the
assumptions and the glossary. How a requirement is designed and verified
is left to the requirements specification. The test document's coverage
matrix is the traceability matrix: each requirement with what verifies
it, and the gaps, a requirement no test verifies among them.

The books have names of their own, so a folder holds `brd.pdf` (the brd
document) and `business-requirements.pdf` (the book) side by side.

## Dates and the same bytes

The date in the front matter is the approval's when the version is
approved (`specarch approve`), else the date of the release record of
that version (`records/releases/`). With neither, the front matter says
the version is not dated and marks that as a gap. Nothing is taken from
the clock: a docx's parts are written in a fixed order, each dated
1980-01-01, with no revision or random ids, and a pdf holds no creation
date. So the same specification writes the same bytes in every format,
and `--check` and `approve` compare each file byte for byte. The files
are compressed with the Go standard library's deflate, whose output can
change with a Go release; `--check` then reports the files once.

## Gaps and problems

A book marks what the Markdown documents mark, in a shaded box with a bar
on its left at the entry it concerns:

- **Problem**: an error or warning of the validator, a requirement no test
  verifies among them;
- **Open question** and **Draft**: an open question and the documents it
  holds up;
- **Open change request**: a change request open on the element;
- **Stakeholders unconfirmed**: the stakeholder list is not confirmed
  complete;
- **Gap**: what the front matter cannot fill, such as the date.

An invalid specification gets every format and every book, and the run
exits 1.

## Links

- Each heading is a bookmark and a link target.
- A link between the documents of one book goes to the heading it names.
- An element's id, a key of the specification that holds a digit or a
  hyphen such as `REQ-12` or `NEED-3`, links to the heading that is that
  id, or else the first table row it starts. In a docx its bookmark is
  named after it, such as `id_REQ_12`.
- A link to a web address is a link; a link to a file or heading outside
  the book is printed with its target in brackets.

## Print and accessibility

- A4 pages with 20 mm margins; the header names the book and the
  specification's version, the footer the page and the number of pages.
- A table that crosses a page repeats its header row.
- The pdf is tagged: the structure tree holds the headings, paragraphs,
  lists, tables, links and the table of contents in reading order, the
  file names its language, and the header and footer are artifacts. Its
  text is real text with a Unicode map; a character none of its fonts has
  is written as its code point, such as `[U+2318]`.
- The docx names its language in its styles and core properties, and a
  character XML cannot hold is written as its code point the same way.

There is one layout per format, the one above: no theme or template
option.
