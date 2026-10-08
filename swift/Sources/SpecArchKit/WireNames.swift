import Foundation

// A property's name on the wire, by the rule info.wireNames names
// (ADR-062), and the check that two properties of one object do not take
// one wire name. The Go build's internal/wirename writes the same names.

/// The only rule info.wireNames takes.
let snakeCaseWireNames = "snake_case"

/// A camelCase name in snake_case: a capital starts a word after a
/// lower-case letter or a digit, and so does the last capital of a run of
/// capitals that a lower-case letter follows. loanedOn is loaned_on, userID
/// and userId are both user_id, and HTTPServer is http_server.
func snakeWireName(_ name: String) -> String {
    func isLower(_ c: Unicode.Scalar) -> Bool { c >= "a" && c <= "z" }
    func isUpper(_ c: Unicode.Scalar) -> Bool { c >= "A" && c <= "Z" }
    func isDigit(_ c: Unicode.Scalar) -> Bool { c >= "0" && c <= "9" }
    let rs = Array(name.unicodeScalars)
    var out = String.UnicodeScalarView()
    for (i, r) in rs.enumerated() {
        if isUpper(r) {
            if i > 0 && (isLower(rs[i - 1]) || isDigit(rs[i - 1]) || i + 1 < rs.count && isLower(rs[i + 1]) && isUpper(rs[i - 1])) {
                out.append("_")
            }
            out.append(Unicode.Scalar(r.value + 32)!)
            continue
        }
        out.append(r)
    }
    return String(out)
}

/// A name on the wire under a rule: the name as written when the rule is
/// empty.
func wireName(_ rule: String, _ name: String) -> String {
    rule == snakeCaseWireNames ? snakeWireName(name) : name
}

extension Checker {
    /// Refuses two properties of one object that go on the wire under one
    /// name, once the specification names its wire names: an entity's, a
    /// view's together with its entity's, and the inline objects of a body,
    /// a parameter's schema and a message's payload, at any depth.
    func checkWireNames(_ d: Design) {
        let rule = str(d.root.child("info")?.child("wireNames"))
        guard rule == snakeCaseWireNames else { return }
        for e in pairs(d.root.child("entities")) {
            wireObject(rule, nil, e.value.child("properties"), ["entities", e.key.value])
        }
        for v in pairs(d.root.child("views")) {
            let from = d.entities[str(v.value.child("from"))]
            wireObject(rule, from?.child("properties"), v.value.child("properties"), ["views", v.key.value])
        }
        for p in pairs(d.root.child("paths")) {
            wireParameters(rule, p.value.child("parameters"), ["paths", p.key.value])
            for m in methods {
                guard let op = p.value.child(m) else { continue }
                let at = ["paths", p.key.value, m]
                wireParameters(rule, op.child("parameters"), at)
                wireContent(rule, op.child("requestBody")?.child("content"), at + ["requestBody"])
                for r in pairs(op.child("responses")) {
                    wireContent(rule, r.value.child("content"), at + ["responses", r.key.value])
                }
            }
        }
        for ch in pairs(d.root.child("channels")) {
            for m in pairs(ch.value.child("messages")) {
                wireField(rule, m.value.child("payload"), ["channels", ch.key.value, "messages", m.key.value, "payload"])
            }
        }
    }

    private func wireParameters(_ rule: String, _ params: YNode?, _ at: [String]) {
        guard let params, params.kind == .sequence else { return }
        for (i, p) in params.items.enumerated() {
            wireField(rule, p.child("schema"), at + ["parameters", String(i), "schema"])
        }
    }

    private func wireContent(_ rule: String, _ content: YNode?, _ at: [String]) {
        for mt in pairs(content) {
            wireField(rule, mt.value.child("schema"), at + ["content", mt.key.value, "schema"])
        }
    }

    /// Checks a field's own properties and its items', at any depth.
    private func wireField(_ rule: String, _ f: YNode?, _ at: [String]) {
        guard let f, f.kind == .mapping else { return }
        wireObject(rule, nil, f.child("properties"), at)
        wireField(rule, f.child("items"), at + ["items"])
    }

    /// Checks the properties of one object: first those it carries from
    /// elsewhere (a view's entity), which are reported against nothing, then
    /// its own, each reported where it takes a wire name already taken.
    private func wireObject(_ rule: String, _ carried: YNode?, _ props: YNode?, _ at: [String]) {
        var taken: [String: String] = [:]
        for p in pairs(carried) {
            let wire = wireName(rule, p.key.value)
            if taken[wire] == nil { taken[wire] = p.key.value }
        }
        for p in pairs(props) {
            let name = p.key.value
            let wire = wireName(rule, name)
            let path = at + ["properties", name]
            if let other = taken[wire] {
                if other != name {
                    add(p.key, pointer(path), .wireName, "\(other) and \(name) both go on the wire as \(wire) (info.wireNames: \(rule)); rename one")
                }
            } else {
                taken[wire] = name
            }
            wireField(rule, p.value, path)
        }
    }
}
