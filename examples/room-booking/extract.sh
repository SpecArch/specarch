#!/bin/sh
# extract.sh extracts the room booking service twice, from the code-facts
# dumps of its TypeScript and its plain JavaScript source. Each dump is
# read into a tree of its own and validated, and the questions of the two
# trees are compared. It is what CI runs twice to show the output is the
# same.
#
#   examples/room-booking/extract.sh <out folder>
#
# The out folder must be new or empty. It gets:
#
# - ts and js: the tree each reading wrote, ts.txt and js.txt their
#   standard output, and ts-validate.txt and js-validate.txt what
#   specarch validate prints for each.
# - differences.txt: each question only one of the trees asks, marked
#   ts or js, with its numbers, its line numbers and the file extensions
#   left out, so that what is left is what types change.
#
# The exit status is 0 when every step ran and neither tree has errors.
#
# The dumps are committed in sources/facts. tools/code-facts/dump-javascript.sh
# makes them again from sources/ts and sources/js once either changes,
# which needs Node; this script does not.
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
mkdir -p "$out"
cd "$out"

for lang in ts js; do
	$specarch extract javascript --implementation "$here/implementation/room-booking-checks.yaml" --out "$lang" "$sources/facts/$lang.json" >"$lang.txt"
	$specarch validate "$lang" >"$lang-validate.txt" 2>&1
done

# The questions of a tree, one per line, with what tells the two
# languages apart only by their files left out.
questions() {
	grep -h '^    question: ' "$1"/design/questions.yaml "$1"/deployment/questions.yaml 2>/dev/null |
		sed -E -e 's/^    question: //' -e "s/^'//" -e "s/'\$//" -e "s/''/'/g" \
			-e 's/[.](ts|js):[0-9]+/:N/g' -e 's/[.](ts|js)([^A-Za-z]|$)/\2/g' -e 's#sources/(ts|js)/#sources/#g' |
		LC_ALL=C sort
}
questions ts >ts-questions.txt
questions js >js-questions.txt
LC_ALL=C comm -23 ts-questions.txt js-questions.txt | sed 's/^/ts: /' >differences.txt
LC_ALL=C comm -13 ts-questions.txt js-questions.txt | sed 's/^/js: /' >>differences.txt
rm ts-questions.txt js-questions.txt

echo "extract.sh: wrote $out/ts and $out/js; $(tail -n 1 ts-validate.txt); $(tail -n 1 js-validate.txt)"
