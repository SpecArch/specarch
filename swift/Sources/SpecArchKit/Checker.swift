import Foundation

/// The kind of a SpecArch file, read from its name.
public enum Kind {
    case none, design, implementation, record, idiom
}

public let implementationSuffix = ".specarch-implementation.yaml"

/// Tells the kind of a file from its name: the root file of a
/// specification, an implementation file, or neither.
public func kindOf(_ name: String) -> Kind {
    if name == rootFile || name.hasSuffix("/" + rootFile) { return .design }
    if name.hasSuffix(implementationSuffix) { return .implementation }
    return .none
}

/// Reads the specification rooted at a folder: the one an implementation
/// file names under implements.
typealias Loader = (String) -> Spec

/// Runs every check on a specification and on the implementation files
/// inside it, and returns the diagnostics to report, sorted. What an open
/// question covers is left out; checkSpecCovered returns it too.
func checkSpec(_ s: Spec) -> [Diagnostic] {
    checkSpecCovered(s).kept
}

/// Runs every check and returns the diagnostics to report and, apart, the
/// ones an open must question covers (see covered), both sorted.
func checkSpecCovered(_ s: Spec) -> (kept: [Diagnostic], covered: [Diagnostic]) {
    var all = checkSpecAll(s)
    let placer = Placer(spec: s)
    guard let root = s.root else {
        placer.place(&all)
        sortDiagnostics(&all)
        return (all, [])
    }
    var (kept, covered) = coveredByQuestions(all, root)
    placer.place(&kept)
    placer.place(&covered)
    sortDiagnostics(&kept)
    sortDiagnostics(&covered)
    return (kept, covered)
}

private func checkSpecAll(_ s: Spec) -> [Diagnostic] {
    let c = Checker(file: s.rootPath, files: s.files)
    for p in s.problems {
        c.addFile(p.file, p.line, p.path, Rule(rawValue: p.rule)!, p.message)
    }
    var out: [Diagnostic] = []
    var design: Design?
    if let root = s.root {
        c.root = root
        if c.rootIsSpec() {
            c.checkSchema(.design, s.value)
            c.checkChangeLog()
            c.checkBoundaryDesign()
            let d = Design(root)
            d.spec = s
            c.checkDesign(d)
            design = d
        }
    }
    out += withoutEchoes(c.diags)
    for impl in s.implementations {
        let ic = Checker(file: impl.path)
        ic.runImplementation(impl.data, s, nil)
        out += withoutEchoes(ic.diags)
    }
    if let design {
        out += checkRecords(s, design)
    }
    sortDiagnostics(&out)
    return out
}

/// Checks one implementation file given on its own. The specification it
/// implements is loaded through load.
func checkImplementationFile(path: String, data: Data, load: @escaping Loader) -> [Diagnostic] {
    let c = Checker(file: path)
    c.runImplementation(data, nil, load)
    var ds = withoutEchoes(c.diags)
    Placer(dir: dirPath(path)).place(&ds)
    sortDiagnostics(&ds)
    return ds
}

/// Reports a file given by name that is neither a root file nor an
/// implementation file.
func checkNamed(_ path: String) -> [Diagnostic] {
    let c = Checker(file: path)
    if let root = rootOf(path) {
        c.addLine(1, "/", .fileKind, "this file is part of the specification whose root file is \(joinPath(root, rootFile)); run specarch validate on that specification's folder")
    } else {
        c.addLine(1, "/", .fileKind, "the file is neither \(rootFile) nor an implementation file (*\(implementationSuffix)); name a specification's folder or root file")
    }
    var ds = c.diags
    Placer(dir: dirPath(path)).place(&ds)
    return ds
}

/// Drops a diagnostic that only repeats a schema error: one at the same
/// path, or inside a key the schema refused. A missing key is reported at
/// the object that lacks it, so it says nothing wrong about what the object
/// does hold; it makes no echo of the other errors there.
func withoutEchoes(_ ds: [Diagnostic]) -> [Diagnostic] {
    var schemaAt = Set<String>()
    var refused: [String] = []
    for d in ds where d.rule == .schema {
        if !d.message.hasSuffix(" is missing; add it here") {
            schemaAt.insert(d.path)
        }
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
    let file: String                       // the file diagnostics name when a node is not known
    let files: [ObjectIdentifier: String]  // the file of each node of a merged specification
    var root: YNode!
    var diags: [Diagnostic] = []
    let text = Placer(dir: "") // the lines of the files read, for an expression's column

    init(file: String, files: [ObjectIdentifier: String] = [:]) {
        self.file = file
        self.files = files
    }

    func fileOf(_ n: YNode?) -> String {
        guard let n else { return file }
        return files[ObjectIdentifier(n)] ?? file
    }

    func add(_ n: YNode?, _ path: String, _ rule: Rule, _ message: String) {
        addFile(fileOf(n), n?.line ?? 1, path, rule, message)
    }

    func addLine(_ line: Int, _ path: String, _ rule: Rule, _ message: String) {
        addFile(file, line, path, rule, message)
    }

    func addFile(_ file: String, _ line: Int, _ path: String, _ rule: Rule, _ message: String) {
        diags.append(Diagnostic(file: file, line: max(line, 1), severity: .error, path: path.isEmpty ? "/" : path, rule: rule, message: message))
    }

    func warn(_ n: YNode?, _ path: String, _ rule: Rule, _ message: String) {
        add(n, path, rule, message)
        diags[diags.count - 1].severity = .warning
    }

    /// Refuses a root file that starts with specarchImplementation, since
    /// every later check would only repeat that one mistake.
    func rootIsSpec() -> Bool {
        guard root.kind == .mapping else { return true } // the schema reports it
        if root.key("specarchImplementation") != nil && root.key("specarch") == nil {
            add(root.key("specarchImplementation"), "/specarchImplementation", .fileKind,
                "\(rootFile) is a specification's root file but starts with specarchImplementation; an implementation file is named <name>.<stack>\(implementationSuffix)")
            return false
        }
        return true
    }

    /// Checks an implementation file. With s given, the file is part of that
    /// specification; otherwise load reads the one it names.
    func runImplementation(_ data: Data, _ s: Spec?, _ load: Loader?) {
        let doc = parseYAML(String(decoding: data, as: UTF8.self))
        for p in doc.problems {
            addLine(p.line, p.path, Rule(rawValue: p.rule)!, p.message)
        }
        guard let root = doc.root else { return }
        self.root = root
        if root.kind == .mapping && root.key("specarch") != nil && root.key("specarchImplementation") == nil {
            add(root.key("specarch"), "/specarch", .fileKind,
                "the file is named as an implementation file but starts with specarch; a specification's root file is named \(rootFile)")
            return
        }
        checkSchema(.implementation, doc.value)
        checkChangeLog()
        checkBoundaryImplementation()
        checkImplementation(s, load)
    }
}
