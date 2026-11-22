# PDF documents

`specarch document pdf` writes four PDF books from a specification:
the documents a project files and a customer signs. Each book is made
from the Markdown the other document targets write for the same
specification, so a book never says anything the specification does not,
and a specification with gaps still gets all four books, with every gap
shown where it is (ADR-248).

```sh
specarch document pdf --out docs/books spec
specarch document pdf --check --out docs/books spec
specarch document pdf --language id --out docs/books-id spec
```

Without `--out`, the books go into the folder an implementation file
names for the `pdf` target:

```yaml
targets:
  pdf:
    output: ../../docs/books
```

## The four books

| File | Book | Made from |
|---|---|---|
| `brd.pdf` | business requirements document | `brd` |
| `requirements.pdf` | requirements specification | `requirements`, then `traceability` |
| `technical.pdf` | technical specification | `techspec`, then `deployment` and `operations` |
| `qa.pdf` | test document | `testplan`, then `traceability` |

The business requirements document (`specarch document brd` writes it as
`brd.md` too) follows the business requirements specification of
ISO/IEC/IEEE 29148: the purpose, the intent (problem, goals, out of
scope, quality goals), the stakeholders, the business needs with the goals
each serves, each requirement at business level (statement, priority,
status, the needs it refines and the goals those serve), the constraints,
the assumptions and the glossary. How a requirement is designed and
verified is left to the requirements specification.

The test document's coverage matrix is the traceability matrix: each
requirement with what verifies it, and the gaps, a requirement no test
verifies among them.

## What every book holds

1. **Front matter**: the book's title, the specification's title,
   version, date and status, its owners, the root file and meta-model it
   was made from, the documents the book contains, and the change history,
   one row per release record, newest first.
2. **Contents**: every heading to the third level, with its page, each a
   link.
3. **The documents**, each starting on a new page.
4. **Problems and gaps**: every mark of the book, grouped by document,
   each with the page of its mark.

The date is the approval's when the version is approved
(`specarch approve`), else the date of the release record of that version
(`records/releases/`). With neither, the front matter says the version is
not dated and marks that as a gap. Nothing is taken from the clock, so the
same specification writes the same bytes and `--check` compares a book
byte for byte.

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

Every mark is listed in the Problems and gaps list. The label of a mark
links to its entry in the list, and the entry's page links back to the
mark. An invalid specification gets its four books, and the run exits 1.

## Links

- Each heading is a bookmark (down to the fourth level) and a link target.
- A link between the documents of one book goes to the heading it names.
- An element's id, a key of the specification that holds a digit or a
  hyphen such as `REQ-12` or `NEED-3`, links to the heading that is that
  id, or else the first table row it starts.
- A link to a file or heading outside the book is printed with its target
  in brackets.

## Print and accessibility

- A4 pages with 20 mm margins; the header names the book and the footer
  the page.
- Real text in the Go fonts, embedded as subsets, with a Unicode map, so
  the text can be searched and copied. A character none of the fonts has
  is written as its code point, such as `[U+2318]`.
- Tagged: the structure tree holds the headings, paragraphs, lists,
  tables, links and the table of contents in reading order, and the file
  names its language; the header and footer are artifacts.
- A table that crosses a page repeats its header row; a word wider than
  its column is broken.
- A Mermaid diagram is printed as its source in a labelled box: the html
  document draws the entity diagram and the workflows.

There is one layout, the one above: no theme or template option.
