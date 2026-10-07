# Principles

SpecArch follows one principle, called the Low IQ Tax: every file, key,
message and document should cost its reader as little thinking as possible.
Brain power is finite, and the reader is often tired, new, or the author six
months later. So SpecArch keeps what changes often apart from what rarely
changes, says everything plainly and explicitly, and refuses what it cannot
interpret instead of guessing. The core of anything in SpecArch should be
understood in about five seconds.

## The rules

1. Write for the tired reader. Simplicity over magic. If the core of a
   file, a key or a message cannot be grasped in about five seconds, it is
   too clever.
2. Keep the changing apart from the fixed. What changes often (definitions,
   configuration, endpoints, values) lives apart from the logic that rarely
   changes, so changing what a system does never means wading through how it
   does it. Put the needles in the needle box and the hay in the barn.
3. Clarity first, then brevity. Some repetition is fine when it makes a
   thing clearer.
4. No magic and no cleverness: no hidden control flow, no side effects that
   are not written down, no surprising defaults, no shorthand names. Full,
   explicit names. One way to say one thing. Follow the patterns already in
   use.
5. No needless indirection: no empty folders, files or layers, and no
   abstraction added for later.
6. No placeholders. No data means no line. Nothing half-defined is accepted
   as defined.
7. When not completely sure, stop and ask. For a tool this means ambiguity
   is an error.
8. Few modes. A small number of clear options beats negotiation and
   configurability.
9. Documents give the summary first and go from the big picture to the
   detail, in plain words, without decorative symbols.
10. Be concrete where the machine is. A design so abstract that it ignores
    register widths, languages and runtimes is only theory. Saying "number"
    leaves every implementation to guess; saying "int64, carried as a string
    in JSON" lets each one get it right.
11. Before adopting a standard, or deviating from one, ask why it is the
    way it is and what led its authors there. Write that reason next to the
    choice. Never relax a standard's rule for convenience without saying
    what the rule protects against and why that does not apply here.

## How SpecArch applies them

### The two kinds of file

They are the needle box and the barn. The specification
says what the system is and does. An implementation file says how one stack
builds it. Generators and runtimes hold the logic. A change to the design
never touches a generator, and a library upgrade never touches the design.

### The meta-model

The meta-model uses full English words for its own keys
(`specarchImplementation`, `standardOutput`, `repeatable`, `exitCodes`).
Keywords borrowed from JSON Schema, OpenAPI and AsyncAPI keep their standard
spelling, because a reader who knows the standard already knows them. Every
object is closed: an unknown key is an error, not something ignored. The
smallest valid specification is a root file of two keys that reads top to
bottom.

### Types

Every value has a concrete type that says how wide it is: int32, int64,
uint64, double, decimal with precision and scale, string, bool, bytes, date,
timestamp, duration. The reason is practical. In JavaScript every number is
a 64-bit float: an int64 above 2^53 loses its last digits without an error,
and nothing in the language stops `1` and `"1"` from being mixed up. A design
that says only "number" cannot prevent that; a design that says "int64,
carried as a string in JSON" does. So the design also says how a value
travels when implementations would differ: decimals and large integers as
strings, and a string of digits stays a string.

### Access

Access is fail-closed. An operation, command or page without a
permission is invalid, and a permission nobody is granted is reported. There
is no default that opens anything.

### Expressions

The expression language is a small subset of CEL, the Common Expression
Language, whose C-like syntax most programmers already read. It has a short,
closed list of operators and functions, and CEL's strict types: an int, a
double and a decimal never mix unless a conversion is written, because an
implicit conversion is exactly where precision is lost unseen. Anything
outside the list is refused with a message that names it.

### The validator

The validator's messages say what is wrong, where it is (file, line and
YAML path) and how to fix it, in one plain sentence each. It reports every
problem in one run. It does not guess: a file it cannot interpret fails.

### Generated output

Generated code and documents are boring, explicit and readable by
someone who has never heard of SpecArch. They carry a header saying where
they came from, and they are never edited by hand.

### Documents

Design files, schemas, documents and the README always read as the current
truth. A change log only grows, and the reader of a design wants the design
as it is now, not the story of how it got there. So none of them says
"previously", "changed from", "updated on" or "new in"; the history of
changes lives in `history/`, one file per day, and in git. Version numbers
stay: they are facts about the current file.

Documents in this repository give the summary first. A section with
nothing to say is left out rather than filled with a placeholder.
