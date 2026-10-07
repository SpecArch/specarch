import Foundation

/// The program version.
public let programVersion = "0.1.0"

let usage = """
usage:
  specarch validate <file or folder>...   check SpecArch files
  specarch version                         print the program version

A folder is searched for *.specarch-design.yaml and
*.specarch-implementation.yaml files.

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
        stdout.write("specarch \(programVersion)\ndesign files: meta-model 0.1\nimplementation files: meta-model 0.1\n")
        return 0
    case "generate":
        stderr.write("specarch generate: this build has no generators; the Go build of specarch has them\n")
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
/// error, 1 when any file has an error, 0 otherwise.
func runValidate(_ argsIn: [String], _ stdout: TextSink, _ stderr: TextSink) -> Int32 {
    var args = argsIn
    if args.first == "--" { args.removeFirst() }
    if args.isEmpty {
        stderr.write("specarch validate needs at least one file or folder\n\n\(usage)")
        return 2
    }
    for a in args where a.count > 1 && a.hasPrefix("-") {
        stderr.write("specarch validate has no option \(a); to check a file whose name starts with -, write -- before it\n\n\(usage)")
        return 2
    }
    let (files, collectError) = collect(args, stderr)
    var ioError = collectError
    var all: [Diagnostic] = []
    for f in files {
        guard let data = FileManager.default.contents(atPath: f) else {
            stderr.write("specarch: cannot read \(f)\n")
            ioError = true
            continue
        }
        all += check(path: f, data: data, read: readFile)
    }
    sortDiagnostics(&all)
    for d in all { stdout.write(d.description + "\n") }
    let errors = errorCount(all)
    stderr.write("specarch: \(plural(files.count, "file")) checked: \(plural(errors, "error")), \(plural(all.count - errors, "warning"))\n")
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

/// Expands folders into the SpecArch files under them, sorted, and keeps
/// files as given.
func collect(_ args: [String], _ stderr: TextSink) -> ([String], Bool) {
    var files: [String] = []
    var seen = Set<String>()
    var ioError = false
    func addFile(_ p: String) {
        if !seen.contains(p) {
            seen.insert(p)
            files.append(p)
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
            addFile(a)
            continue
        }
        var found: [String] = []
        if let walker = FileManager.default.enumerator(atPath: a) {
            for case let rel as String in walker {
                let full = joinPath(a, rel)
                var sub: ObjCBool = false
                if FileManager.default.fileExists(atPath: full, isDirectory: &sub), !sub.boolValue,
                   kindOf((rel as NSString).lastPathComponent) != .none {
                    found.append(full)
                }
            }
        }
        if found.isEmpty {
            stderr.write("specarch: \(a) holds no *\(designSuffix) or *\(implementationSuffix) file; name a folder that does\n")
            ioError = true
        }
        for f in found.sorted(by: byteLess) { addFile(f) }
    }
    return (files, ioError)
}

private func plural(_ n: Int, _ word: String) -> String {
    n == 1 ? "1 \(word)" : "\(n) \(word)s"
}
