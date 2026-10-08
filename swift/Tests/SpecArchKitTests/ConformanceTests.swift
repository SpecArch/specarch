import Foundation
import Testing
import Yams
@testable import SpecArchKit

/// The repository root, found from this file's place in it.
private let repository = URL(fileURLWithPath: #filePath)
    .deletingLastPathComponent().deletingLastPathComponent().deletingLastPathComponent().deletingLastPathComponent()
/// The conformance suite is the tests stage of SpecArch's own specification.
private let conformance = repository.appendingPathComponent("spec/tests")

/// The commands this implementation offers. Documents and generators are built in Go only.
private let offered: Set<String> = ["validate", "version"]

/// Every file under dir, keyed by its slash path, leaving out the top-level
/// names given.
private func readTree(_ dir: URL, skipping: Set<String> = []) -> [String: Data] {
    var out: [String: Data] = [:]
    guard let walker = FileManager.default.enumerator(atPath: dir.path) else { return out }
    for case let rel as String in walker {
        if let top = rel.split(separator: "/").first, skipping.contains(String(top)) { continue }
        var isDir: ObjCBool = false
        let full = dir.appendingPathComponent(rel)
        if FileManager.default.fileExists(atPath: full.path, isDirectory: &isDir), !isDir.boolValue {
            out[rel] = FileManager.default.contents(atPath: full.path)
        }
    }
    return out
}

private struct Case {
    let name: String
    let arguments: [String]
    let exitStatus: Int32
    let standardOutput: String
}

private func cases() throws -> [Case] {
    let names = try FileManager.default.contentsOfDirectory(atPath: conformance.path).sorted(by: byteLess)
    return try names.compactMap { name in
        let file = conformance.appendingPathComponent(name).appendingPathComponent("case.yaml")
        guard let text = try? String(contentsOf: file, encoding: .utf8) else { return nil }
        guard let doc = try Yams.load(yaml: text) as? [String: Any] else { return nil }
        return Case(name: name,
                    arguments: (doc["arguments"] as? [Any] ?? []).map { "\($0)" },
                    exitStatus: Int32(doc["exitStatus"] as? Int ?? -1),
                    standardOutput: doc["standardOutput"] as? String ?? "")
    }
}

/// Runs every case of the language-neutral conformance suite for the
/// commands this implementation offers, in a copy of the case's folder, and
/// compares the exit status, the standard output and the files written.
@Test func conformanceSuite() throws {
    let all = try cases()
    #expect(!all.isEmpty)
    for c in all where offered.contains(c.arguments.first ?? "") {
        let src = conformance.appendingPathComponent(c.name)
        let inputs = readTree(src, skipping: ["case.yaml", "test.yaml", "expected"])
        let expected = readTree(src.appendingPathComponent("expected"))
        let work = FileManager.default.temporaryDirectory.appendingPathComponent("specarch-\(UUID().uuidString)")
        for (name, data) in inputs {
            let p = work.appendingPathComponent(name)
            try FileManager.default.createDirectory(at: p.deletingLastPathComponent(), withIntermediateDirectories: true)
            try data.write(to: p)
        }
        try FileManager.default.createDirectory(at: work, withIntermediateDirectories: true)
        let previous = FileManager.default.currentDirectoryPath
        FileManager.default.changeCurrentDirectoryPath(work.path)
        let stdout = StringSink(), stderr = StringSink()
        let status = run(c.arguments, stdout: stdout, stderr: stderr)
        FileManager.default.changeCurrentDirectoryPath(previous)
        #expect(status == c.exitStatus, "\(c.name): exit status \(status), want \(c.exitStatus)\n\(stderr.text)")
        #expect(stdout.text == c.standardOutput, "\(c.name): standard output differs\ngot:\n\(stdout.text)\nwant:\n\(c.standardOutput)")
        var want = inputs
        for (k, v) in expected { want[k] = v }
        let after = readTree(work)
        #expect(after == want, "\(c.name): the files after the run differ from what the case expects")
        try? FileManager.default.removeItem(at: work)
    }
}

/// SpecArch's own specification, merged.
private func ownSpecification() throws -> YNode {
    let s = Spec(dir: repository.appendingPathComponent("spec").path)
    return try #require(s.root, "spec/ does not load")
}

/// Every test of an offered command has a case.yaml, unless it is marked
/// not applicable.
@Test func everyDesignTestHasACase() throws {
    let tests = try ownSpecification().child("tests")
    for p in tests?.pairs ?? [] {
        guard let command = p.value.child("command")?.str, offered.contains(command), p.value.child("notApplicable") == nil else { continue }
        let file = conformance.appendingPathComponent(p.key.value).appendingPathComponent("case.yaml")
        #expect(FileManager.default.fileExists(atPath: file.path), "spec/tests/\(p.key.value) is a test of \(command) but has no case.yaml")
    }
}

/// The rules are exactly the values of the specification's Rule enum.
@Test func rulesMatchDesign() throws {
    let rule = try ownSpecification().child("enums")?.child("Rule")?.child("enum")?.items.map(\.value) ?? []
    #expect(!rule.isEmpty)
    #expect(rule.sorted() == Rule.allCases.map(\.rawValue).sorted())
}

/// The embedded schemas are the files in schema/.
@Test func embeddedSchemasAreCurrent() throws {
    let design = try String(contentsOf: repository.appendingPathComponent("schema/specarch-design-0.1.schema.json"), encoding: .utf8)
    let implementation = try String(contentsOf: repository.appendingPathComponent("schema/specarch-implementation-0.1.schema.json"), encoding: .utf8)
    #expect(designSchemaJSON + "\n" == design, "run swift/embed-schemas.sh")
    #expect(implementationSchemaJSON + "\n" == implementation, "run swift/embed-schemas.sh")
    let record = try String(contentsOf: repository.appendingPathComponent("schema/specarch-record-0.1.schema.json"), encoding: .utf8)
    #expect(recordSchemaJSON + "\n" == record, "run swift/embed-schemas.sh")
    let idiom = try String(contentsOf: repository.appendingPathComponent("schema/specarch-idiom-0.1.schema.json"), encoding: .utf8)
    #expect(idiomSchemaJSON + "\n" == idiom, "run swift/embed-schemas.sh")
}

/// The embedded idioms are the files in idioms/, every one of them.
@Test func embeddedIdiomsAreCurrent() throws {
    let folder = repository.appendingPathComponent("idioms")
    var paths: [String] = []
    for concern in try FileManager.default.contentsOfDirectory(atPath: folder.path) {
        let dir = folder.appendingPathComponent(concern)
        guard let names = try? FileManager.default.contentsOfDirectory(atPath: dir.path) else { continue }
        for n in names where n.hasSuffix(".specarch-idiom.yaml") { paths.append("idioms/\(concern)/\(n)") }
    }
    #expect(paths.sorted() == shippedIdiomFiles.map(\.path).sorted(), "run swift/embed-schemas.sh")
    for f in shippedIdiomFiles {
        let text = try String(contentsOf: repository.appendingPathComponent(f.path), encoding: .utf8)
        #expect(f.text + "\n" == text, "run swift/embed-schemas.sh")
    }
}

/// The evaluator knows every keyword the schemas use.
@Test func schemasLoad() throws {
    _ = try SchemaEvaluator(json: designSchemaJSON)
    _ = try SchemaEvaluator(json: implementationSchemaJSON)
    _ = try SchemaEvaluator(json: recordSchemaJSON)
    _ = try SchemaEvaluator(json: idiomSchemaJSON)
}
