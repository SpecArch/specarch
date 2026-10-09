#!/bin/sh
# dump-javascript.sh writes the code-facts dump that specarch extract
# javascript reads (ADR-089). It runs the JavaScript reader,
# readers/javascript in this repository, on the tracked JavaScript,
# TypeScript and Vue files under a folder and its tsconfig.json and jsconfig.json
# files, and the reader writes every fact the TypeScript compiler gives it
# with the version of the format, the compiler's name and version, the
# folder's path from the repository's root and the commit that last
# changed it.
#
#   tools/code-facts/dump-javascript.sh <folder> <dump file>
#
# The folder must be committed: a change not committed, or a file that is
# neither tracked nor ignored, is refused, since no commit would name what
# the facts were read from. Commit the dump beside the code afterwards; it
# is stale once the folder changes, and specarch extract javascript
# refuses it then, and when another version of the TypeScript compiler
# made it than the one specarch pins.
#
# The reader needs Node; the first run installs the TypeScript compiler
# with npm ci at the version readers/javascript/package-lock.json pins.
# The compiler reads only the files listed, never node_modules.
set -eu

if [ $# -ne 2 ]; then
	echo "usage: dump-javascript.sh <folder> <dump file>" >&2
	exit 2
fi
folder=$1
dump=$2

if [ ! -d "$folder" ]; then
	echo "dump-javascript.sh: $folder is not a folder" >&2
	exit 2
fi
case $dump in
/*) ;;
*) dump="$(pwd)/$dump" ;;
esac
reader=$(cd "$(dirname "$0")/../../readers/javascript" && pwd)
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
	echo "dump-javascript.sh: $path has changes not committed or files not tracked; commit them first" >&2
	exit 1
fi
commit=$(git_clean log -1 --format=%H -- .)
if [ -z "$commit" ]; then
	echo "dump-javascript.sh: no commit has changed $path" >&2
	exit 1
fi

if [ ! -d "$reader/node_modules/typescript" ]; then
	(cd "$reader" && npm ci --ignore-scripts --no-audit --no-fund) >&2
fi

# The tracked source and configuration files, from the repository's root.
(cd "$root" && GIT_CONFIG_GLOBAL=/dev/null GIT_CONFIG_NOSYSTEM=1 git -c core.quotePath=false ls-files -- "$path" |
	grep -E '(\.(ts|tsx|mts|cts|js|jsx|mjs|cjs|vue)$|(^|/)(tsconfig|jsconfig)\.json$)' | grep -v '\.d\.[cm]\?ts$' | grep -v '(^|/)node_modules/' || true) >"$dump.files"
(cd "$root" && node "$reader/code-facts-javascript.mjs" "$path" "$commit" <"$dump.files") >"$dump.tmp"
rm -f "$dump.files"
mv "$dump.tmp" "$dump"
echo "dump-javascript.sh: wrote $dump from $path at $commit in $root"
