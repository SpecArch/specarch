// Package readers holds the readers of code-facts dumps that run on a
// language's own toolchain, Swift and Dart, as source with the lock files
// that pin every dependency. specarch facts writes them into a per-user
// cache and builds them there (ADR-116); the JavaScript reader is in the
// binary itself (internal/facts).
package readers

import "embed"

// Files are the readers' sources and lock files, by their path under
// readers/.
//
//go:embed swift/Package.swift swift/Package.resolved swift/Sources
//go:embed dart/pubspec.yaml dart/pubspec.lock dart/bin
var Files embed.FS
