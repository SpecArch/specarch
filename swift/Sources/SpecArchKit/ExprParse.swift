import Foundation

/// One problem in an expression. Line and column count from 1, inside the
/// expression text.
public struct ExprError {
    public enum Kind { case syntax, name, typeMismatch }
    public let line: Int
    public let column: Int
    public let kind: Kind
    public let message: String
}

/// One node of a parsed expression.
public final class ExprNode {
    public enum Op: Equatable {
        case int, uint, double, text, bool, null, name, call, neg, not, cond
        case binary(String) // + - * / == != < <= > >= && ||
    }
    public let op: Op
    public var intValue: Decimal = 0 // for int and uint
    public var doubleValue: Double = 0
    public var text = ""            // a string literal, a name, or a function name
    public var boolValue = false
    public var args: [ExprNode] = []
    public let line: Int
    public let column: Int

    init(_ op: Op, line: Int, column: Int) {
        self.op = op
        self.line = line
        self.column = column
    }
}

/// The functions of the subset.
public let exprFunctions: Set<String> = ["int", "uint", "double", "decimal", "string", "date", "timestamp", "duration", "size", "round", "floor", "ceil", "min", "max"]
public let exprFunctionList = "int, uint, double, decimal, string, date, timestamp, duration, size, round, floor, ceil, min and max"

// MARK: - Lexer

private struct Token {
    enum Kind { case ident, int, uint, double, string, bytes, op, eof, bad }
    let kind: Kind
    let text: String
    var intValue: Decimal = 0
    var doubleValue: Double = 0
    let line: Int
    let column: Int
}

private func lex(_ src: String) -> [Token] {
    let s = Array(src.unicodeScalars)
    var i = 0, line = 1, col = 1
    var out: [Token] = []
    func peek(_ k: Int = 0) -> Unicode.Scalar? { i + k < s.count ? s[i + k] : nil }
    func advance() {
        if s[i] == "\n" { line += 1; col = 1 } else { col += 1 }
        i += 1
    }
    func isDigit(_ c: Unicode.Scalar?) -> Bool { c.map { $0 >= "0" && $0 <= "9" } ?? false }
    func isIdentStart(_ c: Unicode.Scalar?) -> Bool {
        guard let c else { return false }
        return c == "_" || (c >= "a" && c <= "z") || (c >= "A" && c <= "Z")
    }
    while i < s.count {
        let c = s[i]
        if c == " " || c == "\t" || c == "\n" || c == "\r" { advance(); continue }
        let tl = line, tc = col
        // strings, with the r and b prefixes
        var prefix = ""
        if (c == "r" || c == "R" || c == "b" || c == "B"), let q = peek(1), q == "\"" || q == "'" {
            prefix = String(c).lowercased()
            advance()
        }
        if let q = peek(), q == "\"" || q == "'" {
            let triple = peek(1) == q && peek(2) == q
            for _ in 0..<(triple ? 3 : 1) { advance() }
            var text = ""
            var closed = false
            while i < s.count {
                if triple {
                    if s[i] == q && peek(1) == q && peek(2) == q { advance(); advance(); advance(); closed = true; break }
                } else if s[i] == q {
                    advance(); closed = true; break
                } else if s[i] == "\n" {
                    break
                }
                if s[i] == "\\" && prefix != "r", let e = peek(1) {
                    advance(); advance()
                    switch e {
                    case "n": text += "\n"
                    case "t": text += "\t"
                    case "r": text += "\r"
                    case "\\": text += "\\"
                    case "\"": text += "\""
                    case "'": text += "'"
                    default: text += "\\" + String(e)
                    }
                    continue
                }
                text.unicodeScalars.append(s[i])
                advance()
            }
            if !closed {
                out.append(Token(kind: .bad, text: String(q), line: tl, column: tc))
                return out
            }
            out.append(Token(kind: prefix == "b" ? .bytes : .string, text: text, line: tl, column: tc))
            continue
        }
        if isIdentStart(c) {
            var text = ""
            while let d = peek(), isIdentStart(d) || isDigit(d) { text.unicodeScalars.append(d); advance() }
            out.append(Token(kind: .ident, text: text, line: tl, column: tc))
            continue
        }
        if isDigit(c) || (c == "." && isDigit(peek(1))) {
            var text = ""
            if c == "0", let x = peek(1), x == "x" || x == "X" {
                advance(); advance()
                while let d = peek(), d.properties.isASCIIHexDigit { text.unicodeScalars.append(d); advance() }
                let v = UInt64(text, radix: 16) ?? 0
                if let u = peek(), u == "u" || u == "U" {
                    advance()
                    out.append(Token(kind: .uint, text: text, intValue: Decimal(string: String(v))!, line: tl, column: tc))
                } else {
                    out.append(Token(kind: .int, text: text, intValue: Decimal(string: String(v))!, line: tl, column: tc))
                }
                continue
            }
            var isFloat = false
            while let d = peek(), isDigit(d) { text.unicodeScalars.append(d); advance() }
            if peek() == ".", isDigit(peek(1)) {
                isFloat = true
                text += "."; advance()
                while let d = peek(), isDigit(d) { text.unicodeScalars.append(d); advance() }
            }
            if let e = peek(), e == "e" || e == "E" {
                let sign = peek(1)
                if isDigit(sign) || ((sign == "+" || sign == "-") && isDigit(peek(2))) {
                    isFloat = true
                    text.unicodeScalars.append(e); advance()
                    if sign == "+" || sign == "-" { text.unicodeScalars.append(sign!); advance() }
                    while let d = peek(), isDigit(d) { text.unicodeScalars.append(d); advance() }
                }
            }
            if isFloat {
                out.append(Token(kind: .double, text: text, doubleValue: Double(text) ?? 0, line: tl, column: tc))
            } else if let u = peek(), u == "u" || u == "U" {
                advance()
                out.append(Token(kind: .uint, text: text, intValue: Decimal(string: text) ?? 0, line: tl, column: tc))
            } else {
                out.append(Token(kind: .int, text: text, intValue: Decimal(string: text) ?? 0, line: tl, column: tc))
            }
            continue
        }
        let two = String(c) + (peek(1).map { String($0) } ?? "")
        if ["==", "!=", "<=", ">=", "&&", "||"].contains(two) {
            advance(); advance()
            out.append(Token(kind: .op, text: two, line: tl, column: tc))
            continue
        }
        if "?:<>+-*/%!()[]{},.".unicodeScalars.contains(c) {
            advance()
            out.append(Token(kind: .op, text: String(c), line: tl, column: tc))
            continue
        }
        out.append(Token(kind: .bad, text: String(c), line: tl, column: tc))
        return out
    }
    out.append(Token(kind: .eof, text: "<EOF>", line: line, column: col))
    return out
}

// MARK: - Parser

private struct SyntaxFailure: Error {
    let token: Token
    let message: String
}

private final class Parser {
    var tokens: [Token]
    var pos = 0
    var refusals: [ExprError] = []

    init(_ tokens: [Token]) { self.tokens = tokens }

    var tok: Token { tokens[pos] }
    func next() -> Token { let t = tokens[pos]; if pos < tokens.count - 1 { pos += 1 }; return t }
    func isOp(_ s: String) -> Bool { tok.kind == .op && tok.text == s }

    func unexpected() -> SyntaxFailure {
        let t = tok
        switch t.kind {
        case .eof:
            return SyntaxFailure(token: t, message: "the expression ends where a value or a closing bracket was expected; complete it")
        case .bad:
            return SyntaxFailure(token: t, message: "\(t.text) is not a character SpecArch expressions use; write * for times, / for divided by, && for and, || for or")
        default:
            return SyntaxFailure(token: t, message: "\(t.kind == .string ? "'" + t.text + "'" : t.text) is not expected here; check the operators and brackets around it")
        }
    }

    func expect(_ s: String) throws {
        guard isOp(s) else { throw unexpected() }
        _ = next()
    }

    func refuse(_ t: Token, _ message: String) {
        refusals.append(ExprError(line: t.line, column: t.column, kind: .syntax, message: message))
    }

    func parse() throws -> ExprNode {
        let e = try expr()
        guard tok.kind == .eof else { throw unexpected() }
        return e
    }

    func expr() throws -> ExprNode {
        let c = try or()
        guard isOp("?") else { return c }
        let q = next()
        let a = try or()
        try expect(":")
        let b = try expr()
        let n = ExprNode(.cond, line: q.line, column: q.column)
        n.args = [c, a, b]
        return n
    }

    func binaryLevel(_ ops: Set<String>, _ lower: () throws -> ExprNode) throws -> ExprNode {
        var left = try lower()
        while tok.kind == .op || (tok.kind == .ident && tok.text == "in") {
            if tok.kind == .ident && tok.text == "in" && ops.contains("in") {
                let t = next()
                _ = try lower()
                refuse(t, "in is not part of SpecArch expressions; compare with == and join with ||")
                continue
            }
            guard tok.kind == .op, ops.contains(tok.text) else { break }
            let t = next()
            let right = try lower()
            if t.text == "%" {
                refuse(t, "% (remainder) is not part of SpecArch expressions")
                continue
            }
            let n = ExprNode(.binary(t.text), line: t.line, column: t.column)
            n.args = [left, right]
            left = n
        }
        return left
    }

    func or() throws -> ExprNode { try binaryLevel(["||"], and) }
    func and() throws -> ExprNode { try binaryLevel(["&&"], relation) }
    func relation() throws -> ExprNode { try binaryLevel(["==", "!=", "<", "<=", ">", ">=", "in"], sum) }
    func sum() throws -> ExprNode { try binaryLevel(["+", "-"], product) }
    func product() throws -> ExprNode { try binaryLevel(["*", "/", "%"], unary) }

    func unary() throws -> ExprNode {
        if isOp("!") {
            let t = next()
            let n = ExprNode(.not, line: t.line, column: t.column)
            n.args = [try unary()]
            return n
        }
        if isOp("-") {
            let t = next()
            let operand = try unary()
            // A minus right before a number is part of the number, as CEL reads it.
            if operand.op == .int && tokens[pos - 1].kind == .int && operand.column == t.column + 1 {
                let n = ExprNode(.int, line: t.line, column: t.column)
                n.intValue = -operand.intValue
                return n
            }
            if operand.op == .double && operand.column == t.column + 1 {
                let n = ExprNode(.double, line: t.line, column: t.column)
                n.doubleValue = -operand.doubleValue
                return n
            }
            let n = ExprNode(.neg, line: t.line, column: t.column)
            n.args = [operand]
            return n
        }
        return try member()
    }

    func member() throws -> ExprNode {
        let n = try primary()
        while true {
            if isOp(".") {
                let dot = next()
                guard tok.kind == .ident else { throw unexpected() }
                let name = next()
                if isOp("(") {
                    let open = next()
                    _ = try arguments()
                    refuse(open, "the method form x.\(name.text)() is not part of SpecArch expressions; write \(name.text)(x)")
                } else {
                    refuse(dot, "field access with . is not part of SpecArch expressions; name the field directly")
                }
                continue
            }
            if isOp("[") {
                let open = next()
                _ = try expr()
                try expect("]")
                refuse(open, "indexing with [ ] is not part of SpecArch expressions")
                continue
            }
            return n
        }
    }

    func arguments() throws -> [ExprNode] {
        var args: [ExprNode] = []
        if isOp(")") { _ = next(); return args }
        while true {
            args.append(try expr())
            if isOp(",") { _ = next(); continue }
            try expect(")")
            return args
        }
    }

    func primary() throws -> ExprNode {
        let t = tok
        switch t.kind {
        case .int, .uint:
            _ = next()
            let n = ExprNode(t.kind == .int ? .int : .uint, line: t.line, column: t.column)
            n.intValue = t.intValue
            return n
        case .double:
            _ = next()
            let n = ExprNode(.double, line: t.line, column: t.column)
            n.doubleValue = t.doubleValue
            return n
        case .string:
            _ = next()
            let n = ExprNode(.text, line: t.line, column: t.column)
            n.text = t.text
            return n
        case .bytes:
            _ = next()
            refuse(t, "byte strings (written b\"...\") are not part of SpecArch expressions")
            return ExprNode(.null, line: t.line, column: t.column)
        case .ident:
            _ = next()
            switch t.text {
            case "true", "false":
                let n = ExprNode(.bool, line: t.line, column: t.column)
                n.boolValue = t.text == "true"
                return n
            case "null":
                return ExprNode(.null, line: t.line, column: t.column)
            default:
                break
            }
            if isOp("(") {
                let open = next()
                let args = try arguments()
                if !exprFunctions.contains(t.text) {
                    refuse(open, "\(t.text) is not a function of SpecArch expressions; the functions are \(exprFunctionList)")
                }
                let n = ExprNode(.call, line: open.line, column: open.column)
                n.text = t.text
                n.args = args
                return n
            }
            let n = ExprNode(.name, line: t.line, column: t.column)
            n.text = t.text
            return n
        case .op where t.text == "(":
            _ = next()
            let e = try expr()
            try expect(")")
            return e
        case .op where t.text == "[":
            _ = next()
            if !isOp("]") {
                while true {
                    _ = try expr()
                    if isOp(",") { _ = next(); continue }
                    break
                }
            }
            try expect("]")
            refuse(t, "lists [ ] are not part of SpecArch expressions")
            return ExprNode(.null, line: t.line, column: t.column)
        case .op where t.text == "{":
            _ = next()
            var depth = 1
            while depth > 0 && tok.kind != .eof {
                if isOp("{") { depth += 1 }
                if isOp("}") { depth -= 1 }
                _ = next()
            }
            if depth > 0 { throw unexpected() }
            refuse(t, "maps and objects { } are not part of SpecArch expressions")
            return ExprNode(.null, line: t.line, column: t.column)
        default:
            throw unexpected()
        }
    }
}

/// Parses one expression of the subset. Anything outside the subset is
/// refused by name.
public func parseExpr(_ src: String) -> (ExprNode?, [ExprError]) {
    if src.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty {
        return (nil, [ExprError(line: 1, column: 1, kind: .syntax, message: "the expression is empty; write one")])
    }
    let p = Parser(lex(src))
    do {
        let n = try p.parse()
        if !p.refusals.isEmpty { return (nil, p.refusals) }
        return (n, [])
    } catch let f as SyntaxFailure {
        return (nil, [ExprError(line: f.token.line, column: f.token.column, kind: .syntax, message: f.message)])
    } catch {
        return (nil, [ExprError(line: 1, column: 1, kind: .syntax, message: "\(error)")])
    }
}
