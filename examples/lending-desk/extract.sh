#!/bin/sh
# extract.sh extracts the lending desk end to end: every surface its
# sources hold, each by its own reader into a tree of its own, then the
# trees merged into one specification, validated, and its open questions
# listed. It is the procedure of docs/extraction.md run on the example, and
# what CI runs twice to show the output is the same.
#
#   examples/lending-desk/extract.sh <out folder>
#
# The out folder must be new or empty. It gets:
#
# - trees/<reader>: the tree each reader wrote, and <reader>.txt its
#   standard output: manual (extract documents), openapi, database,
#   router, go, permissions, pages and workflows. The go reader reads the
#   service's source, with the permission check its implementation file
#   names, and merges with the route table its printer wrote. The pages
#   reader reads the web folder's code through its code-facts dump,
#   sources/facts/web.json, which tools/code-facts/dump-javascript.sh
#   makes again once sources/web changes.
# - spec: the merged specification, and merge.txt the merge's standard
#   output.
# - validate.txt and gaps.txt: what specarch validate and specarch gaps
#   print for spec.
#
# The readers read the committed dumps under sources/, so no database and
# no Podman is needed; the dumps are remade by hand with the scripts under
# tools/. A source with changes not committed is refused by its reader.
#
# The exit status is 0 when every step ran and the merged specification
# has no errors. specarch gaps exits 1 while a question is open, which is
# the normal state of a specification read from sources, so that status
# is not a failure here.
#
# The steps run in the out folder and name what they write relative to
# it, so what they print is the same wherever the folder is.
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

# read_surface <tree> <reader> <path>: one reader into a tree of its own.
read_surface() {
	name=$1
	source=$2
	path=$3
	$specarch extract "$source" --out "trees/$name" "$path" >"$name.txt"
}

read_surface manual documents "$sources/manual.md"
read_surface database database "$sources/catalogue/catalogue.json"
read_surface router router "$sources/routes/routes.json"
$specarch extract go --implementation "$here/spec/implementation/go/lending-desk.go.specarch-implementation.yaml" \
	--out trees/go "$sources/code" >go.txt
read_surface permissions permissions "$sources/permissions/permissions.json"
read_surface openapi openapi "$sources/openapi/openapi.yaml"
$specarch extract pages --facts "$sources/facts/web.json" --out trees/pages "$sources/web/app" >pages.txt
read_surface workflows workflows "$sources/workflows/write-off.bpmn"

# The documents first, the code after, so the merge's questions come in
# the order an owner reads the sources.
$specarch merge --out spec \
	trees/manual trees/openapi \
	trees/database trees/router trees/go trees/permissions trees/pages \
	trees/workflows >merge.txt

$specarch validate spec >validate.txt 2>&1

status=0
$specarch gaps spec >gaps.txt || status=$?
if [ "$status" -gt 1 ]; then
	echo "extract.sh: specarch gaps failed with status $status" >&2
	exit 1
fi
echo "extract.sh: wrote $out/spec; $(tail -n 1 validate.txt)"
