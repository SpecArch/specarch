// code-facts-swift reads Swift source by its syntax and writes the facts
// specarch extract swift reads, as a code-facts dump (docs/reading-code.md,
// ADR-087). It is run by tools/code-facts/dump-swift.sh in the root of the
// repository, which gives it the folder's path, the commit and the
// tracked Swift files on standard input, one per line.
//
//   code-facts-swift <path> <commit> < files
//
// The program says what the syntax says and nothing more: imports,
// declarations, stored properties, enum cases, functions, the calls whose
// name is one of the names below, a few assignments and subscripts, each
// with its file, line and column and its literal values. What a fact means is decided by
// specarch extract, in Go, so the rules of extract live in one place.

import Foundation
import SwiftParser
import SwiftSyntax

let parserName = "SwiftSyntax"
let parserVersion = "604.0.0"

// The names of the calls the program writes: the idioms of SwiftUI,
// Foundation and Vapor that specarch extract swift reads.
let callNames: Set<String> = [
    // SwiftUI screens, navigation and fields
    "WindowGroup", "NavigationLink", "navigationDestination", "sheet", "fullScreenCover", "popover",
    "TabView", "Tab", "tabItem", "navigationTitle",
    "TextField", "SecureField", "TextEditor", "Toggle", "Picker", "DatePicker", "Stepper", "Slider",
    // Foundation clients and settings
    "URLRequest", "URL", "data", "dataTask", "upload", "uploadTask", "download", "downloadTask", "object",
    // Vapor routes, groups, middleware, settings and bodies
    "get", "post", "put", "patch", "delete", "on", "grouped", "group", "register", "use", "decode",
]

// The members whose assignment the program writes.
let assignedMembers: Set<String> = [
    "httpMethod", "keyDecodingStrategy", "keyEncodingStrategy", "dateDecodingStrategy", "dateEncodingStrategy",
]

// The subscripts the program writes: a dictionary read by a key.
let subscriptBases = ["infoDictionary", "environment"]

// How deep a call's arguments and closures are written; a literal is
// always written, a call or a closure below this only by its name.
let depth = 3
let textLimit = 160

// JSON is a value written with its keys in the order they were added, so
// the same source gives the same bytes.
indirect enum JSON {
    case string(String)
    case number(String)
    case bool(Bool)
    case array([JSON])
    case object([(String, JSON)])
}

func write(_ value: JSON, into out: inout String) {
    switch value {
    case .string(let s):
        out += "\""
        for u in s.unicodeScalars {
            switch u {
            case "\"": out += "\\\""
            case "\\": out += "\\\\"
            case "\n": out += "\\n"
            case "\r": out += "\\r"
            case "\t": out += "\\t"
            default:
                if u.value < 0x20 {
                    out += String(format: "\\u%04x", u.value)
                } else {
                    out.unicodeScalars.append(u)
                }
            }
        }
        out += "\""
    case .number(let n):
        out += n
    case .bool(let b):
        out += b ? "true" : "false"
    case .array(let items):
        out += "["
        for (i, item) in items.enumerated() {
            if i > 0 { out += ", " }
            write(item, into: &out)
        }
        out += "]"
    case .object(let pairs):
        out += "{"
        for (i, pair) in pairs.enumerated() {
            if i > 0 { out += ", " }
            write(.string(pair.0), into: &out)
            out += ": "
            write(pair.1, into: &out)
        }
        out += "}"
    }
}

struct Fact {
    var file: String
    var line: Int
    var column: Int
    var kind: String
    var fields: [(String, JSON)]

    var json: JSON {
        .object([("kind", .string(kind)), ("file", .string(file)), ("line", .number(String(line))), ("column", .number(String(column)))] + fields)
    }
}

func text(_ node: some SyntaxProtocol) -> String {
    let t = node.trimmedDescription
    if t.count <= textLimit { return t }
    return String(t.prefix(textLimit)) + "…"
}

/// The value of an expression as the syntax gives it: a literal, a name, a
/// call, a closure, or its text.
func encode(_ expr: ExprSyntax, _ left: Int) -> JSON {
    if let s = expr.as(StringLiteralExprSyntax.self) {
        if let v = s.representedLiteralValue {
            return .object([("string", .string(v))])
        }
        var parts: [JSON] = []
        for seg in s.segments {
            switch seg {
            case .stringSegment(let t):
                parts.append(.object([("text", .string(t.content.text))]))
            case .expressionSegment(let e):
                parts.append(.object([("expression", .string(e.expressions.trimmedDescription))]))
            }
        }
        return .object([("interpolated", .array(parts))])
    }
    if let i = expr.as(IntegerLiteralExprSyntax.self) {
        return .object([("integer", .string(i.literal.text))])
    }
    if let f = expr.as(FloatLiteralExprSyntax.self) {
        return .object([("float", .string(f.literal.text))])
    }
    if let b = expr.as(BooleanLiteralExprSyntax.self) {
        return .object([("boolean", .bool(b.literal.tokenKind == .keyword(.true)))])
    }
    if expr.is(NilLiteralExprSyntax.self) {
        return .object([("nil", .bool(true))])
    }
    if let f = expr.as(ForceUnwrapExprSyntax.self) { return encode(f.expression, left) }
    if let o = expr.as(OptionalChainingExprSyntax.self) { return encode(o.expression, left) }
    if let t = expr.as(TryExprSyntax.self) { return encode(t.expression, left) }
    if let a = expr.as(AwaitExprSyntax.self) { return encode(a.expression, left) }
    if let k = expr.as(KeyPathExprSyntax.self) {
        return .object([("keyPath", .string(k.trimmedDescription))])
    }
    if let call = expr.as(FunctionCallExprSyntax.self) {
        return .object([("call", callObject(call, left - 1))])
    }
    if let c = expr.as(ClosureExprSyntax.self) {
        return .object([("closure", closureObject(c, left - 1))])
    }
    if let a = expr.as(ArrayExprSyntax.self) {
        return .object([("array", .array(a.elements.map { encode($0.expression, left) }))])
    }
    if let m = expr.as(MemberAccessExprSyntax.self) {
        if m.declName.baseName.text == "self", let base = m.base {
            return .object([("type", .string(base.trimmedDescription))])
        }
        if m.base == nil {
            return .object([("member", .string("." + m.declName.baseName.text))])
        }
    }
    if let name = dottedName(expr) {
        if name.hasPrefix("$") {
            return .object([("binding", .string(String(name.dropFirst())))])
        }
        return .object([("name", .string(name))])
    }
    return .object([("text", .string(text(expr)))])
}

/// A name, or names joined by dots, such as app or Bundle.main.
func dottedName(_ expr: ExprSyntax) -> String? {
    if let d = expr.as(DeclReferenceExprSyntax.self) {
        return d.baseName.text
    }
    if let m = expr.as(MemberAccessExprSyntax.self), let base = m.base, let b = dottedName(base) {
        return b + "." + m.declName.baseName.text
    }
    return nil
}

/// The called name of a call and the expression it is called on.
func callee(_ call: FunctionCallExprSyntax) -> (String, ExprSyntax?)? {
    var called = call.calledExpression
    if let g = called.as(GenericSpecializationExprSyntax.self) {
        called = g.expression
    }
    if let d = called.as(DeclReferenceExprSyntax.self) {
        return (d.baseName.text, nil)
    }
    if let m = called.as(MemberAccessExprSyntax.self) {
        return (m.declName.baseName.text, m.base)
    }
    return nil
}

func callObject(_ call: FunctionCallExprSyntax, _ left: Int) -> JSON {
    guard let (name, base) = callee(call) else {
        return .object([("text", .string(text(call)))])
    }
    var pairs: [(String, JSON)] = [("name", .string(name))]
    if let base {
        pairs.append(("base", encode(base, min(left, depth - 1))))
    }
    if left <= 0 {
        return .object(pairs)
    }
    let arguments = argumentList(call, left)
    if !arguments.isEmpty {
        pairs.append(("arguments", .array(arguments)))
    }
    let closures = closureList(call, left)
    if !closures.isEmpty {
        pairs.append(("closures", .array(closures)))
    }
    return .object(pairs)
}

func argumentList(_ call: FunctionCallExprSyntax, _ left: Int) -> [JSON] {
    call.arguments.map { arg in
        var pairs: [(String, JSON)] = []
        if let label = arg.label {
            pairs.append(("label", .string(label.text)))
        }
        pairs.append(("value", encode(arg.expression, left)))
        return .object(pairs)
    }
}

func closureList(_ call: FunctionCallExprSyntax, _ left: Int) -> [JSON] {
    var list: [JSON] = []
    if let c = call.trailingClosure {
        list.append(.object([("value", .object([("closure", closureObject(c, left - 1))]))]))
    }
    for extra in call.additionalTrailingClosures {
        list.append(.object([("label", .string(extra.label.text)), ("value", .object([("closure", closureObject(extra.closure, left - 1))]))]))
    }
    return list
}

func closureObject(_ c: ClosureExprSyntax, _ left: Int) -> JSON {
    var pairs: [(String, JSON)] = []
    var names: [JSON] = []
    switch c.signature?.parameterClause {
    case .simpleInput(let list):
        names = list.map { .string($0.name.text) }
    case .parameterClause(let clause):
        names = clause.parameters.map { .string(($0.secondName ?? $0.firstName).text) }
    case nil:
        break
    }
    if !names.isEmpty {
        pairs.append(("parameters", .array(names)))
    }
    if left >= 0 {
        pairs.append(("statements", .array(statements(c.statements, left))))
    }
    return .object(pairs)
}

func statements(_ items: CodeBlockItemListSyntax, _ left: Int) -> [JSON] {
    items.map { item in
        switch item.item {
        case .expr(let e):
            return statement(e, left)
        case .stmt(let s):
            if let e = s.as(ExpressionStmtSyntax.self) {
                return statement(e.expression, left)
            }
            if let r = s.as(ReturnStmtSyntax.self), let e = r.expression {
                return statement(e, left)
            }
            return .object([("text", .string(text(s)))])
        case .decl(let d):
            return .object([("text", .string(text(d)))])
        }
    }
}

func statement(_ e: ExprSyntax, _ left: Int) -> JSON {
    if let i = e.as(IfExprSyntax.self) {
        var branches: [JSON] = [.array(statements(i.body.statements, left))]
        var next = i.elseBody
        while let body = next {
            switch body {
            case .codeBlock(let block):
                branches.append(.array(statements(block.statements, left)))
                next = nil
            case .ifExpr(let more):
                branches.append(.array(statements(more.body.statements, left)))
                next = more.elseBody
            }
        }
        return .object([("branches", .array(branches))])
    }
    if let s = e.as(SwitchExprSyntax.self) {
        var branches: [JSON] = []
        for c in s.cases {
            if case .switchCase(let sc) = c {
                branches.append(.array(statements(sc.statements, left)))
            }
        }
        return .object([("branches", .array(branches))])
    }
    return encode(e, left + 1)
}

func attributeList(_ list: AttributeListSyntax) -> [JSON] {
    list.compactMap { element in
        guard case .attribute(let a) = element else { return nil }
        var pairs: [(String, JSON)] = [("name", .string(a.attributeName.trimmedDescription))]
        if let args = a.arguments {
            pairs.append(("arguments", .string(text(args))))
        }
        return .object(pairs)
    }
}

final class Reader: SyntaxVisitor {
    let file: String
    let converter: SourceLocationConverter
    var facts: [Fact] = []
    var types: [String] = []
    var members: [String] = []

    init(file: String, tree: SourceFileSyntax) {
        self.file = file
        self.converter = SourceLocationConverter(fileName: file, tree: tree)
        super.init(viewMode: .sourceAccurate)
    }

    func add(_ node: some SyntaxProtocol, _ kind: String, _ fields: [(String, JSON)]) {
        let loc = converter.location(for: node.positionAfterSkippingLeadingTrivia)
        facts.append(Fact(file: file, line: loc.line, column: loc.column, kind: kind, fields: fields))
    }

    var within: [(String, JSON)] {
        var pairs: [(String, JSON)] = []
        if !types.isEmpty { pairs.append(("within", .string(types.joined(separator: ".")))) }
        if let m = members.last { pairs.append(("member", .string(m))) }
        return pairs
    }

    func typeFact(_ node: some SyntaxProtocol, _ declaration: String, _ name: String, _ inherits: InheritanceClauseSyntax?, _ attributes: AttributeListSyntax) {
        var fields: [(String, JSON)] = [("declaration", .string(declaration)), ("name", .string(name))]
        if !types.isEmpty { fields.append(("within", .string(types.joined(separator: ".")))) }
        if let inherits {
            fields.append(("inherits", .array(inherits.inheritedTypes.map { .string($0.type.trimmedDescription) })))
        }
        let attrs = attributeList(attributes)
        if !attrs.isEmpty { fields.append(("attributes", .array(attrs))) }
        add(node, "type", fields)
        types.append(name)
    }

    override func visit(_ node: StructDeclSyntax) -> SyntaxVisitorContinueKind {
        typeFact(node, "struct", node.name.text, node.inheritanceClause, node.attributes)
        return .visitChildren
    }
    override func visitPost(_ node: StructDeclSyntax) { types.removeLast() }

    override func visit(_ node: ClassDeclSyntax) -> SyntaxVisitorContinueKind {
        typeFact(node, "class", node.name.text, node.inheritanceClause, node.attributes)
        return .visitChildren
    }
    override func visitPost(_ node: ClassDeclSyntax) { types.removeLast() }

    override func visit(_ node: ActorDeclSyntax) -> SyntaxVisitorContinueKind {
        typeFact(node, "actor", node.name.text, node.inheritanceClause, node.attributes)
        return .visitChildren
    }
    override func visitPost(_ node: ActorDeclSyntax) { types.removeLast() }

    override func visit(_ node: EnumDeclSyntax) -> SyntaxVisitorContinueKind {
        typeFact(node, "enum", node.name.text, node.inheritanceClause, node.attributes)
        return .visitChildren
    }
    override func visitPost(_ node: EnumDeclSyntax) { types.removeLast() }

    override func visit(_ node: ProtocolDeclSyntax) -> SyntaxVisitorContinueKind {
        typeFact(node, "protocol", node.name.text, node.inheritanceClause, node.attributes)
        return .visitChildren
    }
    override func visitPost(_ node: ProtocolDeclSyntax) { types.removeLast() }

    override func visit(_ node: ExtensionDeclSyntax) -> SyntaxVisitorContinueKind {
        typeFact(node, "extension", node.extendedType.trimmedDescription, node.inheritanceClause, node.attributes)
        return .visitChildren
    }
    override func visitPost(_ node: ExtensionDeclSyntax) { types.removeLast() }

    override func visit(_ node: ImportDeclSyntax) -> SyntaxVisitorContinueKind {
        add(node, "import", [("name", .string(node.path.map { $0.name.text }.joined(separator: ".")))])
        return .skipChildren
    }

    override func visit(_ node: FunctionDeclSyntax) -> SyntaxVisitorContinueKind {
        var fields: [(String, JSON)] = []
        if !types.isEmpty { fields.append(("within", .string(types.joined(separator: ".")))) }
        fields.append(("name", .string(node.name.text)))
        let params: [JSON] = node.signature.parameterClause.parameters.map { p in
            var pairs: [(String, JSON)] = [("label", .string(p.firstName.text))]
            pairs.append(("name", .string((p.secondName ?? p.firstName).text)))
            pairs.append(("type", .string(p.type.trimmedDescription)))
            return .object(pairs)
        }
        fields.append(("parameters", .array(params)))
        if node.modifiers.contains(where: { $0.name.text == "static" || $0.name.text == "class" }) {
            fields.append(("static", .bool(true)))
        }
        add(node, "function", fields)
        members.append(node.name.text)
        return .visitChildren
    }
    override func visitPost(_ node: FunctionDeclSyntax) { members.removeLast() }

    override func visit(_ node: InitializerDeclSyntax) -> SyntaxVisitorContinueKind {
        members.append("init")
        return .visitChildren
    }
    override func visitPost(_ node: InitializerDeclSyntax) { members.removeLast() }

    override func visit(_ node: VariableDeclSyntax) -> SyntaxVisitorContinueKind {
        let member = node.parent?.is(MemberBlockItemSyntax.self) ?? false
        var first: String?
        for binding in node.bindings {
            guard let name = binding.pattern.as(IdentifierPatternSyntax.self)?.identifier.text else { continue }
            first = first ?? name
            guard member, !types.isEmpty else { continue }
            let t = types.joined(separator: ".")
            var fields: [(String, JSON)] = [("within", .string(t)), ("name", .string(name)), ("declaration", .string(node.bindingSpecifier.text))]
            if let type = binding.typeAnnotation?.type {
                fields.append(("type", .string(type.trimmedDescription)))
            }
            let attrs = attributeList(node.attributes)
            if !attrs.isEmpty { fields.append(("attributes", .array(attrs))) }
            if node.modifiers.contains(where: { $0.name.text == "static" || $0.name.text == "class" }) {
                fields.append(("static", .bool(true)))
            }
            if let block = binding.accessorBlock {
                var computed = true
                if case .accessors(let list) = block.accessors {
                    computed = list.contains { a in
                        let k = a.accessorSpecifier.text
                        return k != "willSet" && k != "didSet"
                    }
                }
                if computed { fields.append(("computed", .bool(true))) }
            }
            if let value = binding.initializer?.value {
                fields.append(("value", encode(value, 1)))
            }
            add(binding, "property", fields)
        }
        // A property's getter or initial value is read as its own member.
        members.append(member ? (first ?? "") : (members.last ?? ""))
        return .visitChildren
    }
    override func visitPost(_ node: VariableDeclSyntax) { members.removeLast() }

    override func visit(_ node: EnumCaseDeclSyntax) -> SyntaxVisitorContinueKind {
        guard !types.isEmpty else { return .visitChildren }
        let t = types.joined(separator: ".")
        for element in node.elements {
            var fields: [(String, JSON)] = [("within", .string(t)), ("name", .string(element.name.text))]
            if let raw = element.rawValue?.value {
                fields.append(("rawValue", encode(raw, 0)))
            }
            if element.parameterClause != nil {
                fields.append(("associated", .bool(true)))
            }
            add(element, "case", fields)
        }
        return .visitChildren
    }

    override func visit(_ node: FunctionCallExprSyntax) -> SyntaxVisitorContinueKind {
        guard let (name, base) = callee(node), callNames.contains(name) else { return .visitChildren }
        var fields = within
        fields.append(("name", .string(name)))
        if let base {
            fields.append(("base", encode(base, depth - 1)))
        }
        let arguments = argumentList(node, depth)
        if !arguments.isEmpty { fields.append(("arguments", .array(arguments))) }
        let closures = closureList(node, depth)
        if !closures.isEmpty { fields.append(("closures", .array(closures))) }
        if let assigned = assignedTo(Syntax(node)) { fields.append(("assignedTo", .string(assigned))) }
        let (loop, condition) = placed(Syntax(node))
        if loop { fields.append(("inLoop", .bool(true))) }
        if condition { fields.append(("inCondition", .bool(true))) }
        // A modifier's call is placed at its name, not at the start of the
        // chain it is called on.
        if let m = node.calledExpression.as(MemberAccessExprSyntax.self), m.base != nil {
            add(m.declName, "call", fields)
        } else {
            add(node, "call", fields)
        }
        return .visitChildren
    }

    override func visit(_ node: SubscriptCallExprSyntax) -> SyntaxVisitorContinueKind {
        guard let base = dottedName(node.calledExpression) ?? optionalBase(node.calledExpression),
              subscriptBases.contains(where: { base == $0 || base.hasSuffix("." + $0) }),
              let key = node.arguments.first?.expression else { return .visitChildren }
        var fields = within
        fields.append(("base", .object([("name", .string(base))])))
        fields.append(("key", encode(key, 0)))
        add(node, "subscript", fields)
        return .visitChildren
    }

    override func visit(_ node: SequenceExprSyntax) -> SyntaxVisitorContinueKind {
        let elements = Array(node.elements)
        guard elements.count == 3, elements[1].is(AssignmentExprSyntax.self),
              let target = elements[0].as(MemberAccessExprSyntax.self),
              assignedMembers.contains(target.declName.baseName.text) else { return .visitChildren }
        var fields = within
        fields.append(("target", .string(target.trimmedDescription)))
        fields.append(("value", encode(elements[2], 1)))
        add(node, "assignment", fields)
        return .visitChildren
    }

}

/// Counts the places the parser could not read: a token it found missing
/// and text it could not place. Missing tokens are seen only in the view
/// of the tree that holds them.
final class ErrorCounter: SyntaxVisitor {
    var count = 0

    init() { super.init(viewMode: .all) }

    override func visit(_ node: UnexpectedNodesSyntax) -> SyntaxVisitorContinueKind {
        count += 1
        return .skipChildren
    }

    override func visit(_ token: TokenSyntax) -> SyntaxVisitorContinueKind {
        if token.presence == .missing { count += 1 }
        return .visitChildren
    }
}

/// The name of an optional dictionary such as Bundle.main.infoDictionary?.
func optionalBase(_ expr: ExprSyntax) -> String? {
    if let o = expr.as(OptionalChainingExprSyntax.self) { return dottedName(o.expression) }
    if let f = expr.as(ForceUnwrapExprSyntax.self) { return dottedName(f.expression) }
    return nil
}

/// The name a call's value is given: let name = call, or if let and guard
/// let, through try, await and unwrapping.
func assignedTo(_ node: Syntax) -> String? {
    var n = node
    while let p = n.parent, p.is(TryExprSyntax.self) || p.is(AwaitExprSyntax.self) || p.is(ForceUnwrapExprSyntax.self) || p.is(OptionalChainingExprSyntax.self) {
        n = p
    }
    guard let clause = n.parent?.as(InitializerClauseSyntax.self) else { return nil }
    if let binding = clause.parent?.as(PatternBindingSyntax.self) {
        return binding.pattern.as(IdentifierPatternSyntax.self)?.identifier.text
    }
    if let cond = clause.parent?.as(OptionalBindingConditionSyntax.self) {
        return cond.pattern.as(IdentifierPatternSyntax.self)?.identifier.text
    }
    return nil
}

/// Whether a call is in a loop, and whether it is in a branch: the body of
/// an if, a guard's else or a case of a switch, up to the function, the
/// initializer or the accessor it is in. A closure given to forEach is a
/// loop.
func placed(_ node: Syntax) -> (Bool, Bool) {
    var loop = false, condition = false
    var child = node
    while let p = child.parent {
        if p.is(FunctionDeclSyntax.self) || p.is(InitializerDeclSyntax.self) || p.is(AccessorDeclSyntax.self) || p.is(MemberBlockItemSyntax.self) {
            break
        }
        if p.is(ForStmtSyntax.self) || p.is(WhileStmtSyntax.self) || p.is(RepeatStmtSyntax.self) {
            loop = true
        }
        if p.is(SwitchCaseSyntax.self) {
            condition = true
        }
        if let block = p.as(CodeBlockSyntax.self), let owner = block.parent {
            if owner.is(IfExprSyntax.self) || owner.is(GuardStmtSyntax.self) {
                condition = true
            }
        }
        if p.is(ClosureExprSyntax.self), let call = enclosingCall(p), let (name, _) = callee(call), name == "forEach" {
            loop = true
        }
        child = p
    }
    return (loop, condition)
}

/// The call a closure is given to, as its trailing closure or an argument.
func enclosingCall(_ closure: Syntax) -> FunctionCallExprSyntax? {
    var n = closure.parent
    while let p = n {
        if let call = p.as(FunctionCallExprSyntax.self) { return call }
        if p.is(LabeledExprSyntax.self) || p.is(LabeledExprListSyntax.self) || p.is(MultipleTrailingClosureElementSyntax.self) || p.is(MultipleTrailingClosureElementListSyntax.self) {
            n = p.parent
            continue
        }
        return nil
    }
    return nil
}

let args = CommandLine.arguments
guard args.count == 3 else {
    FileHandle.standardError.write("usage: code-facts-swift <path> <commit> < files\n".data(using: .utf8)!)
    exit(2)
}
var files: [String] = []
while let line = readLine() {
    if !line.isEmpty { files.append(line) }
}
files.sort()

var facts: [Fact] = []
var fileList: [JSON] = []
for file in files {
    guard let data = FileManager.default.contents(atPath: file), let source = String(data: data, encoding: .utf8) else {
        FileHandle.standardError.write("code-facts-swift: cannot read \(file) as UTF-8\n".data(using: .utf8)!)
        exit(1)
    }
    let tree = Parser.parse(source: source)
    let reader = Reader(file: file, tree: tree)
    reader.walk(tree)
    let errors = ErrorCounter()
    errors.walk(tree)
    facts += reader.facts
    fileList.append(.object([("file", .string(file)), ("syntaxErrors", .number(String(errors.count)))]))
}
let kindRank = ["import": 0, "type": 0, "function": 1, "property": 2, "case": 3, "call": 4, "subscript": 5, "assignment": 6]
facts = facts.enumerated().sorted { a, b in
    let (x, y) = (a.element, b.element)
    if x.file != y.file { return x.file < y.file }
    if x.line != y.line { return x.line < y.line }
    if x.column != y.column { return x.column < y.column }
    if x.kind != y.kind { return kindRank[x.kind, default: 9] < kindRank[y.kind, default: 9] }
    return a.offset < b.offset
}.map(\.element)

var out = "{\n"
out += "    \"codeFacts\": 1,\n"
out += "    \"language\": \"swift\",\n"
out += "    \"parser\": {\"name\": \"\(parserName)\", \"version\": \"\(parserVersion)\"},\n"
out += "    \"path\": "
write(.string(args[1]), into: &out)
out += ",\n    \"commit\": "
write(.string(args[2]), into: &out)
out += ",\n    \"files\": ["
for (i, f) in fileList.enumerated() {
    out += i == 0 ? "\n        " : ",\n        "
    write(f, into: &out)
}
out += fileList.isEmpty ? "],\n" : "\n    ],\n"
out += "    \"facts\": ["
for (i, f) in facts.enumerated() {
    out += i == 0 ? "\n        " : ",\n        "
    write(f.json, into: &out)
}
out += facts.isEmpty ? "]\n" : "\n    ]\n"
out += "}\n"
FileHandle.standardOutput.write(out.data(using: .utf8)!)
