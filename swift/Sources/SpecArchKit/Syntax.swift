import Foundation

// A YAML syntax error is reported in words the validator owns, the same in
// both builds (ADR-177). The two builds read YAML with different parsers,
// go.yaml.in/yaml/v3 and libyaml through Yams, which come from one source
// and give one set of messages, but not always the same one for the same
// text: a tag that runs into the ] after it is an unclosed [ to one and a
// missing space to the other. So the parser's message picks a class, a
// check of the text settles the cases the parsers tell apart differently,
// and the class's sentence is the message; the parser's own text is kept as
// the problem's detail, printed on standard error. The tables are the Go
// build's (internal/source/syntax.go), entry for entry.

/// Names the YAML parser of this build in a problem's detail.
public let parserName = "libyaml (Yams)"

/// The classes whose line the parser gives as the line where the unclosed
/// block starts, counted from 0.
let startsBlock: [String: Bool] = [
    "unclosed_flow_mapping": true,
    "unclosed_flow_sequence": true,
    "block_mapping_line": true,
    "block_sequence_line": true,
]

/// The class of each message the parsers give.
let syntaxClass: [String: String] = [
    "did not find expected ',' or '}'": "unclosed_flow_mapping",
    "did not find expected ',' or ']'": "unclosed_flow_sequence",
    "did not find expected key": "block_mapping_line",
    "did not find expected '-' indicator": "block_sequence_line",
    "did not find expected node content": "missing_value",
    "mapping values are not allowed in this context": "colon_out_of_place",
    "found unexpected ':'": "colon_out_of_place",
    "mapping keys are not allowed in this context": "question_out_of_place",
    "block sequence entries are not allowed in this context": "dash_out_of_place",
    "could not find expected ':'": "key_without_colon",
    "found character that cannot start any token": "reserved_character",
    "found unexpected end of stream": "unclosed_quote",
    "found unexpected document indicator": "document_marker_in_quote",
    "found unknown escape character": "unknown_escape",
    "did not find expected hexdecimal number": "escape_digits",
    "found invalid Unicode character escape code": "escape_code_point",
    "found an indentation indicator equal to 0": "indentation_indicator",
    "did not find expected comment or line break": "text_after_indicator",
    "did not find expected whitespace or line break": "no_space_after",
    "did not find expected alphabetic or numeric character": "anchor_without_name",
    "found undefined alias": "unknown_alias",
    "found undefined tag handle": "unknown_tag_handle",
    "did not find the expected '>'": "unclosed_verbatim_tag",
    "did not find expected tag URI": "tag_without_name",
    "did not find URI escaped octet": "tag_escape",
    "did not find expected '!'": "unclosed_tag_handle",
    "found a tab character that violates indentation": "tab_indentation",
    "found a tab character where an indentation space is expected": "tab_indentation",
    "control characters are not allowed": "not_text",
    "invalid leading UTF-8 octet": "not_text",
    "invalid trailing UTF-8 octet": "not_text",
    "invalid length of a UTF-8 sequence": "not_text",
    "incomplete UTF-8 octet sequence": "not_text",
    "invalid Unicode character": "not_text",
    "found an incorrect leading UTF-8 octet": "not_text",
    "found an incorrect trailing UTF-8 octet": "not_text",
    "incomplete UTF-16 character": "not_text",
    "incomplete UTF-16 surrogate pair": "not_text",
    "expected low surrogate area": "not_text",
    "invalid character sequence": "not_text",
    "found unknown directive name": "directive",
    "could not find expected directive name": "directive",
    "found duplicate %YAML directive": "directive",
    "found incompatible YAML document": "directive",
    "did not find expected version number": "directive",
    "did not find expected digit or '.' character": "directive",
    "found extremely long version number": "directive",
    "found duplicate %TAG directive": "directive",
    "found unexpected non-alphabetical character": "directive",
    "did not find expected whitespace": "directive",
    "did not find expected <document start>": "no_document_start",
    "Maximum nesting level reached, set with yaml_set_max_nest_level())": "too_deep",
    "input is too long": "too_long",
]

/// The class of a message that names a value of the text.
let syntaxPrefix: [(prefix: String, cls: String)] = [
    ("unknown anchor '", "unknown_alias"),
    ("input error", "not_text"),
]

/// What each class says: what is wrong, then what to do.
let syntaxSentence: [String: String] = [
    "unclosed_flow_mapping": "the { that starts on this line is not closed; close it with }, and separate its entries with commas",
    "unclosed_flow_sequence": "the [ that starts on this line is not closed; close it with ], and separate its items with commas",
    "block_mapping_line": "the mapping that starts on this line has a line that is not a key; check the indentation below it",
    "block_sequence_line": "the list that starts on this line has a line that is not an item; check the indentation below it",
    "tag_runs_on": "a tag in the [ ] or { } that starts on this line runs into the , [ ] { or } written after it, which YAML parsers read in different ways; put a space after the tag",
    "question_in_flow": "a ? in the [ ] or { } that starts on this line stands inside a value, which YAML parsers read in different ways; quote the value, or write a key's ? at the start of an entry with a space after it",
    "missing_value": "a value is missing where one must stand, such as after a , or a [; write the value, or take out the , or [ before the gap",
    "colon_out_of_place": "a colon stands where a key cannot start, such as a second key on one line; put the key on a line of its own, or quote the value that holds the colon",
    "question_out_of_place": "a ? stands where a key cannot start; take it out, or quote the value that holds it",
    "dash_out_of_place": "a - item stands where a list cannot start, such as after a value on its line; put the list on the lines below its key",
    "key_without_colon": "a line has no colon after its key, so it is neither a key nor an item; add the colon, or indent the line under the entry it belongs to",
    "reserved_character": "a value starts with a character YAML keeps for itself, such as @, ` or %, or with a tab; quote the value, or put spaces in place of the tab",
    "unclosed_quote": "a quoted value is not closed; close it with the quote it opens with",
    "document_marker_in_quote": "a line inside a quoted value starts with --- or ..., which ends a document; indent that line",
    "unknown_escape": "a double-quoted value holds a \\ escape YAML does not know; write \\\\ for a backslash, or quote the value with single quotes",
    "escape_digits": "a \\x, \\u or \\U escape in a double-quoted value is not followed by its hexadecimal digits; write the digits, or \\\\ for a backslash",
    "escape_code_point": "a \\u or \\U escape names no Unicode character; write a code point that is one",
    "indentation_indicator": "a block value's indentation indicator is 0; give 1 to 9, or none",
    "text_after_indicator": "text follows where the line must end, such as after a block value's | or >; move it to the lines below, or make it a comment with #",
    "no_space_after": "a tag, anchor or alias is not followed by a space; put a space after it",
    "anchor_without_name": "an anchor (&) or alias (*) has no name; name it with letters, digits, - or _",
    "unknown_alias": "an alias (*name) names no anchor (&name) written before it; write the anchor first, or the value itself",
    "unknown_tag_handle": "a tag's handle (!name!) is defined by no %TAG directive; write the tag with ! or !!, or define the handle",
    "unclosed_verbatim_tag": "a verbatim tag !< is not closed; close it with >",
    "tag_without_name": "a tag has no name after its handle; name it, or take the tag out",
    "tag_escape": "a tag holds a % not followed by two hexadecimal digits; write the digits, or take the % out",
    "unclosed_tag_handle": "a tag's handle is not closed with !; close it",
    "tab_indentation": "a tab stands in the indentation, and YAML indents with spaces only; indent with spaces",
    "not_text": "the file holds a control character or bytes that are not UTF-8; save the file as UTF-8 text and take the character out",
    "directive": "a % directive is not one YAML reads (%YAML 1.1 or 1.2, %TAG with a handle and a prefix, each once); correct it or take it out",
    "no_document_start": "a % directive is not followed by ---; write --- on the line after the directives",
    "too_deep": "the values nest deeper than the parser reads; nest them less deeply",
    "too_long": "the file is longer than the parser reads; split it into files",
    "other": "this is not well-formed YAML; correct it at this line, and read the parser's own message, printed on standard error",
]

/// The class of a parser's message.
func classOf(_ problem: String) -> String {
    if let c = syntaxClass[problem] { return c }
    for p in syntaxPrefix where problem.hasPrefix(p.prefix) { return p.cls }
    return "other"
}

/// The line and the sentence of a parser's message given at a line of the
/// text, given saying whether the parser named a line. For the
/// unclosed-block messages the parser gives the line where the block
/// starts, counted from 0, and names none when that is the first line. The
/// flow collection the line starts is read for the two cases the parsers
/// word differently (flowClass).
func syntaxProblemAt(_ text: String, _ line: Int, _ given: Bool, _ problem: String) -> (Int, String) {
    var cls = classOf(problem)
    var line = line
    if startsBlock[cls] == true {
        line = given ? line + 1 : 1
    }
    switch cls {
    case "unclosed_flow_mapping", "unclosed_flow_sequence", "no_space_after":
        let lines = text.components(separatedBy: "\n")
        let (l, col) = enclosing(lines, line)
        let (c, at) = flowClass(lines, l, col)
        if !c.isEmpty { (cls, line) = (c, at) }
    default:
        break
    }
    return (line, syntaxSentence[cls]!)
}

/// The line and the column (counted in characters from 1) of the innermost
/// [ or { still open where a line with none of its own starts, or the line
/// itself and -1: one parser names the line where the collection starts,
/// the other the line of the tag, and both are read from the collection.
/// The file is read from its top, as flowClass reads a collection, so that
/// a bracket inside a quoted value, a comment or a block scalar is not
/// counted; outside [ ] and { } a bracket or a quote counts only where a
/// value starts.
func enclosing(_ lines: [String], _ line: Int) -> (Int, Int) {
    guard line >= 1 && line <= lines.count && openerAt(lines[line - 1]) < 0 else { return (line, -1) }
    var open: [(line: Int, column: Int)] = []
    var quote: Unicode.Scalar?
    var block = -1 // the indentation a block scalar's lines are deeper than
    var prev: Unicode.Scalar? // the last character that is not a space
    var word: Unicode.Scalar? // the first character of the last word
    for l in 1..<line {
        let r = Array(lines[l - 1].unicodeScalars)
        var indent = 0
        while indent < r.count && isSpace(r[indent]) { indent += 1 }
        if block >= 0 {
            if indent == r.count || indent > block { continue }
            block = -1
        }
        if open.isEmpty && quote == nil { (prev, word) = (nil, nil) }
        var i = 0
        scan: while i < r.count {
            defer { i += 1 }
            let c = r[i]
            if let q = quote {
                if q == "\"" && c == "\\" {
                    i += 1
                } else if c == q && q == "'" && i + 1 < r.count && r[i + 1] == "'" {
                    i += 1
                } else if c == q {
                    quote = nil
                    prev = c
                }
                continue
            }
            if isSpace(c) { continue }
            if c == "#" && (i == 0 || isSpace(r[i - 1])) { break scan }
            var starts = i == 0 || isSpace(r[i - 1])
            if !open.isEmpty { starts = starts || "[{,".unicodeScalars.contains(r[i - 1]) }
            // A value starts after a key's : or ?, a -, [ { or , or after an
            // anchor or a tag.
            let value = starts && (prev == nil || ":?-[{,".unicodeScalars.contains(prev!)
                || ((word == "&" || word == "!") && isSpace(r[i - 1])))
            if starts { word = c }
            if (c == "[" || c == "{") && (!open.isEmpty || value) {
                open.append((l, i + 1))
            } else if (c == "]" || c == "}") && !open.isEmpty {
                open.removeLast()
            } else if (c == "\"" || c == "'") && value {
                quote = c
            } else if (c == "|" || c == ">") && value && open.isEmpty {
                block = indent
                break scan
            }
            prev = c
        }
    }
    guard let last = open.last else { return (line, -1) }
    return (last.line, last.column)
}

/// Where the first [ or { of a line is, before a comment; -1 when there is
/// none.
func openerAt(_ line: String) -> Int {
    let r = Array(line.unicodeScalars)
    var end = commentAt(r)
    if end < 0 { end = r.count }
    for i in 0..<end where r[i] == "[" || r[i] == "{" { return i }
    return -1
}

/// Where a comment starts on a line; -1 when it has none.
func commentAt(_ r: [Unicode.Scalar]) -> Int {
    for (i, c) in r.enumerated() where c == "#" && (i == 0 || isSpace(r[i - 1])) { return i }
    return -1
}

/// Reads every flow collection of a tree that parsed for what the two
/// builds' parsers read differently (flowClass), and returns the first as
/// a problem at the line of the innermost collection that holds it; nil
/// when there is none.
func flowProblem(_ text: String, _ root: YNode) -> Problem? {
    var lines: [String]?
    func walk(_ n: YNode) -> Problem? {
        if n.kind != .scalar {
            if lines == nil { lines = text.components(separatedBy: "\n") }
            // A block mapping starts where its first key does, so a key
            // written as [ ] or { } is found here too; the values after it
            // are read by walking on.
            if let (l, c) = flowStart(lines!, n.line, n.column) {
                let (cls, line) = flowClass(lines!, l, c)
                if !cls.isEmpty {
                    return Problem(line: line, path: "/", rule: "yaml_syntax", message: syntaxSentence[cls]!)
                }
            }
        }
        for (k, v) in n.pairs {
            if let p = walk(k) ?? walk(v) { return p }
        }
        for i in n.items {
            if let p = walk(i) { return p }
        }
        return nil
    }
    return walk(root)
}

/// Where the [ or { of a collection that starts at a line and a column is,
/// past an anchor or a tag written before it; nil when the collection is
/// not written in flow style. The text tells it, in both builds alike.
func flowStart(_ lines: [String], _ line: Int, _ column: Int) -> (Int, Int)? {
    var l = line, i = column - 1
    while l >= 1 && l <= lines.count {
        let r = Array(lines[l - 1].unicodeScalars)
        while i < r.count && isSpace(r[i]) { i += 1 }
        if i >= r.count {
            (l, i) = (l + 1, 0)
            continue
        }
        if r[i] == "&" || r[i] == "!" {
            while i < r.count && !isSpace(r[i]) { i += 1 }
            continue
        }
        return r[i] == "[" || r[i] == "{" ? (l, i + 1) : nil
    }
    return nil
}

/// Reads the flow collection that starts at a line and a column (counted in
/// characters from 1; -1 for its first [ or {, or the line itself when it
/// has none) for what go.yaml.in/yaml/v3 and libyaml read differently, and
/// returns its class and the line of the innermost collection that holds
/// it, or "" when there is none:
///
///   - tag_runs_on: a tag followed by , [ ] { or } with no space between.
///     yaml.v3 reads them as part of the tag; libyaml ends the tag before a
///     , and refuses a ] or }.
///   - question_in_flow: a ? that is not a key's ? (at the start of an
///     entry, followed by a space). yaml.v3 ends a value at any ? in a flow
///     collection and starts a key; libyaml reads it as part of the value.
///
/// Quoted values and comments are skipped.
func flowClass(_ lines: [String], _ line: Int, _ column: Int) -> (String, Int) {
    guard line >= 1 && line <= lines.count else { return ("", line) }
    var start = column - 1
    if column < 0 { start = max(openerAt(lines[line - 1]), 0) }
    var open: [Int] = [] // the line of each collection still open
    var prev: Unicode.Scalar? // the last character that is not a space
    var word: Unicode.Scalar? // the first character of the word before the current one
    var cur: Unicode.Scalar? // the first character of the current word
    var quote: Unicode.Scalar?
    for l in line...lines.count {
        let r = Array(lines[l - 1].unicodeScalars)
        var i = l == line ? start : 0
        while i < r.count {
            defer { i += 1 }
            let c = r[i]
            if let q = quote {
                if q == "\"" && c == "\\" {
                    i += 1
                } else if c == q && q == "'" && i + 1 < r.count && r[i + 1] == "'" {
                    i += 1
                } else if c == q {
                    quote = nil
                    prev = c
                }
                continue
            }
            if isSpace(c) {
                if cur != nil { (word, cur) = (cur, nil) }
                continue
            }
            // A value starts after [ { , or :, or after an anchor or a tag.
            if cur == nil && !"[]{},".unicodeScalars.contains(c) { cur = c }
            let token = (i == 0 || isSpace(r[i - 1]) || "[{,".unicodeScalars.contains(r[i - 1]))
                && (prev == nil || "[{,:".unicodeScalars.contains(prev!) || (cur == c && (word == "&" || word == "!")))
            if c == "#" && (i == 0 || isSpace(r[i - 1])) {
                i = r.count
                continue
            } else if c == "[" || c == "{" {
                open.append(l)
                (word, cur) = (nil, nil)
            } else if c == "]" || c == "}" {
                if open.count <= 1 { return ("", line) }
                open.removeLast()
                (word, cur) = (nil, nil)
            } else if c == "," {
                (word, cur) = (nil, nil)
            } else if (c == "\"" || c == "'") && token {
                quote = c
            } else if c == "!" && token && !(i + 1 < r.count && r[i + 1] == "<") {
                var j = i + 1
                while j < r.count && !isSpace(r[j]) {
                    if ",[]{}".unicodeScalars.contains(r[j]) { return ("tag_runs_on", innermost(open, l)) }
                    j += 1
                }
            } else if c == "?" {
                if !(i + 1 >= r.count || isSpace(r[i + 1])) || !(prev == nil || "[{,".unicodeScalars.contains(prev!)) {
                    return ("question_in_flow", innermost(open, l))
                }
            }
            prev = c
        }
        if open.isEmpty { return ("", line) }
    }
    return ("", line)
}

/// The line of the innermost open collection, or the given line when none
/// is open.
private func innermost(_ open: [Int], _ line: Int) -> Int {
    open.last ?? line
}

private func isSpace(_ c: Unicode.Scalar) -> Bool { c == " " || c == "\t" }

/// The character a byte that is not UTF-8 is read as: one libyaml refuses,
/// as yaml.v3 refuses the byte, so that both builds report the file.
let notUTF8: Unicode.Scalar = "\u{FFFE}"

/// The text of a YAML file, every byte that is not UTF-8 read as notUTF8.
public func yamlText(_ data: Data) -> String {
    var out = String.UnicodeScalarView()
    var it = data.makeIterator()
    var decoder = UTF8()
    var valid = true
    loop: while true {
        switch decoder.decode(&it) {
        case .scalarValue(let v): out.append(v)
        case .emptyInput: break loop
        case .error:
            out.append(notUTF8)
            valid = false
        }
    }
    return valid ? String(decoding: data, as: UTF8.self) : String(out)
}
