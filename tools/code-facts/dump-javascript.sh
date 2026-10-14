#!/bin/sh
# dump-javascript.sh writes the code-facts dump that specarch extract javascript
# reads (ADR-089), for a checkout of this repository: it builds specarch
# from the checkout and runs specarch facts javascript, which an installed
# specarch runs the same way with no checkout (ADR-116).
#
#   tools/code-facts/dump-javascript.sh <folder> <dump file>
#
# The JavaScript reader is inside specarch, on the TypeScript compiler in
# third_party/typescript-go; it needs no Node and reads only the files
# listed, never node_modules.
#
# The folder must be committed: a change not committed, or a file that is
# neither tracked nor ignored, is refused, since no commit would name what
# the facts were read from. Commit the dump beside the code afterwards,
# or in a repository holding the code's as a submodule; it is stale once
# the folder changes, and specarch extract javascript refuses it then, and
# when another reader version made it than the one specarch pins.
set -eu

if [ $# -ne 2 ]; then
	echo "usage: dump-javascript.sh <folder> <dump file>" >&2
	exit 2
fi
here=$(cd "$(dirname "$0")/../.." && pwd)
bin=$(mktemp "${TMPDIR:-/tmp}/specarch.XXXXXX")
trap 'rm -f "$bin"' EXIT
(cd "$here" && go build -o "$bin" ./cmd/specarch)
"$bin" facts javascript "$1" "$2"
