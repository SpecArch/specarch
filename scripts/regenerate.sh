#!/bin/sh
# Writes every generated file landing-check.sh checks, from the repository
# root: the fragment schemas and hint lines, the documents of SpecArch's own
# specification, of examples/library-lending, of examples/lending-desk and
# of examples/release-watcher, the agent skill in skills/specarch,
# the library lending example's generated code and data, and the reading
# room's screens and tests, generated across the estate in examples/estate. To settle a
# conflict in one of these files, take either side, then run this script on
# the merged tree and run scripts/landing-check.sh.
#
# Where heavy-lock is on the PATH, the work runs as one heavy-lock job of
# kind go_build_test, which heavy-lock sends to a Linux runner when one is
# healthy, so run the script bare, not under heavy-lock. The runner gets the
# committed files with the staged changes and no .git; it writes the files
# there, packs every file it wrote or removed into .bin/regenerated, which
# comes back to this checkout, and the script writes them here and removes
# the ones the runner removed. It runs on this Mac instead, with the reason
# printed, when no runner can take it (heavy-lock names why), when files
# have changes that are not staged or are untracked, when the script itself
# runs inside a heavy-lock job, and where there is no heavy-lock.
set -u

if [ ! -f go.mod ] || [ ! -d swift ]; then
	echo "regenerate: run it from the repository root, not $(pwd)" >&2
	exit 2
fi

bin=$(pwd)/.bin

step() {
	name=$1
	shift
	echo "regenerate: $name"
	if ! "$@"; then
		echo "regenerate: FAILED at: $name (on $(uname -n))" >&2
		exit 1
	fi
}

# What a runner sends back: files.tar (every file it wrote, with its path
# from the repository root), removed.txt (one path a line) and host.
back=.bin/regenerated

build_tools() {
	mkdir -p "$bin" || return 1
	go build -o "$bin/specarch" ./cmd/specarch || return 1
	for generator in sql seed sqlc openapi ui ui-typescript ui-vue-htm ui-swiftui bpmn register tests-go deploy; do
		go build -o "$bin/specarch-gen-$generator" ./cmd/specarch-gen-$generator || return 1
	done
}

documents() {
	for target in intent techspec requirements testplan traceability deployment commissioning handover; do
		"$bin/specarch" document $target spec examples || return 1
	done
	for target in techspec requirements testplan traceability deployment commissioning handover manual operations questions problems changes releases communications timeline testruns progress html; do
		"$bin/specarch" document $target spec examples/library-lending || return 1
	done
	for target in techspec requirements traceability questions problems html; do
		"$bin/specarch" document $target examples/lending-desk || return 1
	done
	"$bin/specarch" document operations examples/release-watcher || return 1
}

generated() {
	for target in sql seed sqlc openapi ui bpmn deploy; do
		PATH="$bin:$PATH" "$bin/specarch" generate $target --unapproved examples/library-lending || return 1
	done
	PATH="$bin:$PATH" "$bin/specarch" generate ui --unapproved --estate examples/estate/specarch-estate.yaml examples/estate/reading-room || return 1
	PATH="$bin:$PATH" "$bin/specarch" generate tests --unapproved --estate examples/estate/specarch-estate.yaml examples/estate/reading-room || return 1
}

write_all() {
	step "the fragment schemas and hint lines (go run ./tools/fragment-schemas)" go run ./tools/fragment-schemas
	step "build specarch and the specarch-gen-* plug-ins into .bin" build_tools
	step "the generated documents" documents
	step "the library lending example's generated code and data, and the reading room's screens" generated
	step "the agent skill in skills/specarch (specarch skill)" "$bin/specarch" skill --out skills
}

# sums writes every file of the tree but .bin into $1, one "<checksum>
# <size> <path>" line each, sorted as whole lines for comm.
sums() {
	find . -path ./.bin -prune -o -type f -exec cksum {} + | sed 's| \./| |' | LC_ALL=C sort >"$1"
}

# pack writes $back from the sums taken before ($1) and after ($2).
pack() {
	mkdir -p "$back" || return 1
	# A path is written when its line is new; removed when the path is gone.
	LC_ALL=C comm -13 "$1" "$2" | cut -d' ' -f3- >"$tmp/written.txt" || return 1
	cut -d' ' -f3- "$1" | LC_ALL=C sort >"$tmp/before-paths.txt" || return 1
	cut -d' ' -f3- "$2" | LC_ALL=C sort >"$tmp/after-paths.txt" || return 1
	LC_ALL=C comm -23 "$tmp/before-paths.txt" "$tmp/after-paths.txt" >"$back/removed.txt" || return 1
	tar -cf "$back/files.tar" -T "$tmp/written.txt" || return 1
	uname -n >"$back/host"
	echo "regenerate: packed $(wc -l <"$tmp/written.txt" | tr -d ' ') files written and $(wc -l <"$back/removed.txt" | tr -d ' ') removed on $(uname -n) into $back"
}

# unpack writes what the runner sent back into this checkout and removes the
# files it removed.
unpack() {
	host=$(cat "$back/host" 2>/dev/null || echo "a runner, its name not recorded")
	if tar -tf "$back/files.tar" | grep -E '(^/|(^|/)\.\.(/|$))' >"$tmp/bad.txt"; then
		echo "regenerate: refused to unpack $back/files.tar from $host: it holds paths outside the checkout: $(head -3 "$tmp/bad.txt" | tr '\n' ' ')" >&2
		return 1
	fi
	tar -xf "$back/files.tar" || return 1
	while IFS= read -r f; do
		case $f in
		/* | ../* | */../* | ..) echo "regenerate: refused to remove $f, sent by $host: it is outside the checkout" >&2; return 1 ;;
		esac
		rm -f -- "$f" || return 1
	done <"$back/removed.txt"
	echo "regenerate: wrote $(tar -tf "$back/files.tar" | grep -vc '/$' | tr -d ' ') files and removed $(wc -l <"$back/removed.txt" | tr -d ' ') sent back from $host"
}

# not_to_runner prints why the work cannot go to a runner, or nothing when
# it can.
not_to_runner() {
	if [ -n "${HEAVY_LOCK_HELD:-}" ]; then
		echo "the script runs inside the heavy-lock job ${HEAVY_LOCK_HELD}, and a heavy-lock inside a job runs its command in place; run sh scripts/regenerate.sh bare to use a runner"
		return
	fi
	if ! command -v heavy-lock >/dev/null 2>&1; then
		echo "there is no heavy-lock on the PATH"
		return
	fi
	if ! git rev-parse --git-dir >/dev/null 2>&1; then
		echo "$(pwd) is not a git checkout, so there is no snapshot to send"
		return
	fi
	unstaged=$(git diff --name-only | wc -l | tr -d ' ')
	untracked=$(git ls-files --others --exclude-standard | wc -l | tr -d ' ')
	if [ "$unstaged" != 0 ] || [ "$untracked" != 0 ]; then
		echo "a runner gets the committed files with the staged changes only, and $unstaged tracked files have changes not staged and $untracked files are untracked (git add them, or commit, to send them)"
	fi
}

tmp=$(mktemp -d) || exit 1
trap 'rm -rf "$tmp"' EXIT

case ${1:-} in
_write)
	# In a runner's snapshot (no .git) the files written are packed for the
	# way back; on this Mac they are written in place.
	echo "regenerate: runs on $(uname -n) ($(uname -s) $(uname -m))"
	if [ -e .git ]; then
		write_all
	else
		rm -rf "$back"
		step "the checksums of the tree before" sums "$tmp/before.txt"
		write_all
		step "the checksums of the tree after" sums "$tmp/after.txt"
		step "pack the files written and removed into $back" pack "$tmp/before.txt" "$tmp/after.txt"
	fi
	exit 0
	;;
"") ;;
*)
	echo "regenerate: unknown argument $1; give none" >&2
	exit 2
	;;
esac

rm -rf "$back"
why=$(not_to_runner)
if [ -n "$why" ]; then
	echo "regenerate: runs on this Mac ($(uname -n)): $why"
	if [ -n "${HEAVY_LOCK_HELD:-}" ] || ! command -v heavy-lock >/dev/null 2>&1; then
		sh scripts/regenerate.sh _write
	else
		heavy-lock --host local specarch:regenerate -- sh scripts/regenerate.sh _write
	fi
else
	HEAVY_LOCK_KIND=go_build_test heavy-lock --artifact "$back" specarch:regenerate -- sh scripts/regenerate.sh _write
fi
status=$?
if [ $status != 0 ]; then
	echo "regenerate: FAILED: the work exited $status (heavy-lock job specarch:regenerate); the lines above name the host it ran on and the step" >&2
	exit 1
fi
if [ -f "$back/files.tar" ]; then
	step "write the files sent back into this checkout" unpack
fi

echo "regenerate: done; run sh scripts/landing-check.sh next"
