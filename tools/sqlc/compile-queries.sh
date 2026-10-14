#!/bin/sh
# compile-queries.sh checks a folder of sqlc queries against a folder of
# migrations: sqlc reads the migrations as the schema and type-checks every
# query against it, the check sqlc generate runs before it writes any Go.
# It runs sqlc vet, with no rules of its own, in a disposable container
# under Podman, with the files copied in, so nothing on the disk is written
# and no sqlc is installed. vet parses and type-checks each query as
# generate does; sqlc compile and generate with no gen block check nothing
# and exit 0. The exit status is sqlc's: 0 when every query compiles.
#
#   tools/sqlc/compile-queries.sh <migrations folder> <queries folder>
#
# The migrations are the folder's *.sql files, except *.down.sql, read in
# the order of their names; the queries are every *.sql file of the other
# folder, generated and written by hand alike.
#
# PODMAN names the command that runs podman, when it is not podman itself
# (docker takes the same arguments).
set -eu

# sqlc 1.31.1, MIT, the multi-platform index.
image='docker.io/sqlc/sqlc:1.31.1@sha256:70f53171d27b2424e9358869975455a6e955a5aa8e58a998a270a6e34e525537'
podman_cmd=${PODMAN:-podman}

if [ $# -ne 2 ]; then
	echo "usage: compile-queries.sh <migrations folder> <queries folder>" >&2
	exit 2
fi
for folder in "$1" "$2"; do
	if [ ! -d "$folder" ]; then
		echo "compile-queries.sh: $folder is not a folder" >&2
		exit 2
	fi
done

work=$(mktemp -d)
name="specarch-sqlc-$$"
cleanup() {
	$podman_cmd rm -f "$name" >/dev/null 2>&1 || true
	rm -rf "$work"
}
trap cleanup EXIT INT TERM

mkdir "$work/schema" "$work/queries"
found=0
for f in "$1"/*.sql; do
	case $f in
	*.down.sql | "$1/*.sql") continue ;;
	esac
	cp "$f" "$work/schema/"
	found=1
done
if [ "$found" -eq 0 ]; then
	echo "compile-queries.sh: $1 holds no migration (*.sql)" >&2
	exit 2
fi
found=0
for f in "$2"/*.sql; do
	if [ "$f" = "$2/*.sql" ]; then
		continue
	fi
	cp "$f" "$work/queries/"
	found=1
done
if [ "$found" -eq 0 ]; then
	echo "compile-queries.sh: $2 holds no query file (*.sql)" >&2
	exit 2
fi
cat >"$work/sqlc.yaml" <<'YAML'
version: "2"
sql:
  - engine: postgresql
    schema: schema
    queries: queries
YAML

$podman_cmd create --name "$name" -w /src "$image" vet >/dev/null
$podman_cmd cp "$work/." "$name:/src"
$podman_cmd start -a "$name"
