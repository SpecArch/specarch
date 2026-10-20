#!/bin/sh
# dump-gateway.sh writes the gateway table that specarch extract gateway
# reads and specarch check compares (ADR-142). It runs the project's own
# printer in the folder the gateway loads its table of services from, and
# wraps what the printer writes with the version of the format, the
# folder's path from the repository's root and the commit that last changed
# it.
#
#   tools/gateways/dump-gateway.sh <folder> <dump file> <printer> [<argument>...]
#
# The printer is a command of the project that loads the table as the
# gateway does, from the files the gateway's own loading reads, and writes
# one JSON object to standard output, with its opening and closing braces
# on lines of their own:
#
#   {
#       "gateway": "api",
#       "claim": "aud",
#       "environment": "production",
#       "forwards": [
#           {"value": "https://loans.library.example", "name": "loans", "setting": "LOANS_URL", "url": "https://desk.internal.library.example"}
#       ]
#   }
#
# - gateway: the gateway's name as the project calls it.
# - claim: the name of the token's claim whose value selects the service.
# - environment: the environment the addresses were resolved in, or null
#   when the printer ran with the settings unresolved.
# - forwards: one row per entry of the table, each the claim's value; the
#   project's name for the service, or null; the name of the setting the
#   address is read from, or null; and the address as resolved, or null.
#
# Those keys are all there is, and each is written, null where the
# printer cannot say; a gateway table with any other key, or without one
# of these, is refused, and so is a value written twice.
#
# The folder must be committed: a change not committed, or a file that is
# neither tracked nor ignored, is refused, since no commit would name what
# the table was read from. Commit the dump beside the code afterwards, or
# in a repository holding the code's as a submodule
# (tools/repository-key.sh).
set -eu

if [ $# -lt 3 ]; then
	echo "usage: dump-gateway.sh <folder> <dump file> <printer> [<argument>...]" >&2
	exit 2
fi
folder=$1
dump=$2
shift 2

if [ ! -d "$folder" ]; then
	echo "dump-gateway.sh: $folder is not a folder" >&2
	exit 2
fi
case $dump in
/*) ;;
*) dump="$(pwd)/$dump" ;;
esac
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
case $path in
*'"'* | *'\'*)
	echo "dump-gateway.sh: $path has a quote or a backslash, which the gateway table does not take in a path" >&2
	exit 2
	;;
esac
if [ -n "$(git_clean status --porcelain --untracked-files=all -- .)" ]; then
	echo "dump-gateway.sh: $path has changes not committed or files not tracked; commit them first" >&2
	exit 1
fi
commit=$(git_clean log -1 --format=%H -- .)
if [ -z "$commit" ]; then
	echo "dump-gateway.sh: no commit has changed $path" >&2
	exit 1
fi
# A dump file in another repository, one holding the code's as a
# submodule, names that submodule's folder as its repository.
. "$(dirname "$0")/../repository-key.sh"
repository=$(repository_key dump-gateway.sh "$root" "$dump")

table=$(cd "$folder" && "$@")
if [ -z "$table" ]; then
	echo "dump-gateway.sh: the printer wrote nothing" >&2
	exit 1
fi
first=$(printf '%s\n' "$table" | sed -n '1p')
last=$(printf '%s\n' "$table" | sed -n '$p')
if [ "$first" != "{" ] || [ "$last" != "}" ]; then
	echo "dump-gateway.sh: the printer's object does not open and close with a brace on a line of its own" >&2
	exit 1
fi

{
	printf '{\n    "gatewayTable": 1,\n    "path": "%s",\n    "commit": "%s",\n' "$path" "$commit"
	# The printer's members, inside its braces, are the table's own.
	printf '%s\n' "$table" | sed '1d;$d'
	printf '}\n'
} >"$dump.tmp"
add_repository_key dump-gateway.sh "$dump.tmp" "$repository"
mv "$dump.tmp" "$dump"
echo "dump-gateway.sh: wrote $dump from $path at $commit in $root${repository:+, read through the submodule $repository}"
