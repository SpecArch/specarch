# repository-key.sh is sourced by the dump scripts. A dump may be
# committed in another repository than the code's, one that holds the
# code's repository as a submodule: the dump then names that submodule's
# folder as its repository, and specarch extract reads the code at the
# commit the submodule's gitlink pins (docs/extraction.md).

# repository_key <script> <code root> <dump file> prints the folder of the
# code's repository from the root of the repository the dump file is in,
# or nothing when the dump file is in the code's own repository or in no
# repository. A dump file in a repository that does not hold the code's
# is refused, since no gitlink there could pin the code's commit.
repository_key() {
	code_root=$(cd "$2" && pwd -P)
	dump_dir=$(dirname "$3")
	if [ ! -d "$dump_dir" ]; then
		echo "$1: the folder $dump_dir of the dump file does not exist" >&2
		return 2
	fi
	dump_root=$(GIT_CONFIG_GLOBAL=/dev/null GIT_CONFIG_NOSYSTEM=1 git -C "$dump_dir" rev-parse --show-toplevel 2>/dev/null) || return 0
	dump_root=$(cd "$dump_root" && pwd -P)
	if [ "$code_root" = "$dump_root" ]; then
		return 0
	fi
	case $code_root in
	"$dump_root"/*) key=${code_root#"$dump_root"/} ;;
	*)
		echo "$1: the dump file $3 is in the repository $dump_root, which does not hold the code's repository $code_root as a folder; write the dump in the code's repository, or in one that holds it as a submodule" >&2
		return 1
		;;
	esac
	case $key in
	*'"'* | *'\'*)
		echo "$1: the folder $key has a quote or a backslash, which a dump does not take in its repository" >&2
		return 2
		;;
	esac
	printf '%s\n' "$key"
}

# add_repository_key <script> <file> <key> writes the key into the dump
# file as its repository, on the line after the opening brace, when the
# key is not empty.
add_repository_key() {
	if [ -z "$3" ]; then
		return 0
	fi
	if [ "$(head -n 1 "$2")" != "{" ]; then
		echo "$1: the dump does not start with a line holding only {, so the repository cannot be written into it" >&2
		return 1
	fi
	{
		printf '{\n    "repository": "%s",\n' "$3"
		tail -n +2 "$2"
	} >"$2.key"
	mv "$2.key" "$2"
}
