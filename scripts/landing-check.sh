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
set -u

parts=${1:-all}
case $parts in
all | go | swift) ;;
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

# step runs a command and stops the script when it fails, naming the step.
step() {
	name=$1
	shift
	echo "landing-check: $name"
	if ! "$@"; then
		echo "landing-check: FAILED at: $name" >&2
		exit 1
	fi
}

build_specarch() {
	mkdir -p "$bin" && go build -o "$bin/specarch" ./cmd/specarch
}

build_tools() {
	build_specarch || return 1
	for generator in sql sqlc openapi ui ui-typescript ui-swiftui bpmn; do
		go build -o "$bin/specarch-gen-$generator" ./cmd/specarch-gen-$generator || return 1
	done
}

# documents_current checks every generated document. The first loop is the
# one CONTRIBUTING.md gives, over every specification at once;
# examples/lending-desk configures only the documents an owner reads while
# questions are open, so that loop warns for it at testplan, deployment and
# commissioning and goes on (ADR-073).
documents_current() {
	for target in techspec requirements testplan traceability deployment commissioning handover; do
		"$bin/specarch" document $target --check spec examples || return 1
	done
	for target in techspec requirements testplan traceability deployment commissioning handover manual operations questions problems changes releases html; do
		"$bin/specarch" document $target --check spec examples/library-lending || return 1
	done
	for target in techspec requirements traceability questions problems html; do
		"$bin/specarch" document $target --check examples/lending-desk || return 1
	done
}

# generated_current checks the library lending example's generated code and
# data, and the reading room's screens, generated across the estate since
# its picker reads the catalogue service's list. Neither is approved, hence
# --unapproved.
generated_current() {
	for target in sql sqlc openapi ui bpmn; do
		PATH="$bin:$PATH" "$bin/specarch" generate $target --check --unapproved examples/library-lending || return 1
	done
	PATH="$bin:$PATH" "$bin/specarch" generate ui --check --unapproved --estate examples/estate/specarch-estate.yaml examples/estate/reading-room || return 1
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

if [ "$parts" != swift ]; then
	step "go build" go build ./...
	step "go vet" go vet ./...
	step "go test, including the conformance suite" go test ./...
	step "the fragment schemas and hint lines are current (go run ./tools/fragment-schemas -check)" go run ./tools/fragment-schemas -check
	step "build specarch and the specarch-gen-* plug-ins into .bin" build_tools
	step "validate SpecArch's own specification and every example" "$bin/specarch" validate spec examples
	step "generated documents are current (scripts/regenerate.sh writes them)" documents_current
	step "generated code and data are current (scripts/regenerate.sh writes them)" generated_current
fi

if [ "$parts" != go ]; then
	step "swift build" swift build -c release --package-path swift
	step "swift test, including the conformance suite" swift test --package-path swift
	if [ "$parts" = swift ]; then
		step "build specarch into .bin" build_specarch
	fi
	step "the Swift build's validate output is the Go build's" validate_same
	step "both builds print the same model of the specification, every example and the estate" model_same
fi

echo "landing-check: all passed ($parts)"
