import Foundation

extension Checker {
    /// Reads the specification's timeZone, the name of the IANA Time Zone
    /// Database a date or a wall-clock reading meets the clock in: date()
    /// of an instant gives the instant's calendar date in it, and in UTC
    /// when none is declared, which zoneWarnings reports at the call
    /// (ADR-193). A name the database does not hold is time_zone at the
    /// key, and the date is UTC's until the key is corrected.
    func checkTimeZone(_ d: Design) {
        zone = TimeZone(identifier: "UTC")!
        guard let n = d.root.child("timeZone"), n.kind == .scalar else { return }
        zoneDeclared = true
        guard let z = zoneNamed(str(n)) else {
            add(n, "/timeZone", .timeZone, "timeZone \(str(n)) is not a name of the IANA Time Zone Database; write one as the database spells it, such as Asia/Jakarta or UTC")
            return
        }
        zone = z
    }
}

/// The form of a name of the database: parts of letters, digits,
/// underscores, plus and minus, each begun with a capital, joined by slashes
/// (Asia/Jakarta, Etc/GMT+7, America/Port_of_Spain, UTC). The capital keeps
/// out asia/jakarta, which a case-insensitive file system would otherwise
/// read on one host and not on another.
private let zoneNamePattern = try! NSRegularExpression(pattern: "^[A-Z][A-Za-z0-9_+-]*(/[A-Z][A-Za-z0-9_+-]*)*$")

/// The folders a host keeps the database in, the ones the Go build's time
/// package reads, in its order.
private let zoneSources = ["/usr/share/zoneinfo/", "/usr/share/lib/zoneinfo/", "/usr/lib/locale/TZ/", "/etc/zoneinfo"]

/// Reads a timeZone: a name of the database, spelled as the database
/// spells it. It is nil when the name is none. Local, the host's own zone,
/// is no name of the database; a name a folder of the host's database
/// holds in another spelling is refused, since the Go build's time package
/// would read it on a case-insensitive file system alone; and a name no
/// folder holds must be one the database lists, since Foundation knows
/// names beyond it (PST, GMT+7) that the Go build refuses.
func zoneNamed(_ name: String) -> TimeZone? {
    guard matches(zoneNamePattern, name), name != "Local" else { return nil }
    let (held, exact) = zoneSpelled(name)
    if held && !exact { return nil }
    if !held && name != "UTC" && !TimeZone.knownTimeZoneIdentifiers.contains(name) { return nil }
    return TimeZone(identifier: name)
}

/// Whether a folder of the host's database holds the name, read
/// case-insensitively, and whether it holds it spelled as given.
func zoneSpelled(_ name: String) -> (held: Bool, exact: Bool) {
    for dir in zoneSources {
        var path = dir, found = true, spelled = true
        for part in name.split(separator: "/").map(String.init) {
            guard let entries = try? FileManager.default.contentsOfDirectory(atPath: path) else {
                found = false
                break
            }
            var match = ""
            for e in entries {
                if e == part {
                    match = part
                    break
                }
                if e.lowercased() == part.lowercased() { match = e }
            }
            if match.isEmpty {
                found = false
                break
            }
            if match != part { spelled = false }
            path = (path as NSString).appendingPathComponent(match)
        }
        if found { return (true, spelled) }
    }
    return (false, false)
}
