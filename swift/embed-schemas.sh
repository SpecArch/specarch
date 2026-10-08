#!/bin/sh
# Writes Sources/SpecArchKit/Schemas.swift from ../schema and ../idioms, so
# the binary carries the same schemas editors use and the idioms the Go
# build ships. A test fails when they differ.
set -e
cd "$(dirname "$0")"
out=Sources/SpecArchKit/Schemas.swift
{
  echo '// Written by embed-schemas.sh from ../schema and ../idioms. Do not edit; run the script.'
  echo
  echo 'let designSchemaJSON = #"""'
  cat ../schema/specarch-design-0.1.schema.json
  echo '"""#'
  echo
  echo 'let implementationSchemaJSON = #"""'
  cat ../schema/specarch-implementation-0.1.schema.json
  echo '"""#'
  echo
  echo 'let recordSchemaJSON = #"""'
  cat ../schema/specarch-record-0.1.schema.json
  echo '"""#'
  echo
  echo 'let idiomSchemaJSON = #"""'
  cat ../schema/specarch-idiom-0.1.schema.json
  echo '"""#'
  echo
  echo '/// The shipped idioms, by their path under idioms/, as idioms/embed.go embeds them.'
  echo 'let shippedIdiomFiles: [(path: String, text: String)] = ['
  for f in $(cd .. && ls idioms/*/*.specarch-idiom.yaml | LC_ALL=C sort); do
    echo "    (\"$f\", #\"\"\""
    cat "../$f"
    echo '"""#),'
  done
  echo ']'
} > "$out"
