import Foundation

/// The name of an estate's file: the modules' specifications it lists and
/// pins.
public let estateFile = "specarch-estate.yaml"

/// An estate file as read from disk (ADR-113).
final class Estate {
    let path: String
    let doc: Doc
    var modules: [EstateModule] = [] // in the order the file lists them

    init(path: String, data: Data) {
        self.path = path
        doc = parseYAML(yamlText(data))
        guard let root = doc.root, root.kind == .mapping else { return }
        for p in child(root, "modules")?.pairs ?? [] {
            var location = ""
            if let loc = child(p.value, "location")?.str, !loc.isEmpty {
                location = joinPath(dirPath(path), loc)
            }
            modules.append(EstateModule(key: p.key.value, location: location, version: str(child(p.value, "version")), keyNode: p.key, value: p.value))
        }
    }

    /// The folders of the modules whose specification is there: the ones
    /// the estate gives the run to check with it.
    var roots: [String] {
        modules.filter { !$0.location.isEmpty && isRegularFile(joinPath($0.location, rootFile)) }.map(\.location)
    }
}

/// One module an estate lists.
struct EstateModule {
    let key: String
    let location: String // the folder of its specification, from the estate's folder; "" when the entry gives none
    let version: String
    let keyNode: YNode
    let value: YNode
}

func isRegularFile(_ path: String) -> Bool {
    var isDir: ObjCBool = false
    return FileManager.default.fileExists(atPath: path, isDirectory: &isDir) && !isDir.boolValue
}

/// Checks an estate against the specifications read in the run, by their
/// folder: that each module is where it is listed, named and at the version
/// the estate lists, and that no module's calls come back to it through the
/// others.
func checkEstate(_ e: Estate, _ specs: [String: Spec]) -> [Diagnostic] {
    let c = Checker(file: e.path)
    for p in e.doc.problems {
        c.addLine(p.line, p.path, Rule(rawValue: p.rule)!, p.message, detail: p.detail)
    }
    if let root = e.doc.root {
        c.root = root
        c.checkSchema(.estate, e.doc.value)
        if root.kind == .mapping {
            c.checkModules(e, specs)
        }
    }
    var ds = withoutEchoes(withoutHeld(c.diags, heldIn(e.path, e.doc)))
    Placer(dir: dirPath(e.path)).place(&ds)
    sortDiagnostics(&ds)
    return ds
}

extension Checker {
    func checkModules(_ e: Estate, _ specs: [String: Spec]) {
        var found: [String: Spec] = [:]
        for m in e.modules where !m.location.isEmpty {
            let ptr = pointer("modules", m.key)
            let loc = str(child(m.value, "location"))
            guard let s = specs[m.location] else {
                var why = "it holds no \(rootFile)"
                if !FileManager.default.fileExists(atPath: m.location) {
                    why = "there is no such folder"
                }
                let base = e.path.split(separator: "/").last.map(String.init) ?? e.path
                add(child(m.value, "location"), ptr + "/location", .module,
                    "\(m.key) is listed at \(loc), and \(why); write the folder that holds the module's \(rootFile), relative to \(base)")
                continue
            }
            guard let root = s.root, root.kind == .mapping else { continue }
            found[m.key] = s
            let name = specName(root)
            if name.isEmpty {
                add(m.keyNode, ptr, .module, "the specification at \(loc) has no info.name, so it cannot be told to be \(m.key); add name: \(m.key) to its info")
            } else if name != m.key {
                add(m.keyNode, ptr, .module, "the specification at \(loc) is named \(name), not \(m.key); list it under its info.name, or correct the name")
            }
            let v = str(child(child(root, "info"), "version"))
            if !v.isEmpty && !m.version.isEmpty && v != m.version {
                add(child(m.value, "version"), ptr + "/version", .module,
                    "\(m.key) is at version \(v), and the estate lists \(m.version); check it against the other modules, then write \(v) here")
            }
        }
        // An access token is minted for one service, so no two modules
        // accept one audience value; every module of a trust domain accepts
        // the one transaction token, whose audience is the trust domain
        // (ADR-109).
        var first: [String: String] = [:]
        for m in e.modules {
            guard let root = found[m.key]?.root else { continue }
            for aud in Design(root).acceptedAudiences {
                if let other = first[aud] {
                    add(m.keyNode, pointer("modules", m.key), .tokenAudience,
                        "\(other) and \(m.key) both accept an access token for the audience \(aud), so a token minted for one is accepted by the other; give each module an audience of its own")
                    continue
                }
                first[aud] = m.key
            }
        }
        let edges = moduleCalls(found)
        for m in e.modules {
            if let cycle = callCycle(m.key, edges) {
                add(m.keyNode, pointer("modules", m.key), .module,
                    "the operations of \(m.key) call round a cycle of modules: \(cycleText(cycle)); a call waits on one that waits on it, so drop one of the calls or pass that work by a channel message")
            }
        }
    }
}

/// For each module, the modules its operations and jobs call through a
/// dependency whose specification is one of them, sorted.
func moduleCalls(_ found: [String: Spec]) -> [String: [String]] {
    var edges: [String: [String]] = [:]
    for (key, s) in found {
        var deps: [String: String] = [:]
        for d in child(s.root, "dependencies")?.pairs ?? [] {
            deps[d.key.value] = str(child(d.value, "specification"))
        }
        var callers: [YNode] = []
        for p in child(s.root, "paths")?.pairs ?? [] {
            for op in p.value.pairs { callers.append(op.value) }
        }
        for j in child(s.root, "jobs")?.pairs ?? [] { callers.append(j.value) }
        var seen = Set<String>()
        var out: [String] = []
        for n in callers {
            for e in callEntries(n) {
                let to = deps[callDependency(e.ref.str)] ?? ""
                if found[to] != nil && !seen.contains(to) {
                    seen.insert(to)
                    out.append(to)
                }
            }
        }
        edges[key] = out.sorted(by: byteLess)
    }
    return edges
}

/// The shortest cycle of calls through module key, starting at the
/// smallest key on it, or nil when key is on none.
func callCycle(_ key: String, _ edges: [String: [String]]) -> [String]? {
    var best: [String]?
    for start in cycleMembers(key, edges) {
        if let path = shortestPath(start, start, edges), path.contains(key), best == nil || path.count < best!.count {
            best = path
        }
    }
    return best
}

/// The modules from which key is reached and which key reaches, sorted:
/// those that share a cycle with it.
func cycleMembers(_ key: String, _ edges: [String: [String]]) -> [String] {
    edges.keys.sorted(by: byteLess).filter { shortestPath(key, $0, edges) != nil && shortestPath($0, key, edges) != nil }
}

/// The shortest path of one call or more from one module to another,
/// taking each module's calls in order, or nil.
func shortestPath(_ from: String, _ to: String, _ edges: [String: [String]]) -> [String]? {
    var prev: [String: String] = [:]
    var queue = [from]
    var visited = Set<String>()
    var i = 0
    while i < queue.count {
        let n = queue[i]
        i += 1
        for next in edges[n] ?? [] {
            if next == to {
                var path = [to]
                var at = n
                while at != from {
                    path.append(at)
                    at = prev[at]!
                }
                path.append(from)
                return path.reversed()
            }
            if !visited.contains(next) && next != from {
                visited.insert(next)
                prev[next] = n
                queue.append(next)
            }
        }
    }
    return nil
}

/// A cycle in words: a calls b, which calls a.
func cycleText(_ cycle: [String]) -> String {
    var out = cycle[0]
    for (i, k) in cycle.dropFirst().enumerated() {
        out += (i == 0 ? " calls " : ", which calls ") + k
    }
    return out
}
