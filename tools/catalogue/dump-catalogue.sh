#!/bin/sh
# dump-catalogue.sh writes the catalogue dump that specarch extract database
# reads. It starts a disposable PostgreSQL under Podman, applies every
# migration in the folder in the order of the file names, runs
# catalogue.sql, and writes the JSON with the folder's path from the
# repository's root and the commit that last changed it. The container is
# removed afterwards; nothing is kept but the dump.
#
#   tools/catalogue/dump-catalogue.sh <migrations folder> <dump file>
#
# The migrations are the folder's *.sql files, except *.down.sql. A project
# whose migrations need their own tool runs that tool against a database and
# then catalogue.sql with psql, giving the same path and commit.
#
# The folder must be committed: a change not committed, or a file that is
# neither tracked nor ignored, is refused, since no commit would name what
# the dump was made from. Commit the dump beside the code afterwards.
#
# PODMAN names the command that runs podman, when it is not podman itself.
set -eu

image='docker.io/library/postgres:18.6-alpine@sha256:77f585114c32fbca283dc835b0596f4e52b51b4c6662d7810b2f4084f60a1873'
podman_cmd=${PODMAN:-podman}

if [ $# -ne 2 ]; then
	echo "usage: dump-catalogue.sh <migrations folder> <dump file>" >&2
	exit 2
fi
folder=$1
dump=$2
query="$(cd "$(dirname "$0")" && pwd)/catalogue.sql"

if [ ! -d "$folder" ]; then
	echo "dump-catalogue.sh: $folder is not a folder" >&2
	exit 2
fi
# Read git without the user's or the system's configuration, as specarch
# extract does, so the same commit gives the same answer everywhere.
git_clean() {
	GIT_CONFIG_GLOBAL=/dev/null GIT_CONFIG_NOSYSTEM=1 git -C "$folder" "$@"
}
root=$(git_clean rev-parse --show-toplevel)
prefix=$(git_clean rev-parse --show-prefix)
path=${prefix%/}
if [ -z "$path" ]; then
	path=.
fi
if [ -n "$(git_clean status --porcelain --untracked-files=all -- .)" ]; then
	echo "dump-catalogue.sh: $path has changes not committed or files not tracked; commit them first" >&2
	exit 1
fi
commit=$(git_clean log -1 --format=%H -- .)
if [ -z "$commit" ]; then
	echo "dump-catalogue.sh: no commit has changed $path" >&2
	exit 1
fi

name="specarch-catalogue-$$"
cleanup() {
	$podman_cmd rm -f "$name" >/dev/null 2>&1 || true
}
trap cleanup EXIT INT TERM

$podman_cmd run -d --rm --name "$name" \
	-e POSTGRES_PASSWORD=catalogue -e POSTGRES_USER=catalogue -e POSTGRES_DB=catalogue \
	"$image" >/dev/null

tries=0
until $podman_cmd exec "$name" pg_isready -q -U catalogue -d catalogue -h 127.0.0.1 2>/dev/null; do
	tries=$((tries + 1))
	if [ "$tries" -gt 60 ]; then
		echo "dump-catalogue.sh: the database did not start" >&2
		exit 1
	fi
	sleep 1
done

psql_in() {
	$podman_cmd exec -i "$name" psql -X -q -v ON_ERROR_STOP=1 -h 127.0.0.1 -U catalogue -d catalogue "$@"
}

# Only tracked files are applied, in the order of their names.
git_clean ls-files -z -- '*.sql' | tr '\0' '\n' | LC_ALL=C sort | while IFS= read -r file; do
	case $file in
	*/*) continue ;; # only the folder's own files, not its sub-folders
	*.down.sql) continue ;;
	esac
	psql_in -f - <"$folder/$file" >/dev/null
done

psql_in -A -t -v path="$path" -v commit="$commit" -f - <"$query" >"$dump.tmp"
mv "$dump.tmp" "$dump"
echo "dump-catalogue.sh: wrote $dump from $path at $commit in $root"
