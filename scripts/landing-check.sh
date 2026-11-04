#!/bin/sh
# Every check a change must pass before it lands on main, run from the
# repository root:
#
#	sh scripts/landing-check.sh          both parts
#	sh scripts/landing-check.sh go       the Go build, its tests and every
#	                                     generated file current
#	sh scripts/landing-check.sh swift    the Swift build, its tests, and its
#	                                     validate and model output the same
#	                                     as the Go build's (needs Go too)
#
# CI runs the go part in its check job and the swift part in its swift job,
# so this script and CI run the same checks. It stops at the first failure
# and names the step. The specarch-gen-* plug-ins are built into .bin, which
# is not tracked; scratch files go into a folder mktemp makes.
#
# Where heavy-lock is on the PATH, each part runs as one heavy-lock job of
# its own, so run the script bare, not under heavy-lock. heavy-lock sends
# each to a Linux runner when one is healthy, the go part as a job of kind
# go_build_test and the swift part as one of kind swift_linux_static (the
# script names the kind in HEAVY_LOCK_KIND, since heavy-lock reads it from
# the command, and sh is not one it knows). A runner gets the committed
# files with the staged changes, no .git, and runs every step of the part
# there, the swift part building its own specarch to compare with. A part
# runs on the Mac instead, with the reason printed, when no runner can take
# it (heavy-lock names why), when files have changes that are not staged or
# are untracked (the runner would check another tree than this one), when
# the script itself runs inside a heavy-lock job (a heavy-lock inside a job
# runs its command in place), and where there is no heavy-lock (CI). No
# step needs macOS: CI's swift job still runs the swift part on macOS.
set -u

parts=${1:-all}
case $parts in
all | go | swift | _go | _swift) ;;
*)
	echo "landing-check: unknown part $parts; give go, swift or nothing for both" >&2
	exit 2
	;;
esac

if [ ! -f go.mod ] || [ ! -d swift ]; then
	echo "landing-check: run it from the repository root, not $(pwd)" >&2
	exit 2
fi

tmp=$(mktemp -d) || exit 1
trap 'rm -rf "$tmp"' EXIT
bin=$(pwd)/.bin
# The go part leaves the name of the host it ran on here; on a runner it is
# not sent back, so the file says the Go build in .bin is this host's own.
built_here=$bin/.go-part-host

# step runs a command and stops the script when it fails, naming the step and
# the host it ran on.
step() {
	name=$1
	shift
	echo "landing-check: $name"
	if ! "$@"; then
		echo "landing-check: FAILED at: $name (on $(uname -n))" >&2
		exit 1
	fi
}

build_specarch() {
	mkdir -p "$bin" && go build -o "$bin/specarch" ./cmd/specarch
}

build_tools() {
	build_specarch || return 1
	for generator in sql seed sqlc openapi ui ui-typescript ui-vue-htm ui-swiftui bpmn register tests-go deploy; do
		go build -o "$bin/specarch-gen-$generator" ./cmd/specarch-gen-$generator || return 1
	done
}

# documents_current checks every generated document. The first loop is the
# one CONTRIBUTING.md gives, over every specification at once;
# examples/lending-desk configures only the documents an owner reads while
# questions are open, so that loop warns for it at testplan, deployment and
# commissioning and goes on (ADR-073); examples/release-watcher adds its
# operations guide to the techspec the first loop writes.
documents_current() {
	for target in intent techspec requirements testplan traceability deployment commissioning handover; do
		"$bin/specarch" document $target --check spec examples || return 1
	done
	for target in techspec requirements testplan traceability deployment commissioning handover manual operations questions problems changes releases communications timeline testruns progress html; do
		"$bin/specarch" document $target --check spec examples/library-lending || return 1
	done
	for target in techspec requirements traceability questions problems html; do
		"$bin/specarch" document $target --check examples/lending-desk || return 1
	done
	"$bin/specarch" document operations --check examples/release-watcher || return 1
}

# generated_current checks the library lending example's generated code and
# data, and the reading room's screens and tests, generated across the
# estate since its picker reads the catalogue service's list and its
# requestBook calls the catalogue's getBook. Neither is approved, hence
# --unapproved.
generated_current() {
	for target in sql seed sqlc openapi ui bpmn deploy; do
		PATH="$bin:$PATH" "$bin/specarch" generate $target --check --unapproved examples/library-lending || return 1
	done
	PATH="$bin:$PATH" "$bin/specarch" generate ui --check --unapproved --estate examples/estate/specarch-estate.yaml examples/estate/reading-room || return 1
	PATH="$bin:$PATH" "$bin/specarch" generate tests --check --unapproved --estate examples/estate/specarch-estate.yaml examples/estate/reading-room || return 1
}

# validate_same compares the Swift build's validate output with the Go
# build's, byte for byte.
validate_same() {
	"$bin/specarch" validate spec examples >"$tmp/go-validate.txt" 2>&1 || return 1
	swift/.build/release/specarch validate spec examples >"$tmp/swift-validate.txt" 2>&1 || return 1
	cmp "$tmp/go-validate.txt" "$tmp/swift-validate.txt" || {
		diff "$tmp/go-validate.txt" "$tmp/swift-validate.txt"
		return 1
	}
}

# model_same compares the model both builds print, which an editor loads
# from either (ADR-138).
model_same() {
	for paths in "spec examples" examples/estate/specarch-estate.yaml; do
		# $paths is split on purpose: "spec examples" is two paths.
		"$bin/specarch" model $paths >"$tmp/go-model.json" || return 1
		swift/.build/release/specarch model $paths >"$tmp/swift-model.json" || return 1
		cmp "$tmp/go-model.json" "$tmp/swift-model.json" || return 1
	done
}

# go_steps is the go part's own work, on whichever host runs it.
go_steps() {
	echo "landing-check: the go part runs on $(uname -n) ($(uname -s) $(uname -m))"
	step "go build" go build ./...
	step "go vet" go vet ./...
	step "go test, including the conformance suite" go test ./...
	step "the fragment schemas and hint lines are current (go run ./tools/fragment-schemas -check)" go run ./tools/fragment-schemas -check
	step "build specarch and the specarch-gen-* plug-ins into .bin" build_tools
	step "validate SpecArch's own specification and every example" "$bin/specarch" validate spec examples
	step "generated documents are current (scripts/regenerate.sh writes them)" documents_current
	step "generated code and data are current (scripts/regenerate.sh writes them)" generated_current
	uname -n >"$built_here"
}

# swift_steps is the swift part's own work, on whichever host runs it. With
# $1 "built" the go part has just built specarch in this checkout and it is
# used as it is; otherwise it is built now.
swift_steps() {
	echo "landing-check: the swift part runs on $(uname -n) ($(uname -s) $(uname -m))"
	step "swift build" swift build -c release --package-path swift
	step "swift test, including the conformance suite" swift test --package-path swift
	if [ "${1:-}" != built ] || [ ! -x "$bin/specarch" ]; then
		step "build specarch into .bin" build_specarch
	fi
	step "the Swift build's validate output is the Go build's" validate_same
	step "both builds print the same model of the specification, every example and the estate" model_same
}

# not_to_runner prints why the go part cannot go to a runner, or nothing
# when it can.
not_to_runner() {
	if [ -n "${HEAVY_LOCK_HELD:-}" ]; then
		echo "the script runs inside the heavy-lock job ${HEAVY_LOCK_HELD}, and a heavy-lock inside a job runs its command in place; run sh scripts/landing-check.sh bare to use a runner"
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

# run_part runs the internal part $2 (with $3) as the heavy-lock job $1,
# of kind $4: on a runner when heavy-lock can send it there, otherwise on
# this Mac with the reason printed.
run_part() {
	label=$1 part=$2 arg=$3 kind=$4
	why=$(not_to_runner)
	if [ -z "$why" ]; then
		HEAVY_LOCK_KIND=$kind heavy-lock "$label" -- sh scripts/landing-check.sh "$part" $arg
	elif [ -n "${HEAVY_LOCK_HELD:-}" ] || ! command -v heavy-lock >/dev/null 2>&1; then
		echo "landing-check: $label runs here, on $(uname -n): $why"
		sh scripts/landing-check.sh "$part" $arg
	else
		echo "landing-check: $label runs on this Mac ($(uname -n)): $why"
		heavy-lock --host local "$label" -- sh scripts/landing-check.sh "$part" $arg
	fi
	status=$?
	if [ $status != 0 ]; then
		echo "landing-check: FAILED: $label exited $status; the lines above name the host it ran on and the step" >&2
		exit 1
	fi
}

case $parts in
_go)
	go_steps
	exit 0
	;;
_swift)
	swift_steps "${2:-}"
	exit 0
	;;
esac

from=""
if [ "$parts" != swift ]; then
	rm -f "$built_here"
	run_part specarch:landing-go _go "" go_build_test
	[ -f "$built_here" ] && from=built
fi

if [ "$parts" != go ]; then
	run_part specarch:landing-swift _swift "$from" swift_linux_static
fi

echo "landing-check: all passed ($parts)"
