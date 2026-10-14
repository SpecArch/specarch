#!/bin/sh
# pull.sh copies the Go source of the TypeScript compiler that specarch's
# JavaScript reader is built on, from github.com/microsoft/typescript-go at
# a release tag, into third_party/typescript-go (ADR-116).
#
#   tools/typescript-go/pull.sh <tag>        such as typescript/v7.0.2
#
# Upstream keeps its compiler under internal/, which Go lets no other
# module import, and offers no public Go API. So the packages the program
# and the checker need, and only those, are copied out of internal/ with
# their import paths rewritten, with the files they embed (the bundled
# lib.d.ts files and the diagnostics' translations), upstream's LICENSE and
# NOTICE.txt, and UPSTREAM naming the tag and its commit. Nothing in the
# copy is edited by hand: to change it, pull another tag.
#
# After a pull: put the versions upstream's go.mod pins for the modules the
# copy imports into go.mod, scan them (syft, grype, govulncheck), change the
# version the reader writes into its dumps (typeScriptVersion in
# internal/extract/javascript.go and internal/facts), remake every
# JavaScript code-facts dump with specarch facts javascript, and run the
# conformance cases; every difference in a dump is a change of the
# compiler's answers and is named in the history.
set -eu

if [ $# -ne 1 ]; then
	echo "usage: pull.sh <tag>" >&2
	exit 2
fi
tag=$1
here=$(cd "$(dirname "$0")/../.." && pwd)
dest=$here/third_party/typescript-go
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT

git -c advice.detachedHead=false clone -q --depth 1 --branch "$tag" https://github.com/microsoft/typescript-go "$work/src"
commit=$(git -C "$work/src" rev-parse HEAD)
upstream=github.com/microsoft/typescript-go/internal
local=github.com/SpecArch/specarch/third_party/typescript-go

# The packages the program, the checker, the bundled libraries and the
# operating system's file system need, by their folder under internal/.
pkgs=$(cd "$work/src" && go list -deps -f '{{.ImportPath}}' ./internal/compiler ./internal/bundled ./internal/vfs/osvfs | sed -n "s#^$upstream/##p")

rm -rf "$dest"
mkdir -p "$dest"
for p in $pkgs; do
	from=$work/src/internal/$p
	to=$dest/$p
	mkdir -p "$to"
	for f in "$from"/*.go; do
		case $f in
		*_test.go) continue ;;
		esac
		# A program that writes upstream's generated files is not part of
		# the package.
		if grep -q '^//go:build ignore' "$f"; then
			continue
		fi
		sed "s#\"$upstream/#\"$local/#g" "$f" >"$to/$(basename "$f")"
	done
	# The files the package embeds.
	(cd "$work/src" && go list -f '{{range .EmbedFiles}}{{.}}
{{end}}' "./internal/$p") | while read -r e; do
		[ -n "$e" ] || continue
		mkdir -p "$to/$(dirname "$e")"
		cp "$from/$e" "$to/$e"
	done
done
cp "$work/src/LICENSE" "$dest/LICENSE"
cp "$work/src/NOTICE.txt" "$dest/NOTICE.txt"
cat >"$dest/UPSTREAM" <<END
The Go source of the TypeScript compiler, copied from
https://github.com/microsoft/typescript-go at the tag $tag, commit
$commit, by tools/typescript-go/pull.sh, which says how to pull another
tag. Licensed under the Apache License 2.0 (LICENSE, NOTICE.txt). Only the
packages the program and the checker need are copied, out of internal/,
with their import paths rewritten; nothing else is changed.

Upstream's go.mod at this tag pins:
$(sed -n '/^require (/,/^)/p' "$work/src/go.mod")
END
gofmt -l "$dest" || true
echo "pull.sh: copied $(echo "$pkgs" | wc -w | tr -d ' ') packages of $tag ($commit) into third_party/typescript-go"
