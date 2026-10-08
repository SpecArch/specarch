import Foundation

/// A Semantic Versioning 2.0.0 version, without build metadata.
struct Version {
    let major, minor, patch: Int
    let pre: String

    init?(_ v: String) {
        let parts = v.split(separator: "-", maxSplits: 1, omittingEmptySubsequences: false)
        let core = parts[0].split(separator: ".", omittingEmptySubsequences: false)
        guard core.count == 3 else { return nil }
        var n: [Int] = []
        for p in core {
            guard let x = Int(p), x >= 0 else { return nil }
            n.append(x)
        }
        major = n[0]; minor = n[1]; patch = n[2]
        pre = parts.count > 1 ? String(parts[1]) : ""
    }

    var core: String { "\(major).\(minor).\(patch)" }
}

/// Orders two versions by SemVer's precedence (rule 11).
func compareVersions(_ a: Version, _ b: Version) -> Int {
    for (x, y) in [(a.major, b.major), (a.minor, b.minor), (a.patch, b.patch)] where x != y {
        return x < y ? -1 : 1
    }
    if a.pre == b.pre { return 0 }
    if a.pre.isEmpty { return 1 }
    if b.pre.isEmpty { return -1 }
    let x = a.pre.split(separator: ".", omittingEmptySubsequences: false).map(String.init)
    let y = b.pre.split(separator: ".", omittingEmptySubsequences: false).map(String.init)
    for i in 0..<min(x.count, y.count) {
        let c = compareIdentifiers(x[i], y[i])
        if c != 0 { return c }
    }
    if x.count != y.count { return x.count < y.count ? -1 : 1 }
    return 0
}

/// Compares two dot-separated pre-release identifiers: numbers
/// numerically, below any other identifier, and the rest in ASCII order.
private func compareIdentifiers(_ a: String, _ b: String) -> Int {
    let an = isNumeric(a), bn = isNumeric(b)
    if an && bn {
        if a.utf8.count != b.utf8.count { return a.utf8.count < b.utf8.count ? -1 : 1 }
    } else if an {
        return -1
    } else if bn {
        return 1
    }
    if a == b { return 0 }
    return byteLess(a, b) ? -1 : 1
}

private func isNumeric(_ s: String) -> Bool {
    !s.isEmpty && s.utf8.allSatisfy { $0 >= 48 && $0 <= 57 }
}

/// The version steps, smallest first.
private let stepNames = ["", "patch", "minor", "major"]

private func impactRank(_ impact: String) -> Int {
    guard let i = stepNames.firstIndex(of: impact), i > 0 else { return 0 }
    return i
}

/// How far cur steps from prev: 3 major, 2 minor, 1 patch, 0 none.
private func stepOf(_ prev: Version, _ cur: Version) -> Int {
    if cur.major > prev.major { return 3 }
    if cur.major == prev.major && cur.minor > prev.minor { return 2 }
    if cur.major == prev.major && cur.minor == prev.minor && cur.patch > prev.patch { return 1 }
    return 0
}

/// The step an impact needs from prev: before 1.0.0 a major change needs
/// the minor step and anything else the patch step.
private func neededStep(_ prev: Version, _ impact: Int) -> Int {
    if prev.major == 0 && impact > 0 { return max(impact - 1, 1) }
    return impact
}

private func nextVersion(_ prev: Version, _ step: Int) -> String {
    switch step {
    case 3: return "\(prev.major + 1).0.0"
    case 2: return "\(prev.major).\(prev.minor + 1).0"
    default: return "\(prev.major).\(prev.minor).\(prev.patch + 1)"
    }
}

extension RecordChecks {
    /// Checks the releases together: what each released one includes, how
    /// far it steps from the one before, and that info.version agrees with
    /// them. A diagnostic about info.version goes to root.
    func checkReleases(_ recs: [Record], _ root: Checker) {
        var released: [Record] = []
        for r in recs {
            switch r.kind {
            case "release":
                if let n = r.root?.child("specificationVersion") {
                    let v = str(n), want = r.str("version")
                    if !v.isEmpty && !want.isEmpty && v != want {
                        r.c.add(n, "/specificationVersion", .releaseVersion, "the specification's version at a release is the release's version; set it to \(want)")
                    }
                }
                if r.str("status") == "released" {
                    if Version(r.str("version")) != nil { released.append(r) }
                    checkContents(r)
                }
            case "change", "defect":
                checkNamedRelease(r)
            default:
                break
            }
        }
        released = released.enumerated().sorted { x, y in
            let c = compareVersions(Version(x.element.str("version"))!, Version(y.element.str("version"))!)
            return c != 0 ? c < 0 : x.offset < y.offset
        }.map(\.element)
        if released.count > 1 {
            for i in 1..<released.count { checkBump(released[i - 1], released[i]) }
        }
        if let newest = released.last { checkInfoVersion(newest, root) }
    }

    /// Checks what a released release includes.
    func checkContents(_ r: Record) {
        let ver = r.str("version")
        for (i, item) in items(r.root?.child("includes")).enumerated() {
            let id = item.value
            let path = pointer("includes", String(i))
            let x: Record
            if let c = set["change"]?[id] {
                x = c
                let st = x.str("status")
                if st != "implemented" && st != "released" {
                    r.c.add(item, path, .releaseContents, "\(id) is \(st), but a released release includes only changes that are implemented or released; take it out of includes, or carry the change out first")
                    continue
                }
            } else if let dft = set["defect"]?[id] {
                x = dft
                let st = x.str("status")
                if st != "fixed" && st != "released" {
                    r.c.add(item, path, .releaseContents, "\(id) is \(st), but a released release includes only defects that are fixed or released; take it out of includes, or fix the defect first")
                    continue
                }
            } else {
                continue // a tracker's ID, or reported as record_ref
            }
            let named = x.str("release")
            if x.str("status") == "released" && !named.isEmpty && named != ver {
                r.c.add(item, path, .releaseContents, "\(id) was released in \(named), which it names, so release \(ver) cannot include it as well; a change or defect is released once")
            }
        }
    }

    /// Checks that a released change or defect names a released release
    /// that includes it.
    func checkNamedRelease(_ x: Record) {
        if x.str("status") != "released" { return }
        let n = x.root?.child("release")
        let ver = str(n), id = x.str("id")
        guard !ver.isEmpty, let rel = set["release"]?[ver] else { return } // the schema or record_ref reports it
        let st = rel.str("status")
        if st != "released" {
            x.c.add(n, "/release", .releaseContents, "release \(ver) is \(st), so \(id) cannot be released in it yet; set \(id)'s status back, or release \(ver)")
            return
        }
        if items(rel.root?.child("includes")).contains(where: { $0.value == id }) { return }
        x.c.add(n, "/release", .releaseContents, "release \(ver) does not include \(id); add \(id) to its includes, or name the release that does")
    }

    /// Checks that cur steps from prev by at least what it includes needs.
    func checkBump(_ prevRec: Record, _ cur: Record) {
        let prev = Version(prevRec.str("version"))!, now = Version(cur.str("version"))!
        var need = 0, why = "", whyImpact = ""
        for item in items(cur.root?.child("includes")) {
            var impact = ""
            if let x = set["change"]?[item.value] {
                impact = x.str("impact")
            } else if set["defect"]?[item.value] != nil {
                impact = "patch"
            }
            let n = neededStep(prev, impactRank(impact))
            if n > need { (need, why, whyImpact) = (n, item.value, impact) }
        }
        if need == 0 || stepOf(prev, now) >= need { return }
        cur.c.add(cur.root?.child("version"), "/version", .releaseBump, "\(cur.str("version")) includes \(why), whose impact is \(whyImpact), so it needs a \(stepNames[need]) step from \(prevRec.str("version")); release it as \(nextVersion(prev, need))")
    }

    /// Checks info.version against the newest released release and the
    /// planned ones.
    func checkInfoVersion(_ newest: Record, _ root: Checker) {
        guard let n = d.root.child("info")?.child("version") else { return }
        let v = str(n), latest = newest.str("version")
        if v.isEmpty || v == latest { return }
        guard let pv = Version(v) else { return } // the schema reports it
        let lv = Version(latest)!
        if !pv.pre.isEmpty {
            if let planned = set["release"]?[pv.core], planned.str("status") == "planned", compareVersions(pv, lv) > 0 { return }
            root.add(n, "/info/version", .releaseVersion, "info.version is \(v), a pre-release, but there is no planned release \(pv.core) after \(latest); add records/releases/\(pv.core).yaml with status planned, or set info.version to \(latest)")
            return
        }
        root.add(n, "/info/version", .releaseVersion, "info.version is \(v), but the newest released version is \(latest); set it to \(latest), or, while the next release is built, to that release's version with a pre-release tag and a planned release record")
    }
}
