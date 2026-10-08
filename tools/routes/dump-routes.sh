#!/bin/sh
# dump-routes.sh writes the route table that specarch extract router reads.
# It runs the project's own route printer in the folder the router is built
# from, and wraps what the printer writes with the version of the format,
# the folder's path from the repository's root and the commit that last
# changed it.
#
#   tools/routes/dump-routes.sh <router folder> <dump file> <printer> [<argument>...]
#
# The printer is a command of the project, such as a flag of its server or
# a small program beside it, that builds the router as the server does and
# writes every route it registered to standard output as a JSON array, one
# object per route:
#
#   [
#       {"method": "POST", "path": "/loans/{loanId}/return", "permission": "loans.write", "handler": "ReturnBook"}
#   ]
#
# - method: the HTTP method in capitals.
# - path: the path, starting with /, each parameter written {name}.
# - permission: the one permission the route checks, or null when it
#   checks none.
# - handler: the name of the function that answers it.
#
# The four keys are all there is; a route table with any other key, or
# without one of these, is refused.
#
# The folder must be committed: a change not committed, or a file that is
# neither tracked nor ignored, is refused, since no commit would name what
# the routes were read from. Commit the dump beside the code afterwards.
set -eu

if [ $# -lt 3 ]; then
	echo "usage: dump-routes.sh <router folder> <dump file> <printer> [<argument>...]" >&2
	exit 2
fi
folder=$1
dump=$2
shift 2

if [ ! -d "$folder" ]; then
	echo "dump-routes.sh: $folder is not a folder" >&2
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
	echo "dump-routes.sh: $path has a quote or a backslash, which the route table does not take in a path" >&2
	exit 2
	;;
esac
if [ -n "$(git_clean status --porcelain --untracked-files=all -- .)" ]; then
	echo "dump-routes.sh: $path has changes not committed or files not tracked; commit them first" >&2
	exit 1
fi
commit=$(git_clean log -1 --format=%H -- .)
if [ -z "$commit" ]; then
	echo "dump-routes.sh: no commit has changed $path" >&2
	exit 1
fi

routes=$(cd "$folder" && "$@")
if [ -z "$routes" ]; then
	echo "dump-routes.sh: the printer wrote nothing" >&2
	exit 1
fi

{
	printf '{\n    "routeTable": 1,\n    "path": "%s",\n    "commit": "%s",\n    "routes": ' "$path" "$commit"
	# The printer's lines after the first sit inside the object.
	printf '%s\n' "$routes" | sed '2,$s/^/    /'
	printf '}\n'
} >"$dump.tmp"
mv "$dump.tmp" "$dump"
echo "dump-routes.sh: wrote $dump from $path at $commit in $root"
