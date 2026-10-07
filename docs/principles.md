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

## How SpecArch applies them

### The two kinds of file

They are the needle box and the barn. A design file
says what the system is and does. An implementation file says how one stack
builds it. Generators and runtimes hold the logic. A change to the design
never touches a generator, and a library upgrade never touches the design.

### The meta-model

The meta-model uses full English words for its own keys
(`specarchImplementation`, `standardOutput`, `repeatable`, `exitCodes`).
Keywords borrowed from JSON Schema, OpenAPI and AsyncAPI keep their standard
spelling, because a reader who knows the standard already knows them. Every
object is closed: an unknown key is an error, not something ignored. The
smallest valid design file is two keys and reads top to bottom.

### Access

Access is fail-closed. An operation, command or page without a
permission is invalid, and a permission nobody is granted is reported. There
is no default that opens anything.

### Expressions

The expression language is a small subset of CEL, the Common Expression
Language, whose C-like syntax most programmers already read. It has a short,
closed list of operators and functions. Anything outside the list is
refused with a message that names it.

### The validator

The validator's messages say what is wrong, where it is (file, line and
YAML path) and how to fix it, in one plain sentence each. It reports every
problem in one run. It does not guess: a file it cannot interpret fails.

### Generated output

Generated code and documents are boring, explicit and readable by
someone who has never heard of SpecArch. They carry a header saying where
they came from, and they are never edited by hand.

### Documents

Documents in this repository give the summary first. A section with
nothing to say is left out rather than filled with a placeholder.
