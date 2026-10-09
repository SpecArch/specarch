#!/bin/sh
# extract.sh extracts the reading list, an iPhone app and the Vapor server
# it calls, from the code-facts dumps of their Swift source. Each dump is
# read into a tree of its own, the trees are merged, the result validated,
# and its open questions listed. It is what CI runs twice to show the
# output is the same.
#
#   examples/reading-list/extract.sh <out folder>
#
# The out folder must be new or empty. It gets:
#
# - trees/app and trees/server: the tree each reading wrote, and app.txt
#   and server.txt their standard output.
# - spec: the merged specification, and merge.txt the merge's standard
#   output.
# - validate.txt and gaps.txt: what specarch validate and specarch gaps
#   print for spec.
#
# The exit status is 0 when every step ran and the merged specification
# has no errors; specarch gaps exits 1 while a question is open, which is
# the normal state of a specification read from sources.
#
# The dumps are committed in sources/facts. tools/code-facts/dump-swift.sh
# makes them again from sources/app and sources/server once either
# changes, which needs the Swift toolchain; this script does not.
#
# SPECARCH names the command that runs specarch, when it is not specarch
# itself; it runs in the out folder, so a path in it is absolute.
set -eu

if [ $# -ne 1 ]; then
	echo "usage: extract.sh <out folder>" >&2
	exit 2
fi
out=$1
specarch=${SPECARCH:-specarch}
here=$(cd "$(dirname "$0")" && pwd)
sources=$here/sources

if [ -e "$out" ] && [ -n "$(ls -A "$out")" ]; then
	echo "extract.sh: $out is not empty" >&2
	exit 2
fi
mkdir -p "$out/trees"
cd "$out"

$specarch extract swift --source-key app --out trees/app "$sources/facts/app.json" >app.txt
$specarch extract swift --source-key server --implementation "$here/implementation/server-checks.yaml" --out trees/server "$sources/facts/server.json" >server.txt

$specarch merge --out spec trees/app trees/server >merge.txt

$specarch validate spec >validate.txt 2>&1

status=0
$specarch gaps spec >gaps.txt || status=$?
if [ "$status" -gt 1 ]; then
	echo "extract.sh: specarch gaps failed with status $status" >&2
	exit 1
fi
echo "extract.sh: wrote $out/spec; $(tail -n 1 validate.txt)"
