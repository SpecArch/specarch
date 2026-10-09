#!/bin/sh
# extract.sh extracts the notice board, a small service on dxlib, from its
# two readings of one surface: the OpenAPI document dxlib emitted from the
# endpoints the service registers (printed), and the Go source read by its
# syntax (parsed). Each is read into a tree of its own, the trees are
# merged, the result validated, and its open questions listed. It is what
# CI runs twice to show the output is the same.
#
#   examples/notice-board/extract.sh <out folder>
#
# The out folder must be new or empty. It gets:
#
# - trees/openapi and trees/go: the tree each reader wrote, and
#   openapi.txt and go.txt their standard output.
# - spec: the merged specification, and merge.txt the merge's standard
#   output, in which every operation the document prints answers the Go
#   tree's question whether the running system registers it.
# - validate.txt and gaps.txt: what specarch validate and specarch gaps
#   print for spec.
#
# The exit status is 0 when every step ran and the merged specification
# has no errors; specarch gaps exits 1 while a question is open, which is
# the normal state of a specification read from sources.
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

$specarch extract openapi --source-key api --out trees/openapi "$sources/openapi/api.openapi.json" >openapi.txt
$specarch extract go --out trees/go "$sources/testdata" >go.txt

$specarch merge --out spec trees/openapi trees/go >merge.txt

$specarch validate spec >validate.txt 2>&1

status=0
$specarch gaps spec >gaps.txt || status=$?
if [ "$status" -gt 1 ]; then
	echo "extract.sh: specarch gaps failed with status $status" >&2
	exit 1
fi
echo "extract.sh: wrote $out/spec; $(tail -n 1 validate.txt)"
