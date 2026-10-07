import Foundation

private let evaluators: (design: SchemaEvaluator?, implementation: SchemaEvaluator?, error: String?) = {
    do {
        return (try SchemaEvaluator(json: designSchemaJSON), try SchemaEvaluator(json: implementationSchemaJSON), nil)
    } catch {
        return (nil, nil, "\(error)")
    }
}()

/// Plain descriptions of the naming patterns in the schemas, so a message
/// can say "camelCase, such as dueOn" instead of quoting a regular
/// expression.
let patternNames: [String: String] = [
    "^[A-Z][A-Za-z0-9]*$": "PascalCase, such as Loan",
    "^[a-z][A-Za-z0-9]*$": "camelCase, such as dueOn",
    "^[a-z][a-z0-9]*(\\.[a-z][a-z0-9]*)*$": "dotted lower case, such as loans.create",
    "^[a-z][a-z0-9]*(-[a-z0-9]+)*$": "kebab-case, such as members-list",
    "^[a-z][a-z0-9]*(_[a-z0-9]+)*$": "snake_case, such as loan_due_after_loaned",
    "^[A-Z][A-Z0-9]{1,15}-[A-Za-z0-9._]+$": "an ID: an upper-case prefix, a dash and a number or name, such as LIB-5 or NEED-1",
    "^[A-Z][A-Z0-9]{0,15}-[A-Za-z0-9._]+$": "a question ID: an upper-case prefix, a dash and a number or name, such as Q-12 or OPEN-3",
    "^[A-Z][A-Z0-9]{1,15}$": "an upper-case prefix of 2 to 16 letters or digits, such as LIB",
    "^ADR-[0-9]{3,}$": "ADR- and three or more digits, such as ADR-001",
    "^[0-9]+\\.[0-9]+\\.[0-9]+(-[0-9A-Za-z.-]+)?$": "a semantic version, such as 1.2.0",
    "^#/(entities|enums)/[A-Z][A-Za-z0-9]*$": "#/entities/Name or #/enums/Name",
    "^/": "a path starting with /",
    "^([1-5][0-9][0-9]|default)$": "an HTTP status code such as 200, or default",
    "^[a-z]+/[a-z0-9.+-]+$": "a media type, such as application/json",
    "^[a-z][a-z0-9]*(\\.[a-z][a-z0-9]*)*/[A-Z][A-Za-z0-9]*$": "channel/Message, such as loan.lifecycle/LoanCreated",
    "^[a-z][a-z0-9]*(-[a-z0-9]+)*( [a-z][a-z0-9]*(-[a-z0-9]+)*)*$": "kebab-case words separated by single spaces, such as generate techspec",
    "^(0|[1-9][0-9]?|1[0-9][0-9]|2[0-4][0-9]|25[0-5])$": "an exit status from 0 to 255",
    "(^|/)specarch\\.yaml$": "a path ending in specarch.yaml, the specification's root file",
    "^[A-Za-z][A-Za-z0-9 ./'-]*$": "a term: letters, digits, spaces, dots, slashes, apostrophes and dashes, such as late fee",
]

func describePattern(_ p: String) -> String {
    patternNames[p] ?? "the pattern " + p
}

extension Checker {
    func checkSchema(_ kind: Kind, _ value: JSONValue) {
        if let err = evaluators.error {
            addLine(1, "/", .schema, "the built-in schema does not compile (\(err)); this is a bug in specarch")
            return
        }
        let evaluator = kind == .implementation ? evaluators.implementation! : evaluators.design!
        var seen = Set<String>()
        for f in evaluator.validate(value) {
            schemaLeaf(kind, f, &seen)
        }
    }

    /// Reports one failure in plain words, once per value and kind of
    /// problem.
    private func schemaLeaf(_ kind: Kind, _ f: SchemaEvaluator.Failure, _ seen: inout Set<String>) {
        let path = pointer(f.location)
        let (node, _) = resolve(root, f.location)
        switch f.kind {
        case let .propertyName(property, causes):
            schemaPropertyName(kind, f.location, property, causes, &seen)
        case let .additionalProperties(props):
            for p in props {
                if kind == .implementation && designKeys.contains(p) { continue }
                let kp = pointer(f.location + [p])
                if seen.contains(kp + "|extra") { continue }
                seen.insert(kp + "|extra")
                add(node.key(p), kp, .schema, "\(p) is not a key this object can have; remove it or correct its spelling")
            }
        case let .required(missing):
            for m in missing {
                let id = path + "|required|" + m
                if seen.contains(id) { continue }
                seen.insert(id)
                add(node, path, .schema, "\(m) is missing; add it here")
            }
        default:
            let id = path + "|" + keyword(f.kind)
            if seen.contains(path + "|any") || seen.contains(id) { return }
            seen.insert(id)
            seen.insert(path + "|any")
            add(node, path, .schema, plainSchemaMessage(f.kind))
        }
    }

    private func schemaPropertyName(_ kind: Kind, _ location: [String], _ property: String, _ causes: [SchemaEvaluator.Failure], _ seen: inout Set<String>) {
        if kind == .design && matches(stackKeyPattern, property) { return }
        let (obj, _) = resolve(root, location)
        let path = pointer(location + [property])
        if seen.contains(path + "|name") { return }
        seen.insert(path + "|name")
        var why = "it is not a name allowed here"
        for c in causes {
            switch c.kind {
            case let .pattern(_, want): why = "it must be " + describePattern(want)
            case let .enumValue(_, want): why = "it must be one of " + joinValues(want)
            default: break
            }
        }
        add(obj.key(property), path, .schema, "the name \(property) is not valid: \(why); rename it")
    }
}

private func keyword(_ k: SchemaEvaluator.Failure.Kind) -> String {
    switch k {
    case .type: return "type"
    case .enumValue: return "enum"
    case .constValue: return "const"
    case .pattern: return "pattern"
    case .format: return "format"
    case .minItems: return "minItems"
    case .maxItems: return "maxItems"
    case .minLength: return "minLength"
    case .minProperties: return "minProperties"
    case .maxProperties: return "maxProperties"
    case .uniqueItems: return "uniqueItems"
    case .minimum: return "minimum"
    case .exclusiveMinimum: return "exclusiveMinimum"
    case .oneOfMany: return "oneOf"
    case .not: return "not"
    case .falseSchema: return ""
    case .contains: return "contains"
    case .required: return "required"
    case .additionalProperties: return "additionalProperties"
    case .propertyName: return "propertyNames"
    }
}

func plainSchemaMessage(_ k: SchemaEvaluator.Failure.Kind) -> String {
    switch k {
    case let .type(got, want):
        return "this is \(article(got)), but \(want.map(article).joined(separator: " or ")) is expected here"
    case let .enumValue(got, want):
        return "\(display(got)) is not allowed here; use one of \(joinValues(want))"
    case let .constValue(want):
        return "this must be \(display(want))"
    case let .pattern(got, want):
        return "\(quote(got)) does not have the right form; it must be \(describePattern(want))"
    case let .format(got, want):
        return "\(display(got)) is not a valid \(formatName(want)); correct it"
    case let .minItems(got, want):
        return want == 1 ? "this list is empty; give at least one item, or leave the key out where that is allowed"
            : "this list has \(got) items and needs at least \(want)"
    case .minLength:
        return "this text is empty; write it, or leave the key out where that is allowed"
    case .minProperties:
        return "this object is empty; give at least one entry"
    case let .maxItems(got, want):
        return "this list has \(got) items and allows at most \(want)"
    case let .maxProperties(got, want):
        return "this object has \(got) entries and allows at most \(want)"
    case let .uniqueItems(i, j):
        return "items \(i) and \(j) are the same; remove the repeat"
    case let .minimum(got, want):
        return "\(got) is below the minimum of \(want)"
    case let .exclusiveMinimum(got, want):
        return "\(got) must be above \(want)"
    case let .oneOfMany(matched):
        return matched.isEmpty ? "this matches none of the allowed forms"
            : "this matches more than one allowed form; write only one of them (for a field, either $ref or type, not both)"
    case .falseSchema:
        return "this key is not allowed here; remove it"
    case .not:
        return "this value is not allowed here"
    case .contains:
        return "this list does not hold the item it needs"
    case .required, .additionalProperties, .propertyName:
        return ""
    }
}

func article(_ t: String) -> String {
    switch t {
    case "integer": return "an integer"
    case "object", "array": return "an " + t
    case "null": return "empty (null)"
    default: return "a " + t
    }
}

func joinValues(_ vs: [JSONValue]) -> String {
    vs.map(display).sorted(by: byteLess).joined(separator: ", ")
}

func display(_ v: JSONValue) -> String {
    switch v {
    case .string(let s): return s
    case .null: return "null"
    case .number(_, let text): return text
    case .bool(let b): return b ? "true" : "false"
    case .array(let a): return "[" + a.map(jsonText).joined(separator: ",") + "]"
    case .object(let o): return "{" + o.map { quote($0.0) + ":" + jsonText($0.1) }.joined(separator: ",") + "}"
    }
}

private func jsonText(_ v: JSONValue) -> String {
    if case .string(let s) = v { return quote(s) }
    return display(v)
}

func formatName(_ f: String) -> String {
    switch f {
    case "date": return "date (write it as \"YYYY-MM-DD\")"
    case "regex": return "regular expression"
    default: return f
    }
}
