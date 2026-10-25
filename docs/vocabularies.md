# Vocabularies

A vocabulary is the words of one language that `specarch extract
documents` finds a commitment and reads a table of fields by, and that
`specarch merge` reads a number of days in a requirement by (ADR-105).
SpecArch ships English and reads it when nothing else is named. A
project that writes its documents in another language writes a
vocabulary for it, keeps it in its specification repository, and names
it with `--vocabulary`:

    specarch extract documents --vocabulary de.specarch-vocabulary.yaml --out trees/handbook docs/handbuch.md
    specarch merge --vocabulary de.specarch-vocabulary.yaml --out spec trees/code trees/handbook

`--language <tag>` names a language SpecArch ships a vocabulary for
instead. Either way the reader prints a line naming the vocabulary and
writes `language` on the source, so a reader of the tree knows which
words were applied. merge reads each requirement by the vocabulary of
the language of the source it cites first; a source with no `language`
is English.

## The file

The file is YAML, named `<tag>.specarch-vocabulary.yaml`. Every key is
written; a list may be empty where the language has no such word. Words
are matched in any case and only as whole words, so a word inside a
longer one is not found.

| Key | Holds | Read for |
|---|---|---|
| `specarchVocabulary` | `"1.0.0"` | the format's version; a vocabulary on `"0.1"`, the same in shape, is read too |
| `language` | a BCP 47 tag, such as `de` or `pt-BR` | the source's `language` |
| `description` | one sentence | the reader of the file |
| `commitments` | `must` and `should`, each a list | a sentence with one of them is a commitment of that priority |
| `frequencies` | words such as once and twice | a commitment |
| `timeLimits` | words and phrases such as within and no later than | a commitment |
| `pronouns` | words such as it and they | a commitment that starts with one names no subject, and is a question |
| `linkingVerbs` | words such as is and are | merge: the subject is the words before one of these or a commitment word |
| `leadingWords` | words such as the and each | merge: left out of the subject |
| `numbers` | each number word and its whole value | a commitment; merge's number of days |
| `units` | each unit word and the ISO 8601 duration it stands for | merge: a unit of whole days, such as P1D or P7D, gives a number of days |
| `timeSeparators` | the characters between hour and minute | a time of day, such as 9:00 or 9.00, is a commitment |
| `headers` | `field`, `type`, `required`, `sensitivity`, `description` and `rowNumber`, each a list | a table whose first column is a field word and whose others are the other four is a table of fields; a leading row-number column is skipped in every table |
| `yes` | the words of a Required column that mean required | the field is required |
| `sensitivities` | `public`, `internal`, `personal` and `credential`, each a list | a Sensitivity column; a sentence that names a field and holds a personal or credential word gives its sensitivity |
| `types` | `string`, `date`, `dateTime`, `boolean` and `email`, each a list | a Type column; a type with no word is left for a question |

The file is checked when it is read, and refused with its line when a key
is missing or unknown, a number is not a whole number, a unit is not an
ISO 8601 duration, the tag is not a language tag, a word is twice in one
list, or a word is in two lists of one group: two priorities, two
headers, two sensitivities or two types. A word may be in two groups:
English's `required` is both a header and a yes word.

## English

The vocabulary SpecArch ships,
[`vocabularies/en.specarch-vocabulary.yaml`](../vocabularies/en.specarch-vocabulary.yaml),
is the example to start from: copy it, change `language`, and replace
the words. Which languages SpecArch ships beyond English is decided when
a project first supplies one.
