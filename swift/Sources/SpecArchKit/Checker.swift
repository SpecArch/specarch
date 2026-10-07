import Foundation

/// The kind of a SpecArch file, read from its name.
public enum Kind {
    case none, design, implementation
}

public let designSuffix = ".specarch-design.yaml"
public let implementationSuffix = ".specarch-implementation.yaml"

/// Tells the kind of a file from its name.
public func kindOf(_ name: String) -> Kind {
    if name.hasSuffix(designSuffix) { return .design }
    if name.hasSuffix(implementationSuffix) { return .implementation }
    return .none
}

/// Reads a file the caller did not pass in: the design file an
/// implementation file names.
public typealias ReadFile = (String) throws -> Data

/// Runs every check on one file and returns its diagnostics, sorted. The
/// path is used as given in every diagnostic.
public func check(path: String, data: Data, read: ReadFile) -> [Diagnostic] {
    let c = Checker(file: path)
    c.run(data, read)
    var ds = withoutEchoes(c.diags)
    sortDiagnostics(&ds)
    return ds
}

/// Drops a diagnostic that only repeats a schema error: one at the same
/// path, or inside a key the schema refused.
func withoutEchoes(_ ds: [Diagnostic]) -> [Diagnostic] {
    var schemaAt = Set<String>()
    var refused: [String] = []
    for d in ds where d.rule == .schema {
        schemaAt.insert(d.path)
        if d.message.contains("is not a key this object can have") {
            refused.append(d.path + "/")
        }
    }
    return ds.filter { d in
        guard d.rule != .schema, d.severity == .error else { return true }
        if schemaAt.contains(d.path) { return false }
        return !refused.contains { (d.path + "/").hasPrefix($0) }
    }
}

final class Checker {
    let file: String
    var root: YNode!
    var diags: [Diagnostic] = []

    init(file: String) { self.file = file }

    func add(_ n: YNode?, _ path: String, _ rule: Rule, _ message: String) {
        addLine(n?.line ?? 1, path, rule, message)
    }

    func addLine(_ line: Int, _ path: String, _ rule: Rule, _ message: String) {
        diags.append(Diagnostic(file: file, line: max(line, 1), severity: .error, path: path.isEmpty ? "/" : path, rule: rule, message: message))
    }

    func warn(_ n: YNode?, _ path: String, _ rule: Rule, _ message: String) {
        add(n, path, rule, message)
        diags[diags.count - 1].severity = .warning
    }

    func run(_ data: Data, _ read: ReadFile) {
        let kind = kindOf(file)
        if kind == .none {
            addLine(1, "/", .fileKind, "the file name ends in neither \(designSuffix) nor \(implementationSuffix); rename the file so its kind is clear")
            return
        }
        let doc = parseYAML(String(decoding: data, as: UTF8.self))
        for p in doc.problems {
            addLine(p.line, p.path, Rule(rawValue: p.rule)!, p.message)
        }
        guard let root = doc.root else { return }
        self.root = root
        if !kindMatchesRoot(kind) { return }
        checkSchema(kind, doc.value)
        checkChangeLog()
        switch kind {
        case .design:
            checkBoundaryDesign()
            checkDesign(Design(root))
        case .implementation:
            checkBoundaryImplementation()
            checkImplementation(read)
        case .none:
            break
        }
    }

    /// Refuses a file whose name and root key disagree, since every later
    /// check would only repeat that one mistake.
    func kindMatchesRoot(_ kind: Kind) -> Bool {
        guard root.kind == .mapping else { return true }
        let hasDesign = root.key("specarch") != nil
        let hasImpl = root.key("specarchImplementation") != nil
        if kind == .design && hasImpl && !hasDesign {
            add(root.key("specarchImplementation"), "/specarchImplementation", .fileKind,
                "the file is named as a design file but starts with specarchImplementation; rename it to end in \(implementationSuffix), or start it with specarch")
            return false
        }
        if kind == .implementation && hasDesign && !hasImpl {
            add(root.key("specarch"), "/specarch", .fileKind,
                "the file is named as an implementation file but starts with specarch; rename it to end in \(designSuffix), or start it with specarchImplementation")
            return false
        }
        return true
    }
}
