#!/bin/sh
# dump-swift.sh writes the code-facts dump that specarch extract swift
# reads (ADR-087). It runs the Swift reader, readers/swift in this
# repository, on the tracked Swift files under a folder, and the reader
# writes every fact SwiftSyntax gives it with the version of the format,
# the parser's name and version, the folder's path from the repository's
# root and the commit that last changed it.
#
#   tools/code-facts/dump-swift.sh <folder> <dump file>
#
# The folder must be committed: a change not committed, or a file that is
# neither tracked nor ignored, is refused, since no commit would name what
# the facts were read from. Commit the dump beside the code afterwards; it
# is stale once the folder changes, and specarch extract swift refuses it
# then, and when another version of SwiftSyntax made it than the one
# specarch pins.
#
# The reader is built with swift build the first time it runs, which
# needs the Swift toolchain and fetches swift-syntax at the version
# readers/swift/Package.resolved pins.
set -eu

if [ $# -ne 2 ]; then
	echo "usage: dump-swift.sh <folder> <dump file>" >&2
	exit 2
fi
folder=$1
dump=$2

if [ ! -d "$folder" ]; then
	echo "dump-swift.sh: $folder is not a folder" >&2
	exit 2
fi
case $dump in
/*) ;;
*) dump="$(pwd)/$dump" ;;
esac
reader=$(cd "$(dirname "$0")/../../readers/swift" && pwd)
# Read git without the user's or the system's configuration, as specarch
# extract does, so the same commit gives the same answer everywhere.
git_clean() {
	GIT_CONFIG_GLOBAL=/dev/null GIT_CONFIG_NOSYSTEM=1 git -c core.quotePath=false -C "$folder" "$@"
}
root=$(git_clean rev-parse --show-toplevel)
prefix=$(git_clean rev-parse --show-prefix)
path=${prefix%/}
if [ -z "$path" ]; then
	path=.
fi
if [ -n "$(git_clean status --porcelain --untracked-files=all -- .)" ]; then
	echo "dump-swift.sh: $path has changes not committed or files not tracked; commit them first" >&2
	exit 1
fi
commit=$(git_clean log -1 --format=%H -- .)
if [ -z "$commit" ]; then
	echo "dump-swift.sh: no commit has changed $path" >&2
	exit 1
fi

swift build -c release --package-path "$reader" --product code-facts-swift >&2
bin=$(swift build -c release --package-path "$reader" --show-bin-path)/code-facts-swift

# The tracked Swift files, from the repository's root.
(cd "$root" && GIT_CONFIG_GLOBAL=/dev/null GIT_CONFIG_NOSYSTEM=1 git -c core.quotePath=false ls-files -- "$path" | grep '\.swift$' || true) >"$dump.files"
(cd "$root" && "$bin" "$path" "$commit" <"$dump.files") >"$dump.tmp"
rm -f "$dump.files"
mv "$dump.tmp" "$dump"
echo "dump-swift.sh: wrote $dump from $path at $commit in $root"
