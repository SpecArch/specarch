import Foundation

// The version line of a SpecArch file: which rules the file is written
// against, and so which schema checks it (docs/meta-model-1.0.0.md). A
// version is read by its major and minor; the patch never changes what is
// valid or what a file means. internal/metamodel in the Go build.

/// The newest version of every SpecArch format, the one specarch upgrade
/// writes.
let currentMetaModel = "1.0.0"

/// One kind of SpecArch file and the versions of it a program reads.
struct MetaModelFormat: Sendable {
    /// The key of the version line, such as specarch.
    let key: String
    /// Names the format in a message.
    let name: String
    /// The versions before 1.0.0 read beside it, each with a warning,
    /// oldest first.
    let earlier: [String]
    /// A format whose earlier versions are read with no warning: a
    /// vocabulary, which only the Go build reads.
    var silent = false

    /// The formats, each by the key of its version line. The
    /// specification's is the meta-model; the others are as old as 0.1,
    /// the same in shape as 1.0.0.
    static let design = MetaModelFormat(key: "specarch", name: "meta-model", earlier: ["0.1", "0.2", "0.3"])
    static let implementation = MetaModelFormat(key: "specarchImplementation", name: "implementation file", earlier: ["0.1"])
    static let record = MetaModelFormat(key: "specarchRecord", name: "record", earlier: ["0.1"])
    static let idiom = MetaModelFormat(key: "specarchIdiom", name: "idiom", earlier: ["0.1"])
    static let estate = MetaModelFormat(key: "specarchEstate", name: "estate", earlier: ["0.1"])
    static let vocabulary = MetaModelFormat(key: "specarchVocabulary", name: "vocabulary", earlier: ["0.1"], silent: true)

    /// What a version line says to this program.
    enum Class { case known, earlier, later, short, unknown }

    func classify(_ v: String) -> Class {
        if earlier.contains(v) { return .earlier }
        if matches(currentPattern, v) { return .known }
        if !matches(numericPattern, v) { return .unknown }
        let parts = v.split(separator: ".", omittingEmptySubsequences: false)
        guard let major = Int(parts[0]), major >= 1 else { return .unknown }
        return parts.count < 3 ? .short : .later
    }

    /// The version a file is read as: its own when this program reads it,
    /// otherwise the current one.
    func reads(_ v: String) -> String {
        switch classify(v) {
        case .known, .earlier: return v
        default: return currentMetaModel
        }
    }

    /// Whether a file whose line says v is read as a version before 1.0.0,
    /// which the earlier schema checks.
    func isEarlier(_ v: String) -> Bool { classify(v) == .earlier }

    /// The versions a format is read in, for a message.
    var reading: String {
        let s = currentMetaModel + " and any later 1.0 patch"
        if earlier.isEmpty { return s }
        return s + ", and " + joinAnd(Array(earlier.reversed())) + (silent ? "" : " with a warning")
    }

    /// The message for a version line this program does not read, or nil
    /// for one it reads. The file is then read as the current version.
    func problem(_ v: String) -> String? {
        let tail = "; this program reads " + reading + ", and reads the file as " + currentMetaModel
        switch classify(v) {
        case .later:
            return quote(v) + " is a later version than this program reads, which a later specarch reads" + tail
        case .short:
            return quote(v) + " is written in fewer than three parts; write the version as major.minor.patch, such as " + quote(currentMetaModel) + tail
        case .unknown:
            return quote(v) + " is not a version of this format" + tail
        default:
            return nil
        }
    }
}

private let currentPattern = try! NSRegularExpression(pattern: #"^1\.0\.(0|[1-9][0-9]*)$"#)
private let numericPattern = try! NSRegularExpression(pattern: #"^(0|[1-9][0-9]*)(\.(0|[1-9][0-9]*)){0,2}$"#)

/// Whether version v, read as its major and minor, is major.minor or later.
func metaModelAtLeast(_ v: String, _ major: Int, _ minor: Int) -> Bool {
    let parts = v.split(separator: ".", omittingEmptySubsequences: false)
    guard parts.count >= 2, let ma = Int(parts[0]), let mi = Int(parts[1]) else { return false }
    return ma > major || ma == major && mi >= minor
}

/// The keys meta-model 1.0.0 renamed, each still read under its old
/// spelling with a meta_model_deprecated warning until 2.0.0.
let rowPoliciesKey = "rowPolicies"
let rowPoliciesOldKey = "rows"
let sourceDeliveryKey = "sourceDelivery"
let sourceDeliveryOldKey = "source"

/// A role's row policies and the key the file gives them under:
/// rowPolicies, or rows, its spelling before 1.0.0.
func rowPolicies(_ role: YNode?) -> (key: String, node: YNode?) {
    renamed(role, rowPoliciesKey, rowPoliciesOldKey)
}

/// How the source of a release reaches the customer, under the handover's
/// sourceDelivery or source, its spelling before 1.0.0.
func sourceDelivery(_ handover: YNode?) -> (key: String, node: YNode?) {
    renamed(handover, sourceDeliveryKey, sourceDeliveryOldKey)
}

private func renamed(_ n: YNode?, _ key: String, _ old: String) -> (key: String, node: YNode?) {
    if let v = n?.child(key) { return (key, v) }
    if let v = n?.child(old) { return (old, v) }
    return (key, nil)
}
