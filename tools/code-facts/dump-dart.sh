#!/bin/sh
# dump-dart.sh writes the code-facts dump that specarch extract dart
# reads (ADR-092). It runs the Dart reader, readers/dart in this
# repository, on the tracked Dart files under a folder, and the reader
# writes every fact the analyzer's parser gives it with the version of the
# format, the parser's name and version, the folder's path from the
# repository's root and the commit that last changed it.
#
#   tools/code-facts/dump-dart.sh <folder> <dump file>
#
# The folder must be committed: a change not committed, or a file that is
# neither tracked nor ignored, is refused, since no commit would name what
# the facts were read from. Commit the dump beside the code afterwards; it
# is stale once the folder changes, and specarch extract dart refuses it
# then, and when another version of the analyzer made it than the one
# specarch pins.
#
# The reader needs the Dart SDK; the first run fetches the analyzer with
# dart pub get at the version readers/dart/pubspec.lock pins. The parser
# reads only the files listed and resolves no package, so no pub cache of
# the folder read is needed.
set -eu

if [ $# -ne 2 ]; then
	echo "usage: dump-dart.sh <folder> <dump file>" >&2
	exit 2
fi
folder=$1
dump=$2

if [ ! -d "$folder" ]; then
	echo "dump-dart.sh: $folder is not a folder" >&2
	exit 2
fi
case $dump in
/*) ;;
*) dump="$(pwd)/$dump" ;;
esac
reader=$(cd "$(dirname "$0")/../../readers/dart" && pwd)
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
	echo "dump-dart.sh: $path has changes not committed or files not tracked; commit them first" >&2
	exit 1
fi
commit=$(git_clean log -1 --format=%H -- .)
if [ -z "$commit" ]; then
	echo "dump-dart.sh: no commit has changed $path" >&2
	exit 1
fi

if [ ! -f "$reader/.dart_tool/package_config.json" ]; then
	dart pub get --enforce-lockfile --directory "$reader" >&2
fi

# The tracked Dart files, from the repository's root.
(cd "$root" && GIT_CONFIG_GLOBAL=/dev/null GIT_CONFIG_NOSYSTEM=1 git -c core.quotePath=false ls-files -- "$path" | grep '\.dart$' || true) >"$dump.files"
(cd "$root" && dart run "$reader/bin/code_facts_dart.dart" "$path" "$commit" <"$dump.files") >"$dump.tmp"
rm -f "$dump.files"
mv "$dump.tmp" "$dump"
echo "dump-dart.sh: wrote $dump from $path at $commit in $root"
