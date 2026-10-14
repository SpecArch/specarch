#!/bin/sh
# dump-workflows.sh writes the workflow table that specarch extract
# workflows --table reads, for workflow definitions in a format of the
# project's own rather than BPMN 2.0 (ADR-118). It runs the project's own
# workflow printer in the folder the engine loads its definitions from, and
# wraps what the printer writes with the version of the format, the
# folder's path from the repository's root and the commit that last changed
# it.
#
#   tools/workflows/dump-workflows.sh <definitions folder> <dump file> <printer> [<argument>...]
#
# The printer is a command of the project that loads the definitions as
# the engine does, from the files the engine's own loading reads, and
# writes every workflow it loaded to standard output as a JSON array, one
# object per workflow:
#
#   [
#       {
#           "name": "write-off",
#           "file": "flows/write-off.json",
#           "description": "A lost book's loan is written off only after a desk supervisor approves.",
#           "trigger": {
#               "operation": "requestWriteOff",
#               "form": [{"name": "loanId", "type": "string"}]
#           },
#           "steps": [
#               {"name": "approve", "kind": "approval", "approvers": ["desk-supervisor"], "deadline": "P2D", "onDeadline": "refuse", "escalateTo": null},
#               {"name": "writeOff", "kind": "operation", "operation": "writeOffLoan"}
#           ],
#           "emits": []
#       }
#   ]
#
# - name: the workflow's name in the engine.
# - file: the file it is defined in, from the repository's root, or null.
# - description: what the workflow is for, or null.
# - trigger: the operation that starts it, by its operationId or null,
#   and its form's fields, each a name and the engine's type for it.
# - steps: in the order the engine runs them, each a name and a kind:
#   - approval: the roles that may approve (approvers), the deadline as an
#     ISO 8601 duration or null, onDeadline refuse, escalate or null, and
#     escalateTo, the name of the later approval it escalates to, or null;
#   - operation: the operation the step calls, by its operationId or null;
#   - other: a step that is neither, with a description of what it does
#     or null; the steps after it are not read.
# - emits: how it ends: the messages it publishes when it ends, each
#   written channel/Message, or [].
#
# Those keys are all there is, and each is written, null where the
# printer cannot say; a workflow table with any other key, or without one
# of these, is refused.
#
# The folder must be committed: a change not committed, or a file that is
# neither tracked nor ignored, is refused, since no commit would name what
# the workflows were read from. Commit the dump beside the code afterwards,
# or in a repository holding the code's as a submodule
# (tools/repository-key.sh).
set -eu

if [ $# -lt 3 ]; then
	echo "usage: dump-workflows.sh <definitions folder> <dump file> <printer> [<argument>...]" >&2
	exit 2
fi
folder=$1
dump=$2
shift 2

if [ ! -d "$folder" ]; then
	echo "dump-workflows.sh: $folder is not a folder" >&2
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
	echo "dump-workflows.sh: $path has a quote or a backslash, which the workflow table does not take in a path" >&2
	exit 2
	;;
esac
if [ -n "$(git_clean status --porcelain --untracked-files=all -- .)" ]; then
	echo "dump-workflows.sh: $path has changes not committed or files not tracked; commit them first" >&2
	exit 1
fi
commit=$(git_clean log -1 --format=%H -- .)
if [ -z "$commit" ]; then
	echo "dump-workflows.sh: no commit has changed $path" >&2
	exit 1
fi
# A dump file in another repository, one holding the code's as a
# submodule, names that submodule's folder as its repository.
. "$(dirname "$0")/../repository-key.sh"
repository=$(repository_key dump-workflows.sh "$root" "$dump")

workflows=$(cd "$folder" && "$@")
if [ -z "$workflows" ]; then
	echo "dump-workflows.sh: the printer wrote nothing" >&2
	exit 1
fi

{
	printf '{\n    "workflowTable": 1,\n    "path": "%s",\n    "commit": "%s",\n    "workflows": ' "$path" "$commit"
	# The printer's lines after the first sit inside the object.
	printf '%s\n' "$workflows" | sed '2,$s/^/    /'
	printf '}\n'
} >"$dump.tmp"
add_repository_key dump-workflows.sh "$dump.tmp" "$repository"
mv "$dump.tmp" "$dump"
echo "dump-workflows.sh: wrote $dump from $path at $commit in $root${repository:+, read through the submodule $repository}"
