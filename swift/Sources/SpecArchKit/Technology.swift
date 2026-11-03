import Foundation

/// What in the design needs a binding of each interface kind: the
/// sections whose elements the binding's framework serves. A design with
/// none of them needs no binding of that kind (ADR-231).
let bindingNeeds: [String: String] = [
    "http": "no paths, routes or gateways",
    "messaging": "no channels and no dependency of kind messageBroker",
    "cli": "no commands",
    "ui": "no pages",
    "storage": "no entities or views",
]

/// Whether the design has an element a binding of the kind serves.
func bindingNeeded(_ d: Design, _ kind: String) -> Bool {
    switch kind {
    case "http":
        return !d.opList.isEmpty || !pairs(d.root.child("routes")).isEmpty || !pairs(d.root.child("gateways")).isEmpty
    case "messaging":
        if !d.channels.isEmpty { return true }
        return d.dependencies.values.contains { str($0.child("kind")) == "messageBroker" }
    case "cli":
        return !d.commands.isEmpty
    case "ui":
        return !d.pages.isEmpty
    case "storage":
        return !d.entities.isEmpty || !d.views.isEmpty
    default:
        return true
    }
}

extension Checker {
    /// Warns at a binding whose framework the design does not need: no
    /// element of its kind, no idiom of the file's stacks approving the
    /// framework, and no decision that justifies it against a simpler
    /// alternative (ADR-231). approved maps each approved name to the idiom
    /// that approves it; decisions are the design's and the file's. A
    /// binding on the standard library adopts nothing and is left alone,
    /// and a decidedIn naming no decision is the origin check's error
    /// already.
    func checkTechnologies(_ d: Design, _ decisions: [String: YNode], _ approved: [String: String]) {
        for b in pairs(root.child("bindings")) {
            let kind = b.key.value
            let framework = str(b.value.child("framework"))
            guard let lack = bindingNeeds[kind], !framework.isEmpty, framework != "standard library", !bindingNeeded(d, kind) else { continue }
            if approved[framework] != nil { continue }
            let ptr = pointer("bindings", kind)
            let lead = "nothing in the specification needs a \(kind) binding: it has \(lack), and "
            guard let decidedIn = b.value.child("decidedIn") else {
                warn(b.key, ptr, .technologyUnjustified, "\(lead)no decision justifies \(framework) against a simpler alternative; add the design elements that need it, or set origin decided and decidedIn naming an accepted decision that satisfies a requirement and names under alternatives the simpler option it rejects, or approve it under technologies in an idiom of this file's stacks")
                continue
            }
            guard let dec = decisions[decidedIn.value] else { continue }
            var missing: [String] = []
            if items(dec.child("satisfies")).isEmpty { missing.append("no requirement under satisfies") }
            if items(dec.child("alternatives")).isEmpty { missing.append("no simpler alternative under alternatives") }
            if missing.isEmpty { continue }
            warn(decidedIn, ptr + "/decidedIn", .technologyUnjustified, "\(lead)\(decidedIn.value), which decides \(framework), names \(missing.joined(separator: " and ")); add them, so the decision says what \(framework) is for and which simpler option it rejects")
        }
    }
}

/// The technologies the idioms approve for an implementation file: every
/// name under technologies of an idiom that renders one of the file's
/// stacks, whatever the design reads, mapped to the idiom's name (ADR-231).
func approvedTechnologies(_ apply: [String: Idiom], _ isStack: Set<String>) -> [String: String] {
    var out: [String: String] = [:]
    for name in apply.keys.sorted(by: byteLess) {
        let i = apply[name]!
        guard items(i.root.child("stacks")).contains(where: { isStack.contains($0.value) }) else { continue }
        for t in pairs(i.root.child("technologies")) {
            for n in items(t.value.child("names")) where out[n.value] == nil {
                out[n.value] = name
            }
        }
    }
    return out
}
