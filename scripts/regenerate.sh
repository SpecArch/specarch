#!/bin/sh
# Writes every generated file landing-check.sh checks, from the repository
# root: the fragment schemas and hint lines, the documents of SpecArch's own
# specification, of examples/library-lending, of examples/lending-desk and
# of examples/release-watcher,
# the library lending example's generated code and data, and the reading
# room's screens, generated across the estate in examples/estate. To settle a
# conflict in one of these files, take either side, then run this script on
# the merged tree and run scripts/landing-check.sh.
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
		echo "regenerate: FAILED at: $name" >&2
		exit 1
	fi
}

build_tools() {
	mkdir -p "$bin" || return 1
	go build -o "$bin/specarch" ./cmd/specarch || return 1
	for generator in sql sqlc openapi ui ui-typescript ui-swiftui bpmn; do
		go build -o "$bin/specarch-gen-$generator" ./cmd/specarch-gen-$generator || return 1
	done
}

documents() {
	for target in techspec requirements testplan traceability deployment commissioning handover; do
		"$bin/specarch" document $target spec examples || return 1
	done
	for target in techspec requirements testplan traceability deployment commissioning handover manual operations questions problems changes releases html; do
		"$bin/specarch" document $target spec examples/library-lending || return 1
	done
	for target in techspec requirements traceability questions problems html; do
		"$bin/specarch" document $target examples/lending-desk || return 1
	done
	"$bin/specarch" document operations examples/release-watcher || return 1
}

generated() {
	for target in sql sqlc openapi ui bpmn; do
		PATH="$bin:$PATH" "$bin/specarch" generate $target --unapproved examples/library-lending || return 1
	done
	PATH="$bin:$PATH" "$bin/specarch" generate ui --unapproved --estate examples/estate/specarch-estate.yaml examples/estate/reading-room || return 1
}

step "the fragment schemas and hint lines (go run ./tools/fragment-schemas)" go run ./tools/fragment-schemas
step "build specarch and the specarch-gen-* plug-ins into .bin" build_tools
step "the generated documents" documents
step "the library lending example's generated code and data, and the reading room's screens" generated

echo "regenerate: done; run sh scripts/landing-check.sh next"
