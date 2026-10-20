import Foundation

// The keywords of an operation's transport: the handshake whose envelope
// it travels in (ADR-145) and the stream it carries a file as (ADR-146).

/// The methods whose request carries a body, so an upload stream sits on
/// one of them.
private let streamMethods: Set<String> = ["post", "put"]

extension Checker {
    /// Checks each declared handshake: the operation that establishes it
    /// exists and travels in no envelope itself.
    func checkHandshakes(_ d: Design) {
        for h in pairs(d.root.child("handshakes")) {
            guard let by = h.value.child("establishedBy") else { continue }
            let ptr = pointer("handshakes", h.key.value, "establishedBy")
            guard let o = d.operations[by.value] else {
                var valid: [String: YNode] = [:]
                for (id, op) in d.operations { valid[id] = op.node }
                add(by, ptr, .handshake, "\(by.value) is not an operationId of the specification; name the operation a client calls to establish the handshake\(suggest(by.value, valid))")
                continue
            }
            if let into = o.node.child("handshake") {
                add(by, ptr, .handshake, "\(by.value) travels in the envelope of handshake \(into.value), and the operation that establishes a handshake is called before any envelope exists; give it no handshake")
            }
        }
    }

    /// Checks that an operation's handshake is declared and that the
    /// operation carries no upload, whose body is the file and not a JSON
    /// body the envelope wraps.
    func checkHandshake(_ d: Design, _ o: Operation) {
        guard let n = o.node.child("handshake") else { return }
        let ptr = o.pointer("handshake")
        if d.handshakes[n.value] == nil {
            add(n, ptr, .handshake, "\(n.value) is not a handshake of the specification\(suggest(n.value, d.handshakes))")
        }
        if str(o.node.child("stream")?.child("direction")) == "upload" {
            add(n, ptr, .handshake, "\(o.id) streams an upload, whose request body is the file itself, and the envelope of \(n.value) carries a JSON body; an upload cannot travel in it")
        }
    }

    /// Checks an operation's stream against the body it names: an upload on
    /// a post or a put whose request body is one binary media type, a
    /// download whose success responses each carry one binary media type,
    /// and maxResponseBytes only beside a download.
    func checkStream(_ o: Operation) {
        let s = o.node.child("stream")
        let dir = str(s?.child("direction"))
        if let n = o.node.child("limits")?.child("maxResponseBytes"), dir != "download" {
            add(n, o.pointer("limits", "maxResponseBytes"), .stream, "maxResponseBytes bounds the file a download streams, and \(o.id) streams none; remove it, or give the operation stream: {direction: download}")
        }
        guard let s else { return }
        let ptr = o.pointer("stream")
        checkStreamParameters(o, s, dir)
        switch dir {
        case "upload":
            if !streamMethods.contains(o.method) {
                add(s.child("direction"), ptr + "/direction", .stream, "\(o.id) streams an upload on \(o.method), and a stream is sent in the body of a post or a put")
            }
            guard let body = o.node.child("requestBody") else {
                add(s, ptr, .stream, "\(o.id) streams an upload and declares no request body; declare one with the file's media type and a schema of type string and format binary")
                return
            }
            let what = notOneBinary(body.child("content"))
            if !what.isEmpty {
                add(s, ptr, .stream, "an upload stream's request body is one media type whose schema is a string of format binary, and \(o.id) declares \(what)")
            }
        case "download":
            var found = false
            for r in pairs(o.node.child("responses")) where r.key.value.hasPrefix("2") {
                found = true
                let what = notOneBinary(r.value.child("content"))
                if !what.isEmpty {
                    add(s, ptr, .stream, "a download stream's success response carries one media type whose schema is a string of format binary, and the \(r.key.value) response of \(o.id) declares \(what)")
                }
            }
            if !found {
                add(s, ptr, .stream, "\(o.id) streams a download and declares no success response to carry it; declare a 2xx response with the file's media type and a schema of type string and format binary")
            }
        default:
            break
        }
    }
}

extension Checker {
    /// Checks an upload's parameters (ADR-162): beside an upload only, an
    /// object, and no property named as a path or query parameter of the
    /// operation, since a stack that folds them together would carry two
    /// values under one name.
    func checkStreamParameters(_ o: Operation, _ s: YNode, _ dir: String) {
        guard let params = s.child("parameters") else { return }
        let ptr = o.pointer("stream", "parameters")
        if dir != "upload" {
            add(s.key("parameters"), ptr, .stream, "\(o.id) streams a \(dir), and parameters travel beside the file of an upload only; send them in the request body, or leave them out")
            return
        }
        if params.child("$ref") != nil { return }
        let t = str(params.child("type"))
        if t != "object" {
            let what = t.isEmpty ? "of no type" : "of type " + t
            add(params, ptr, .stream, "the parameters of \(o.id)'s upload are one JSON object sent beside the file, and its schema is \(what); give it type: object with its properties, or a $ref to a schema")
            return
        }
        var taken: [String: String] = [:]
        for list in [o.pathItem.child("parameters"), o.node.child("parameters")] {
            for p in items(list) {
                let inValue = str(p.child("in"))
                if inValue == "path" || inValue == "query" {
                    taken[str(p.child("name"))] = inValue
                }
            }
        }
        for p in pairs(params.child("properties")) {
            if let inValue = taken[p.key.value] {
                add(p.key, o.pointer("stream", "parameters", "properties", p.key.value), .stream, "\(p.key.value) is a \(inValue) parameter of \(o.id) and a parameter of its upload; a stack that sends them together would carry two values under one name, so name one of them otherwise")
            }
        }
    }
}

/// What a content map declares when it is not one media type whose schema
/// is a string of format binary, or "" when it is.
func notOneBinary(_ content: YNode?) -> String {
    let ps = pairs(content)
    if ps.isEmpty { return "no content" }
    if ps.count > 1 {
        return "\(ps.count) media types (\(ps.map { $0.key.value }.joined(separator: ", ")))"
    }
    let schema = ps[0].value.child("schema")
    let t = str(schema?.child("type")), f = str(schema?.child("format"))
    if t == "string" && f == "binary" { return "" }
    guard let schema else { return ps[0].key.value + " with no schema" }
    let ref = str(schema.child("$ref"))
    if !ref.isEmpty { return ps[0].key.value + " with a reference to " + ref }
    let type = t.isEmpty ? "no type" : t
    if f.isEmpty { return ps[0].key.value + " with " + type + " and no format" }
    return ps[0].key.value + " with " + type + " of format " + f
}
