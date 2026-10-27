import Foundation

// A decision table (ADR-199): each rule is a box, one set of values per
// input, and overlap and gaps are found by intersecting and subtracting
// boxes. A set holds an enum's or a bool's members, an ordered type's
// intervals, or a string's values (or every string but them), and null.

struct Edge {
    var v: ExprValue?   // nil: unbounded
    var closed = false
}

struct Interval {
    var lo = Edge()
    var hi = Edge()
}

struct ValueSet {
    var null = false
    var members: [String] = []   // enum, bool: in the column's order
    var ivs: [Interval] = []     // int, uint, decimal, double, date: sorted, apart
    var except = false           // string: every string but texts
    var texts: [String] = []     // string: sorted

    var isEmpty: Bool { !null && members.isEmpty && ivs.isEmpty && texts.isEmpty && !except }
}

/// One comparison of a cell: op is "" for equality.
struct TableCondition {
    let op: String
    let v: ExprValue
}

/// What a rule's condition on one input says, as written: a list of
/// alternatives, each null (nil) or comparisons that all hold.
struct TableCell {
    var alternatives: [[TableCondition]?] = []
}

struct TableColumn {
    let name: String
    let field: YNode
    let t: ExprType
    var domain = ValueSet()
    let ok: Bool // a type a table tests
}

struct TableRule {
    let node: YNode
    var when: [String: ValueSet] = [:] // by input, as written (not cut to the domain)
    var cells: [(name: String, cell: TableCell)] = []
    var cellNodes: [String: YNode] = [:]
    var then = ExprValue(.null)
    var box: [ValueSet] = []
}

func isOrdered(_ k: ExprKind) -> Bool {
    [.int, .uint, .decimal, .double, .date].contains(k)
}

private func cmpValue(_ a: ExprValue, _ b: ExprValue) -> Int {
    func cmp<T: Comparable>(_ x: T, _ y: T) -> Int { x < y ? -1 : (x > y ? 1 : 0) }
    switch a.kind {
    case .double: return cmp(a.double, b.double)
    case .date: return cmp(a.seconds, b.seconds)
    default: return cmp(a.num, b.num)
    }
}

/// Moves a value of a discrete type by one place: an int by 1, a decimal
/// by its last digit, a date by a day; a double has no step.
private func step(_ v: ExprValue, _ t: ExprType, _ by: Int) -> ExprValue? {
    var out = v
    switch v.kind {
    case .int, .uint:
        out.num = v.num + Decimal(by)
    case .decimal:
        out.num = v.num + Decimal(sign: by < 0 ? .minus : .plus, exponent: -t.scale, significand: 1)
    case .date:
        out.seconds = v.seconds + Decimal(86400 * by)
    default:
        return nil
    }
    return out
}

/// Closes an open edge of a discrete type on the next value in.
private func normal(_ iv: Interval, _ t: ExprType) -> Interval {
    var iv = iv
    if let lo = iv.lo.v, !iv.lo.closed, let v = step(lo, t, 1) { iv.lo = Edge(v: v, closed: true) }
    if let hi = iv.hi.v, !iv.hi.closed, let v = step(hi, t, -1) { iv.hi = Edge(v: v, closed: true) }
    return iv
}

private func intervalEmpty(_ iv: Interval) -> Bool {
    guard let lo = iv.lo.v, let hi = iv.hi.v else { return false }
    let c = cmpValue(lo, hi)
    return c > 0 || c == 0 && !(iv.lo.closed && iv.hi.closed)
}

private func lowerMax(_ a: Edge, _ b: Edge) -> Edge {
    guard let av = a.v else { return b }
    guard let bv = b.v else { return a }
    let c = cmpValue(av, bv)
    if c == 0 { return Edge(v: av, closed: a.closed && b.closed) }
    return c > 0 ? a : b
}

private func upperMin(_ a: Edge, _ b: Edge) -> Edge {
    guard let av = a.v else { return b }
    guard let bv = b.v else { return a }
    let c = cmpValue(av, bv)
    if c == 0 { return Edge(v: av, closed: a.closed && b.closed) }
    return c < 0 ? a : b
}

/// Sorts intervals and joins those that overlap or touch, a discrete
/// type's when one ends a step before the next begins.
private func canonical(_ ivs: [Interval], _ t: ExprType) -> [Interval] {
    let kept = ivs.map { normal($0, t) }.filter { !intervalEmpty($0) }
    let sorted = kept.enumerated().sorted { x, y in
        let a = x.element, b = y.element
        var c = 0
        switch (a.lo.v, b.lo.v) {
        case (nil, nil): c = 0
        case (nil, _): c = -1
        case (_, nil): c = 1
        case (let av?, let bv?):
            c = cmpValue(av, bv)
            if c == 0 && a.lo.closed != b.lo.closed { c = a.lo.closed ? -1 : 1 }
        }
        return c != 0 ? c < 0 : x.offset < y.offset
    }.map { $0.element }
    var out: [Interval] = []
    for iv in sorted {
        if let last = out.last, joins(last, iv, t) {
            if iv.hi.v == nil {
                out[out.count - 1].hi = iv.hi
            } else if let lh = last.hi.v, let ih = iv.hi.v, cmpValue(ih, lh) > 0 || cmpValue(ih, lh) == 0 && iv.hi.closed {
                out[out.count - 1].hi = iv.hi
            }
            continue
        }
        out.append(iv)
    }
    return out
}

/// Whether b, which does not begin before a, overlaps or touches it.
private func joins(_ a: Interval, _ b: Interval, _ t: ExprType) -> Bool {
    guard let ah = a.hi.v, let bl = b.lo.v else { return true }
    let c = cmpValue(bl, ah)
    if c < 0 || c == 0 && (a.hi.closed || b.lo.closed) { return true }
    if a.hi.closed && b.lo.closed, let next = step(ah, t, 1), cmpValue(next, bl) == 0 { return true }
    return false
}

private func universe(_ t: ExprType) -> [String] {
    t.kind == .bool ? ["false", "true"] : t.values
}

private func sortedUnion(_ a: [String], _ b: [String]) -> [String] {
    var out = a
    for s in b where !out.contains(s) { out.append(s) }
    return out.sorted(by: byteLess)
}

extension TableColumn {
    func intersect(_ a: ValueSet, _ b: ValueSet) -> ValueSet {
        var out = ValueSet(null: a.null && b.null)
        switch t.kind {
        case .enumValue, .bool:
            out.members = a.members.filter { b.members.contains($0) }
        case .string:
            switch (a.except, b.except) {
            case (false, false): out.texts = a.texts.filter { b.texts.contains($0) }
            case (false, true): out.texts = a.texts.filter { !b.texts.contains($0) }
            case (true, false): out.texts = b.texts.filter { !a.texts.contains($0) }
            case (true, true):
                out.except = true
                out.texts = sortedUnion(a.texts, b.texts)
            }
        default:
            var ivs: [Interval] = []
            for x in a.ivs {
                for y in b.ivs {
                    ivs.append(Interval(lo: lowerMax(x.lo, y.lo), hi: upperMin(x.hi, y.hi)))
                }
            }
            out.ivs = canonical(ivs, t)
        }
        return out
    }

    func union(_ a: ValueSet, _ b: ValueSet) -> ValueSet {
        complement(intersect(complement(a), complement(b)))
    }

    /// Every value of the column's type, and null, that a set does not
    /// hold; not cut to the domain.
    func complement(_ a: ValueSet) -> ValueSet {
        var out = ValueSet(null: !a.null)
        switch t.kind {
        case .enumValue, .bool:
            out.members = universe(t).filter { !a.members.contains($0) }
        case .string:
            out.except = !a.except
            out.texts = a.texts
        default:
            var ivs = [Interval()]
            for x in a.ivs {
                var next: [Interval] = []
                for iv in ivs {
                    if let lo = x.lo.v { next.append(Interval(lo: iv.lo, hi: upperMin(iv.hi, Edge(v: lo, closed: !x.lo.closed)))) }
                    if let hi = x.hi.v { next.append(Interval(lo: lowerMax(iv.lo, Edge(v: hi, closed: !x.hi.closed)), hi: iv.hi)) }
                }
                ivs = canonical(next, t)
            }
            out.ivs = ivs
        }
        return out
    }

    func equal(_ a: ValueSet, _ b: ValueSet) -> Bool {
        intersect(a, complement(b)).isEmpty && intersect(b, complement(a)).isEmpty
    }

    /// Whether a value is in a set.
    func contains(_ s: ValueSet, _ v: ExprValue) -> Bool {
        if v.kind == .null { return s.null }
        switch t.kind {
        case .bool: return s.members.contains(v.bool ? "true" : "false")
        case .enumValue: return s.members.contains(v.text)
        case .string: return s.texts.contains(v.text) != s.except
        default:
            for iv in s.ivs {
                let above = iv.lo.v.map { cmpValue(v, $0) > 0 || iv.lo.closed && cmpValue(v, $0) == 0 } ?? true
                let below = iv.hi.v.map { cmpValue(v, $0) < 0 || iv.hi.closed && cmpValue(v, $0) == 0 } ?? true
                if above && below { return true }
            }
            return false
        }
    }

    /// The value of a set a worked example takes: the lowest member, the
    /// first interval's closed end or a value inside it, and a non-null
    /// value before null.
    func pick(_ s: ValueSet) -> ExprValue {
        if t.kind == .bool, let m = s.members.first {
            var v = ExprValue(.bool)
            v.bool = m == "true"
            return v
        }
        if t.kind == .enumValue, let m = s.members.first {
            var v = ExprValue(.enumValue)
            v.text = m
            return v
        }
        if t.kind == .string && !s.except, let first = s.texts.first {
            var v = ExprValue(.string)
            v.text = first
            return v
        }
        if t.kind == .string && s.except {
            var text = "x"
            while s.texts.contains(text) { text += "x" }
            var v = ExprValue(.string)
            v.text = text
            return v
        }
        if let iv = s.ivs.first {
            var d = ExprValue(.double)
            if let lo = iv.lo.v, iv.lo.closed { return lo }
            if let lo = iv.lo.v, let hi = iv.hi.v {
                d.double = (lo.double + hi.double) / 2
                return d
            }
            if let lo = iv.lo.v {
                d.double = lo.double + 1
                return d
            }
            if let hi = iv.hi.v, iv.hi.closed { return hi }
            if let hi = iv.hi.v {
                d.double = hi.double - 1
                return d
            }
            return zeroValue(t)
        }
        return ExprValue(.null)
    }

    /// Writes a set as a condition: a value, a comparison or a list of
    /// them, in YAML, so that it can be pasted into a rule's when.
    func show(_ s: ValueSet) -> String {
        var parts: [String] = []
        if s.null { parts.append("null") }
        switch t.kind {
        case .enumValue, .bool:
            parts += s.members
        case .string where !s.except:
            parts += s.texts.map { quote($0) }
        case .string:
            let ands = s.texts.map { "!= " + quote($0) }
            if !ands.isEmpty { parts.append(yamlText(ands.joined(separator: " and "))) }
        default:
            for iv in s.ivs {
                if let lo = iv.lo.v, let hi = iv.hi.v, iv.lo.closed && iv.hi.closed && cmpValue(lo, hi) == 0 {
                    parts.append(showValue(lo, t))
                    continue
                }
                var ands: [String] = []
                if let lo = iv.lo.v, domain.ivs.isEmpty || !sameEdge(iv.lo, domain.ivs[0].lo) {
                    ands.append((iv.lo.closed ? ">=" : ">") + " " + showValue(lo, t))
                }
                if var v = iv.hi.v, domain.ivs.isEmpty || !sameEdge(iv.hi, domain.ivs[domain.ivs.count - 1].hi) {
                    var op = "<"
                    if iv.hi.closed {
                        // A number's closed end is written as below the
                        // next value, as a rule writes it: < "5.00", not
                        // <= "4.99".
                        op = "<="
                        if v.kind != .date, let next = step(v, t, 1) {
                            op = "<"
                            v = next
                        }
                    }
                    ands.append(op + " " + showValue(v, t))
                }
                if ands.isEmpty { continue }
                parts.append(yamlText(ands.joined(separator: " and ")))
            }
        }
        return parts.count == 1 ? parts[0] : "[" + parts.joined(separator: ", ") + "]"
    }

    /// The values one comparison holds for, null left out.
    func conditionSet(_ c: TableCondition) -> ValueSet {
        switch t.kind {
        case .enumValue, .bool:
            let name = t.kind == .bool ? (c.v.bool ? "true" : "false") : c.v.text
            var s = ValueSet(members: [name])
            if c.op == "!=" {
                s = complement(s)
                s.null = false
            }
            return s
        case .string:
            return ValueSet(except: c.op == "!=", texts: [c.v.text])
        default:
            let v = c.v
            var ivs: [Interval]
            switch c.op {
            case "<": ivs = [Interval(hi: Edge(v: v, closed: false))]
            case "<=": ivs = [Interval(hi: Edge(v: v, closed: true))]
            case ">": ivs = [Interval(lo: Edge(v: v, closed: false))]
            case ">=": ivs = [Interval(lo: Edge(v: v, closed: true))]
            case "!=": ivs = [Interval(hi: Edge(v: v, closed: false)), Interval(lo: Edge(v: v, closed: false))]
            default: ivs = [Interval(lo: Edge(v: v, closed: true), hi: Edge(v: v, closed: true))]
            }
            return ValueSet(ivs: canonical(ivs, t))
        }
    }

    /// Writes a cell as its rule holds it, for specarch try's steps.
    func text(_ cl: TableCell) -> String {
        let alts = cl.alternatives.map { alt -> String in
            guard let alt else { return "null" }
            return alt.map { $0.op.isEmpty ? showValue($0.v, t) : $0.op + " " + showValue($0.v, t) }.joined(separator: " and ")
        }
        return alts.count == 1 ? alts[0] : "[" + alts.joined(separator: ", ") + "]"
    }
}

private func zeroValue(_ t: ExprType) -> ExprValue {
    var v = ExprValue(t.kind)
    if t.kind == .date { v.seconds = 946684800 } // 2000-01-01
    return v
}

private func sameEdge(_ a: Edge, _ b: Edge) -> Bool {
    guard let av = a.v, let bv = b.v else { return a.v == nil && b.v == nil }
    return a.closed == b.closed && cmpValue(av, bv) == 0
}

/// Quotes a condition as YAML holds it: in double quotes, or in single
/// quotes when it holds a double quote.
private func yamlText(_ s: String) -> String {
    if s.contains("\"") { return "'" + s.replacingOccurrences(of: "'", with: "''") + "'" }
    return "\"" + s + "\""
}

typealias TableBox = [ValueSet]

final class DecisionTable {
    var policy = ""
    var inputs: [TableColumn] = [] // every input, in order
    var columns: [TableColumn] = [] // the inputs the table tests
    var rules: [TableRule] = []
    var readOK = true

    func intersectBox(_ a: TableBox, _ b: TableBox) -> TableBox? {
        var out: TableBox = []
        for k in a.indices {
            let s = columns[k].intersect(a[k], b[k])
            if s.isEmpty { return nil }
            out.append(s)
        }
        return out
    }

    /// A box less another, as boxes apart: for each column, the part of a
    /// outside b there, within b on the columns before it.
    func subtract(_ a: TableBox, _ b: TableBox) -> [TableBox] {
        if intersectBox(a, b) == nil { return [a] }
        var out: [TableBox] = []
        var prefix = a
        for k in a.indices {
            let col = columns[k]
            var piece = prefix
            piece[k] = col.intersect(a[k], col.complement(b[k]))
            if !piece[k].isEmpty { out.append(piece) }
            prefix[k] = col.intersect(a[k], b[k])
        }
        return out
    }

    func subtractAll(_ from: [TableBox], _ b: TableBox) -> [TableBox] {
        from.flatMap { subtract($0, b) }
    }

    /// Writes a box as a when: each input whose values are not all its
    /// domain's.
    func showBox(_ b: TableBox) -> String {
        var parts: [String] = []
        for (k, col) in columns.enumerated() where !col.equal(b[k], col.domain) {
            parts.append(col.name + ": " + col.show(b[k]))
        }
        return parts.isEmpty ? "{ }" : "{ " + parts.joined(separator: ", ") + " }"
    }

    /// Joins boxes that differ on one column only, so a gap reads as the
    /// fewest whens.
    func merge(_ start: [TableBox]) -> [TableBox] {
        var boxes = start
        while true {
            var joined = false
            outer: for i in boxes.indices {
                for j in (i + 1)..<boxes.count {
                    var differ = -1
                    for k in columns.indices where !columns[k].equal(boxes[i][k], boxes[j][k]) {
                        if differ >= 0 {
                            differ = -2
                            break
                        }
                        differ = k
                    }
                    if differ == -2 { continue }
                    if differ >= 0 { boxes[i][differ] = columns[differ].union(boxes[i][differ], boxes[j][differ]) }
                    boxes.remove(at: j)
                    joined = true
                    break outer
                }
            }
            if !joined { return boxes }
        }
    }

    func domainBox() -> TableBox { columns.map { $0.domain } }

    /// The inputs a rule gives the output for: under first, those no
    /// earlier rule matches; under unique, those no other rule matches.
    func reach(_ i: Int) -> [TableBox] {
        if emptyColumn(i) != nil { return [] }
        var region = [rules[i].box]
        for j in rules.indices where j != i && !(policy == "first" && j > i) {
            region = subtractAll(region, rules[j].box)
        }
        return region
    }

    /// The input on which a rule's condition holds for no value the input
    /// may take, or nil.
    func emptyColumn(_ i: Int) -> Int? {
        rules[i].box.firstIndex { $0.isEmpty }
    }

    /// A rule's worked example: inputs it gives the output for, by column,
    /// or nil when there are none.
    func example(_ i: Int) -> [ExprValue]? {
        let region = reach(i)
        guard let first = region.first else { return nil }
        return columns.indices.map { columns[$0].pick(first[$0]) }
    }

    /// The table's output for inputs by name, the rules tested in order
    /// with whether each matched, or the reason there is none.
    func evaluate(_ vals: [String: ExprValue]) -> (ExprValue?, [Bool], String) {
        var tested: [Bool] = []
        var matched: [Int] = []
        for (i, r) in rules.enumerated() {
            var hit = true
            for col in columns {
                if let set = r.when[col.name], !col.contains(set, vals[col.name] ?? ExprValue(.null)) {
                    hit = false
                    break
                }
            }
            tested.append(hit)
            if hit {
                matched.append(i)
                if policy == "first" { break }
            }
        }
        if matched.isEmpty { return (nil, tested, "no rule matches these inputs") }
        if matched.count > 1 {
            return (nil, tested, "rules \(joinNumbers(matched)) match these inputs, and hit policy unique lets only one rule match")
        }
        return (rules[matched[0]].then, tested, "")
    }
}

private func joinNumbers(_ rules: [Int]) -> String {
    let parts = rules.map { "\($0 + 1)" }
    if parts.count == 1 { return parts[0] }
    return parts.dropLast().joined(separator: ", ") + " and " + parts[parts.count - 1]
}

private func orList(_ items: [String]) -> String {
    if items.count == 1 { return items[0] }
    return items.dropLast().joined(separator: ", ") + " or " + items[items.count - 1]
}

private let tableOps = ["<=", ">=", "!=", "<", ">"]

/// Cuts a comparison's value from what follows it: a quoted value runs to
/// its closing quote, a plain one to " and ".
private func splitOperand(_ s: String) -> (String, String) {
    let chars = Array(s)
    var end = -1
    if let q = chars.first, q == "\"" || q == "'" {
        var i = 1
        while i < chars.count {
            if chars[i] == "\\" && q == "\"" {
                i += 2
                continue
            }
            if chars[i] == q {
                end = i + 1
                break
            }
            i += 1
        }
        if end < 0 { return (s, "") }
    } else if let r = s.range(of: " and ") {
        end = s.distance(from: s.startIndex, to: r.lowerBound)
    } else {
        end = chars.count
    }
    let operand = String(chars[..<end]).trimmingCharacters(in: .whitespaces)
    let after = String(chars[end...]).trimmingCharacters(in: .whitespaces)
    if after.isEmpty { return (operand, "") }
    if !after.hasPrefix("and ") { return (operand + " " + after, "") }
    return (operand, String(after.dropFirst(4)).trimmingCharacters(in: .whitespaces))
}

extension Design {
    /// The values an input may take: its type's, within its minimum and
    /// maximum, an int32's range and a uint's zero, and null when it
    /// allows null.
    func tableDomain(_ field: YNode, _ t: ExprType) -> ValueSet {
        var s = ValueSet(null: t.nullable)
        switch t.kind {
        case .enumValue, .bool:
            s.members = universe(t)
        case .string:
            s.except = true
        default:
            var iv = Interval()
            func limit(_ key: String, _ lower: Bool, _ closed: Bool) {
                guard let n = field.child(key), n.kind == .scalar else { return }
                var v = ExprValue(t.kind)
                switch t.kind {
                case .double:
                    guard let f = Double(n.value) else { return }
                    v.double = f
                case .int, .uint, .decimal:
                    guard let d = Decimal(string: n.value, locale: Locale(identifier: "en_US_POSIX")) else { return }
                    v.num = d
                default:
                    return
                }
                let e = Edge(v: v, closed: closed)
                if lower { iv.lo = lowerMax(iv.lo, e) } else { iv.hi = upperMin(iv.hi, e) }
            }
            limit("minimum", true, true)
            limit("exclusiveMinimum", true, false)
            limit("maximum", false, true)
            limit("exclusiveMaximum", false, false)
            if t.kind == .uint {
                iv.lo = lowerMax(iv.lo, Edge(v: ExprValue(.uint), closed: true))
            }
            if t.kind == .int && t.bits == 32 {
                var lo = ExprValue(.int), hi = ExprValue(.int)
                lo.num = Decimal(Int32.min)
                hi.num = Decimal(Int32.max)
                iv.lo = lowerMax(iv.lo, Edge(v: lo, closed: true))
                iv.hi = upperMin(iv.hi, Edge(v: hi, closed: true))
            }
            s.ivs = canonical([iv], t)
        }
        return s
    }

    /// Reads a rule's condition on one input: a value, comparisons joined
    /// by and, null, or a list of these.
    func readCell(_ col: TableColumn, _ n: YNode) -> (TableCell, ValueSet, String) {
        let list = n.kind == .sequence ? n.items : [n]
        var out = TableCell()
        var set = ValueSet()
        for item in list {
            guard item.kind == .scalar else {
                return (TableCell(), ValueSet(), "is not a value, a comparison or null; write one, or a list of them")
            }
            let (alt, s, msg) = readAlternative(col, item)
            if !msg.isEmpty { return (TableCell(), ValueSet(), msg) }
            out.alternatives.append(alt)
            set = col.union(set, s)
        }
        return (out, set, "")
    }

    private func readAlternative(_ col: TableColumn, _ n: YNode) -> ([TableCondition]?, ValueSet, String) {
        if n.tag == "!!null" {
            if !col.t.nullable {
                return (nil, ValueSet(), "is null, but input \(col.name) does not allow null; give a value")
            }
            return (nil, ValueSet(null: true), "")
        }
        let text = n.value.trimmingCharacters(in: .whitespaces)
        let isOp = n.tag == "!!str" && tableOps.contains { text.hasPrefix($0) }
        if !isOp {
            let (v, msg) = value(n, col.field)
            if !msg.isEmpty { return (nil, ValueSet(), msg) }
            let cond = TableCondition(op: "", v: v)
            return ([cond], col.conditionSet(cond), "")
        }
        var conds: [TableCondition] = []
        var set = col.complement(ValueSet(null: true))
        var rest = text
        while true {
            guard let op = tableOps.first(where: { rest.hasPrefix($0) }) else {
                return (nil, ValueSet(), "is \(yamlText(text)), whose part \(quote(rest)) does not begin with <, <=, >, >= or !=; join comparisons with and, such as \">= 3 and < 6\"")
            }
            rest = String(rest.dropFirst(op.count).drop { $0 == " " })
            let (operand, after) = splitOperand(rest)
            if operand.isEmpty {
                return (nil, ValueSet(), "is \(yamlText(text)), whose \(op) has no value after it; write one, such as \"\(op) 6\"")
            }
            let doc = parseYAML(operand)
            guard let vn = doc.root, vn.kind == .scalar, doc.problems.isEmpty else {
                return (nil, ValueSet(), "is \(yamlText(text)), whose value \(operand) cannot be read; write it as a worked example writes it")
            }
            let (v, msg) = value(vn, col.field)
            if !msg.isEmpty { return (nil, ValueSet(), "is \(yamlText(text)), whose value \(msg)") }
            if v.kind == .null {
                return (nil, ValueSet(), "is \(yamlText(text)), which compares with null; write null alone for an input that is null")
            }
            if op != "!=" && !isOrdered(col.t.kind) {
                return (nil, ValueSet(), "is \(yamlText(text)), read as a comparison since it begins with \(op), but \(col.t) has no order, so only a value or != is tested; write the values it may be as a list, and test a value that itself begins with <, > or != with a formula")
            }
            let cond = TableCondition(op: op, v: v)
            conds.append(cond)
            set = col.intersect(set, col.conditionSet(cond))
            if after.isEmpty { break }
            rest = after
        }
        return (conds, set, "")
    }
}

extension Checker {
    /// Reads an algorithm's table, reporting each condition and output
    /// that cannot be read at it.
    func readTable(_ d: Design, _ alg: String, _ a: YNode, report: Bool) -> DecisionTable {
        let tn = a.child("table")
        let tab = DecisionTable()
        tab.policy = str(tn?.child("hitPolicy"))
        let base = ["algorithms", alg, "table"]
        var byName: [String: Int] = [:]
        var inputs: [String: YNode] = [:]
        for p in pairs(a.child("inputs")) {
            let t = d.fieldType(p.value)
            var col = TableColumn(name: p.key.value, field: p.value, t: t, ok: isOrdered(t.kind) || [.enumValue, .bool, .string].contains(t.kind))
            if col.ok {
                col.domain = d.tableDomain(p.value, t)
                if col.domain.isEmpty {
                    tab.readOK = false
                    if report {
                        add(p.value, pointer("algorithms", alg, "inputs", p.key.value), .decisionTable,
                            "input \(p.key.value) may take no value: its minimum and maximum leave none, so no rule of the table can match; correct them")
                    }
                }
            }
            byName[p.key.value] = tab.columns.count
            tab.columns.append(col)
            inputs[p.key.value] = p.value
        }
        if tab.policy != "unique" && tab.policy != "first" { tab.readOK = false }
        let output = a.child("output")
        func fail(_ n: YNode, _ ptr: [String], _ message: String) {
            tab.readOK = false
            if report { add(n, pointer(ptr), .decisionTable, message) }
        }
        for (i, rn) in items(tn?.child("rules")).enumerated() {
            let rb = base + ["rules", "\(i)"]
            var r = TableRule(node: rn)
            for p in pairs(rn.child("when")) {
                let at = rb + ["when", p.key.value]
                guard let k = byName[p.key.value] else {
                    fail(p.key, at, "\(p.key.value) is not an input of algorithm \(alg); remove it or correct its name\(suggest(p.key.value, inputs))")
                    continue
                }
                let col = tab.columns[k]
                if !col.ok {
                    fail(p.value, at, "input \(p.key.value) is \(col.t), which a decision table does not test; a table tests int, uint, decimal, double, date, enum, bool and string inputs, so work this one out with a formula")
                    continue
                }
                let (cl, set, msg) = d.readCell(col, p.value)
                if !msg.isEmpty {
                    fail(p.value, at, "the condition on \(p.key.value) of rule \(i + 1) \(msg)")
                    continue
                }
                r.when[p.key.value] = set
                r.cells.append((name: p.key.value, cell: cl))
                r.cellNodes[p.key.value] = p.value
            }
            if let then = rn.child("then"), let output {
                let (v, msg) = d.value(then, output)
                if !msg.isEmpty { fail(then, rb + ["then"], "the output of rule \(i + 1) \(msg)") }
                r.then = v
            } else {
                tab.readOK = false
            }
            tab.rules.append(r)
        }
        // An input of a type a table does not test, named by no rule, is
        // left out of the columns: every rule matches any value of it.
        tab.inputs = tab.columns
        tab.columns = tab.inputs.filter { $0.ok }
        if !tab.readOK { return tab }
        for i in tab.rules.indices {
            tab.rules[i].box = tab.columns.map { col in
                if let w = tab.rules[i].when[col.name] { return col.intersect(col.domain, w) }
                return col.domain
            }
        }
        return tab
    }

    /// Reads an algorithm's table, checks its rules for overlap and gaps by
    /// its hit policy, and checks its worked examples against it.
    func checkTable(_ d: Design, _ alg: String, _ a: YNode) {
        let tab = readTable(d, alg, a, report: true)
        if !tab.readOK { return }
        let rules = a.child("table")?.child("rules")
        let base = ["algorithms", alg, "table", "rules"]
        for j in tab.rules.indices {
            let ptr = pointer(base + ["\(j)"])
            if let k = tab.emptyColumn(j) {
                let name = tab.columns[k].name
                warn(tab.rules[j].cellNodes[name], pointer(base + ["\(j)", "when", name]), .decisionTable,
                     "rule \(j + 1) matches no inputs: its condition on \(name) holds for no value input \(name) may take, within its type, minimum and maximum; correct the condition, or remove the rule")
                continue
            }
            if tab.policy == "unique" {
                for i in 0..<j {
                    if let both = tab.intersectBox(tab.rules[i].box, tab.rules[j].box) {
                        add(tab.rules[j].node, ptr, .tableOverlap,
                            "rules \(i + 1) and \(j + 1) both match \(tab.showBox(both)), and hit policy unique lets only one rule match any inputs; narrow one of them, or write hitPolicy first if the earlier rule is meant to win")
                    }
                }
                continue
            }
            if tab.reach(j).isEmpty {
                warn(tab.rules[j].node, ptr, .tableOverlap,
                     "rule \(j + 1) is never reached: the rules before it match every input it matches, and hit policy first takes the first rule that matches; remove it, or move it above the rules that take its inputs")
            }
        }
        var left = [tab.domainBox()]
        for r in tab.rules { left = tab.subtractAll(left, r.box) }
        if !left.isEmpty {
            let gaps = tab.merge(left).map { tab.showBox($0) }
            var advice = "add a rule for each"
            if tab.policy == "first" { advice += ", or a last rule with no when for every input the others leave" }
            add(rules, pointer(base), .tableGap, "no rule matches \(orList(gaps)); \(advice)")
        }
        guard let output = a.child("output") else { return }
        let want = d.fieldType(output)
        var inputs: [String: YNode] = [:]
        for col in tab.inputs { inputs[col.name] = col.field }
        for (i, ex) in items(a.child("examples")).enumerated() {
            checkExampleWith(d, alg, i, ex, inputs, output, want, "the table") { vals in
                let (v, _, err) = tab.evaluate(vals)
                if let v { return v }
                throw TableError(message: err)
            }
        }
    }
}

struct TableError: Error, CustomStringConvertible {
    let message: String
    var description: String { message }
}
