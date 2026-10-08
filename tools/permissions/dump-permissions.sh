#!/bin/sh
# dump-permissions.sh writes the permission table that specarch extract
# permissions reads. It runs the project's own permission printer in the
# folder the permission check is built from, and wraps what the printer
# writes with the version of the format, the folder's path from the
# repository's root and the commit that last changed it.
#
#   tools/permissions/dump-permissions.sh <folder> <dump file> <printer> [<argument>...]
#
# The printer is a command of the project that reads the grants where the
# running check reads them, such as the tables its seed scripts fill and
# not a policy file the check never loads, and writes one JSON object to
# standard output, its first line { alone:
#
#   {
#       "grants": [
#           {"role": "desk-staff", "permission": "loans.read"}
#       ],
#       "gates": [
#           {"check": "Allowed", "setting": "AUTHORISATION_URL"}
#       ]
#   }
#
# - grants: every role and permission it grants, one object per pair.
# - gates: every permission check that runs only when a setting is
#   present, and so lets every request through when the setting is
#   empty: the check's name and the setting's. The project declares
#   these itself; an empty list says no check has one.
#
# The keys named here are all there is; a permission table with any other
# key, or without one of these, is refused.
#
# The folder must be committed: a change not committed, or a file that is
# neither tracked nor ignored, is refused, since no commit would name what
# the grants were read from. Commit the dump beside the code afterwards.
set -eu

if [ $# -lt 3 ]; then
	echo "usage: dump-permissions.sh <folder> <dump file> <printer> [<argument>...]" >&2
	exit 2
fi
folder=$1
dump=$2
shift 2

if [ ! -d "$folder" ]; then
	echo "dump-permissions.sh: $folder is not a folder" >&2
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
	echo "dump-permissions.sh: $path has a quote or a backslash, which the permission table does not take in a path" >&2
	exit 2
	;;
esac
if [ -n "$(git_clean status --porcelain --untracked-files=all -- .)" ]; then
	echo "dump-permissions.sh: $path has changes not committed or files not tracked; commit them first" >&2
	exit 1
fi
commit=$(git_clean log -1 --format=%H -- .)
if [ -z "$commit" ]; then
	echo "dump-permissions.sh: no commit has changed $path" >&2
	exit 1
fi

printed=$(cd "$folder" && "$@")
if [ "$(printf '%s\n' "$printed" | head -n 1)" != "{" ]; then
	echo "dump-permissions.sh: the printer's first line is not { alone" >&2
	exit 1
fi

{
	printf '{\n    "permissionTable": 1,\n    "path": "%s",\n    "commit": "%s",\n' "$path" "$commit"
	printf '%s\n' "$printed" | sed '1d'
} >"$dump.tmp"
mv "$dump.tmp" "$dump"
echo "dump-permissions.sh: wrote $dump from $path at $commit in $root"
