#!/bin/sh
# dump-swift.sh writes the code-facts dump that specarch extract swift
# reads (ADR-087), for a checkout of this repository: it builds specarch
# from the checkout and runs specarch facts swift, which an installed
# specarch runs the same way with no checkout (ADR-116).
#
#   tools/code-facts/dump-swift.sh <folder> <dump file>
#
# The Swift reader, readers/swift, is embedded in specarch, written once
# into the user's cache and built there with swift build, which needs the
# Swift toolchain and fetches swift-syntax at the version
# readers/swift/Package.resolved pins.
#
# The folder must be committed: a change not committed, or a file that is
# neither tracked nor ignored, is refused, since no commit would name what
# the facts were read from. Commit the dump beside the code afterwards,
# or in a repository holding the code's as a submodule; it is stale once
# the folder changes, and specarch extract swift refuses it then, and
# when another reader version made it than the one specarch pins.
set -eu

if [ $# -ne 2 ]; then
	echo "usage: dump-swift.sh <folder> <dump file>" >&2
	exit 2
fi
here=$(cd "$(dirname "$0")/../.." && pwd)
bin=$(mktemp "${TMPDIR:-/tmp}/specarch.XXXXXX")
trap 'rm -f "$bin"' EXIT
(cd "$here" && go build -o "$bin" ./cmd/specarch)
"$bin" facts swift "$1" "$2"
