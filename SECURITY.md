# Security

## Reporting a weakness

Report a security weakness in SpecArch only through GitHub's private
vulnerability reporting: open the repository's Security tab and choose
"Report a vulnerability"
(<https://github.com/SpecArch/specarch/security/advisories/new>). The report
reaches the owner alone, and the fix is worked on in a private advisory.

Never describe a weakness in a public issue, pull request, discussion or
commit message before the release tag that fixes it is out. That holds for
everyone, the owner included: until the fixed tag exists, anyone who reads
the description can use it against every copy in use. Once the tag is out,
the advisory is published and the weakness may be discussed in the open.

Issues stay open for everything else: bugs that are not a weakness,
questions and suggestions. When it is not clear whether a bug is a weakness,
report it privately; it can be moved to an issue afterwards.

## What to include

- The version or commit, and how SpecArch was run (the command and its
  arguments).
- The smallest input that shows the weakness, and what it makes SpecArch do.
- What an attacker would need: who supplies the input, and what they gain.

## Supported versions

Fixes are made on `main` and released as a new version tag. Older tags are
not patched; take the newest release.
