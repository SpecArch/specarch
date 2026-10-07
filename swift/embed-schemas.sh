#!/bin/sh
# Writes Sources/SpecArchKit/Schemas.swift from ../schema, so the binary
# carries the same schemas editors use. A test fails when they differ.
set -e
cd "$(dirname "$0")"
out=Sources/SpecArchKit/Schemas.swift
{
  echo '// Written by embed-schemas.sh from ../schema. Do not edit; run the script.'
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
} > "$out"
