import Foundation

/// The program version.
public let programVersion = "0.3.0"

let usage = """
usage:
  specarch validate <folder or file>...    check specifications and implementation files
  specarch version                          print the program version

A specification is a folder holding specarch.yaml. A folder given here is
searched for specifications and for *.specarch-implementation.yaml files
outside one. This build has no gaps, document, approve, generate, extract,
diff, derive or idioms verbs; the Go build of specarch has them.

"""

/// Somewhere to write text: standard output, standard error, or a buffer.
public protocol TextSink: AnyObject {
    func write(_ s: String)
}

public final class StringSink: TextSink {
    public var text = ""
    public init() {}
    public func write(_ s: String) { text += s }
}

public final class FileSink: TextSink {
    let handle: FileHandle
    public init(_ handle: FileHandle) { self.handle = handle }
    public func write(_ s: String) { handle.write(Data(s.utf8)) }
}

/// Runs the program with its arguments and returns the exit status.
public func run(_ args: [String], stdout: TextSink, stderr: TextSink) -> Int32 {
    guard let command = args.first else {
        stderr.write(usage)
        return 2
    }
    switch command {
    case "validate":
        return runValidate(Array(args.dropFirst()), stdout, stderr)
    case "version":
        if args.count > 1 {
            stderr.write("specarch version takes no arguments\n\n\(usage)")
            return 2
        }
        stdout.write("specarch \(programVersion)\nspecifications: meta-model 0.1\nimplementation files: meta-model 0.1\n")
        return 0
    case "document", "generate", "extract", "gaps", "approve", "diff", "derive", "idioms":
        stderr.write("specarch \(command): this build has no \(command) verb; the Go build of specarch has it\n")
        return 2
    case "help", "-h", "--help":
        stdout.write(usage)
        return 0
    default:
        stderr.write("specarch has no command \(quote(command))\n\n\(usage)")
        return 2
    }
}

/// The validate command and its exitStatus algorithm: 2 for a usage or read
/// error, 1 when any diagnostic is an error, 0 otherwise.
func runValidate(_ argsIn: [String], _ stdout: TextSink, _ stderr: TextSink) -> Int32 {
    var args = argsIn
    if args.first == "--" { args.removeFirst() }
    if args.isEmpty {
        stderr.write("specarch validate needs at least one folder or file\n\n\(usage)")
        return 2
    }
    for a in args where a.count > 1 && a.hasPrefix("-") {
        stderr.write("specarch validate has no option \(a); to check a path whose name starts with -, write -- before it\n\n\(usage)")
        return 2
    }
    let (inputs, collectError) = collect(args, stderr)
    var ioError = collectError
    var all: [Diagnostic] = []
    var open = 0
    for input in inputs {
        switch input {
        case .root(let dir):
            let s = Spec(dir: dir)
            all += checkSpec(s)
            open += openQuestions(s.root)
        case .implementation(let path):
            guard let data = try? readFile(path) else {
                stderr.write("specarch: cannot read \(path)\n")
                ioError = true
                continue
            }
            all += checkImplementationFile(path: path, data: data, load: { Spec(dir: $0) })
        case .other(let path):
            all += checkNamed(path)
        }
    }
    sortDiagnostics(&all)
    for d in all { stdout.write(d.description + "\n") }
    let errors = errorCount(all)
    let questions = open > 0 ? ", " + plural(open, "open question") : ""
    stderr.write("specarch: \(plural(inputs.count, "input")) checked: \(plural(errors, "error")), \(plural(all.count - errors, "warning"))\(questions)\n")
    if ioError { return 2 }
    return errors > 0 ? 1 : 0
}

/// Reads a file, with the error the C library would name.
public func readFile(_ path: String) throws -> Data {
    var isDir: ObjCBool = false
    guard FileManager.default.fileExists(atPath: path, isDirectory: &isDir) else {
        throw NSError(domain: NSPOSIXErrorDomain, code: Int(ENOENT), userInfo: [NSUnderlyingErrorKey: NSError(domain: NSPOSIXErrorDomain, code: Int(ENOENT))])
    }
    return try Data(contentsOf: URL(fileURLWithPath: path))
}

/// One thing to check: a specification (its root folder), an
/// implementation file outside any specification, or a file given by name
/// that is neither.
enum Input {
    case root(String)
    case implementation(String)
    case other(String)
}

/// Turns the arguments into inputs: a folder is searched for
/// specifications and standalone implementation files; a file is taken as
/// given.
func collect(_ args: [String], _ stderr: TextSink) -> ([Input], Bool) {
    var inputs: [Input] = []
    var seen = Set<String>()
    var ioError = false
    func add(_ key: String, _ input: Input) {
        if !seen.contains(key) {
            seen.insert(key)
            inputs.append(input)
        }
    }
    for a in args {
        var isDir: ObjCBool = false
        guard FileManager.default.fileExists(atPath: a, isDirectory: &isDir) else {
            stderr.write("specarch: cannot read \(a): no such file or directory\n")
            ioError = true
            continue
        }
        if !isDir.boolValue {
            switch kindOf(a) {
            case .design: add("root:" + cleanPath(dirPath(a)), .root(dirPath(a)))
            case .implementation: add("impl:" + cleanPath(a), .implementation(a))
            case .none, .record, .idiom: add("other:" + cleanPath(a), .other(a))
            }
            continue
        }
        let found: (roots: [String], implementations: [String])
        do {
            found = try findSpecs(a)
        } catch {
            stderr.write("specarch: cannot read \(a): \(plainIOError(error))\n")
            ioError = true
            continue
        }
        if found.roots.isEmpty && found.implementations.isEmpty {
            stderr.write("specarch: \(a) holds no \(rootFile) and no *\(implementationSuffix) file; name a folder that does\n")
            ioError = true
        }
        for r in found.roots { add("root:" + cleanPath(r), .root(r)) }
        for i in found.implementations { add("impl:" + cleanPath(i), .implementation(i)) }
    }
    return (inputs, ioError)
}

private func plural(_ n: Int, _ word: String) -> String {
    n == 1 ? "1 \(word)" : "\(n) \(word)s"
}
