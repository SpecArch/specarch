package source

import (
	"strconv"
	"strings"

	"go.yaml.in/yaml/v3"
)

// A YAML syntax error is reported in words the validator owns, the same in
// both builds (ADR-177). The two builds read YAML with different parsers,
// go.yaml.in/yaml/v3 and libyaml through Yams, which come from one source
// and give one set of messages, but not always the same one for the same
// text: a tag that runs into the ] after it is an unclosed [ to one and a
// missing space to the other. So the parser's message picks a class, a
// check of the text settles the cases the parsers tell apart differently,
// and the class's sentence is the message; the parser's own text is kept as
// the problem's detail, printed on standard error.

// ParserName names the YAML parser of this build in a problem's detail.
const ParserName = "go.yaml.in/yaml/v3"

// startsBlock holds the classes whose line the parser gives as the line
// where the unclosed block starts, counted from 0.
var startsBlock = map[string]bool{
	"unclosed_flow_mapping":  true,
	"unclosed_flow_sequence": true,
	"block_mapping_line":     true,
	"block_sequence_line":    true,
}

// syntaxClass is the class of each message the parsers give.
var syntaxClass = map[string]string{
	"did not find expected ',' or '}'":                                   "unclosed_flow_mapping",
	"did not find expected ',' or ']'":                                   "unclosed_flow_sequence",
	"did not find expected key":                                          "block_mapping_line",
	"did not find expected '-' indicator":                                "block_sequence_line",
	"did not find expected node content":                                 "missing_value",
	"mapping values are not allowed in this context":                     "colon_out_of_place",
	"found unexpected ':'":                                               "colon_out_of_place",
	"mapping keys are not allowed in this context":                       "question_out_of_place",
	"block sequence entries are not allowed in this context":             "dash_out_of_place",
	"could not find expected ':'":                                        "key_without_colon",
	"found character that cannot start any token":                        "reserved_character",
	"found unexpected end of stream":                                     "unclosed_quote",
	"found unexpected document indicator":                                "document_marker_in_quote",
	"found unknown escape character":                                     "unknown_escape",
	"did not find expected hexdecimal number":                            "escape_digits",
	"found invalid Unicode character escape code":                        "escape_code_point",
	"found an indentation indicator equal to 0":                          "indentation_indicator",
	"did not find expected comment or line break":                        "text_after_indicator",
	"did not find expected whitespace or line break":                     "no_space_after",
	"did not find expected alphabetic or numeric character":              "anchor_without_name",
	"found undefined alias":                                              "unknown_alias",
	"found undefined tag handle":                                         "unknown_tag_handle",
	"did not find the expected '>'":                                      "unclosed_verbatim_tag",
	"did not find expected tag URI":                                      "tag_without_name",
	"did not find URI escaped octet":                                     "tag_escape",
	"did not find expected '!'":                                          "unclosed_tag_handle",
	"found a tab character that violates indentation":                    "tab_indentation",
	"found a tab character where an indentation space is expected":       "tab_indentation",
	"control characters are not allowed":                                 "not_text",
	"invalid leading UTF-8 octet":                                        "not_text",
	"invalid trailing UTF-8 octet":                                       "not_text",
	"invalid length of a UTF-8 sequence":                                 "not_text",
	"incomplete UTF-8 octet sequence":                                    "not_text",
	"invalid Unicode character":                                          "not_text",
	"found an incorrect leading UTF-8 octet":                             "not_text",
	"found an incorrect trailing UTF-8 octet":                            "not_text",
	"incomplete UTF-16 character":                                        "not_text",
	"incomplete UTF-16 surrogate pair":                                   "not_text",
	"expected low surrogate area":                                        "not_text",
	"invalid character sequence":                                         "not_text",
	"found unknown directive name":                                       "directive",
	"could not find expected directive name":                             "directive",
	"found duplicate %YAML directive":                                    "directive",
	"found incompatible YAML document":                                   "directive",
	"did not find expected version number":                               "directive",
	"did not find expected digit or '.' character":                       "directive",
	"found extremely long version number":                                "directive",
	"found duplicate %TAG directive":                                     "directive",
	"found unexpected non-alphabetical character":                        "directive",
	"did not find expected whitespace":                                   "directive",
	"did not find expected <document start>":                             "no_document_start",
	"Maximum nesting level reached, set with yaml_set_max_nest_level())": "too_deep",
	"input is too long":                                                  "too_long",
}

// syntaxPrefix is the class of a message that names a value of the text.
var syntaxPrefix = []struct{ prefix, class string }{
	{"unknown anchor '", "unknown_alias"},
	{"input error", "not_text"},
}

// syntaxSentence is what each class says: what is wrong, then what to do.
var syntaxSentence = map[string]string{
	"unclosed_flow_mapping":    "the { that starts on this line is not closed; close it with }, and separate its entries with commas",
	"unclosed_flow_sequence":   "the [ that starts on this line is not closed; close it with ], and separate its items with commas",
	"block_mapping_line":       "the mapping that starts on this line has a line that is not a key; check the indentation below it",
	"block_sequence_line":      "the list that starts on this line has a line that is not an item; check the indentation below it",
	"tag_runs_on":              "a tag in the [ ] or { } that starts on this line runs into the , [ ] { or } written after it, which YAML parsers read in different ways; put a space after the tag",
	"question_in_flow":         "a ? in the [ ] or { } that starts on this line stands inside a value, which YAML parsers read in different ways; quote the value, or write a key's ? at the start of an entry with a space after it",
	"missing_value":            "a value is missing where one must stand, such as after a , or a [; write the value, or take out the , or [ before the gap",
	"colon_out_of_place":       "a colon stands where a key cannot start, such as a second key on one line; put the key on a line of its own, or quote the value that holds the colon",
	"question_out_of_place":    "a ? stands where a key cannot start; take it out, or quote the value that holds it",
	"dash_out_of_place":        "a - item stands where a list cannot start, such as after a value on its line; put the list on the lines below its key",
	"key_without_colon":        "a line has no colon after its key, so it is neither a key nor an item; add the colon, or indent the line under the entry it belongs to",
	"reserved_character":       "a value starts with a character YAML keeps for itself, such as @, ` or %, or with a tab; quote the value, or put spaces in place of the tab",
	"unclosed_quote":           "a quoted value is not closed; close it with the quote it opens with",
	"document_marker_in_quote": "a line inside a quoted value starts with --- or ..., which ends a document; indent that line",
	"unknown_escape":           "a double-quoted value holds a \\ escape YAML does not know; write \\\\ for a backslash, or quote the value with single quotes",
	"escape_digits":            "a \\x, \\u or \\U escape in a double-quoted value is not followed by its hexadecimal digits; write the digits, or \\\\ for a backslash",
	"escape_code_point":        "a \\u or \\U escape names no Unicode character; write a code point that is one",
	"indentation_indicator":    "a block value's indentation indicator is 0; give 1 to 9, or none",
	"text_after_indicator":     "text follows where the line must end, such as after a block value's | or >; move it to the lines below, or make it a comment with #",
	"no_space_after":           "a tag, anchor or alias is not followed by a space; put a space after it",
	"anchor_without_name":      "an anchor (&) or alias (*) has no name; name it with letters, digits, - or _",
	"unknown_alias":            "an alias (*name) names no anchor (&name) written before it; write the anchor first, or the value itself",
	"unknown_tag_handle":       "a tag's handle (!name!) is defined by no %TAG directive; write the tag with ! or !!, or define the handle",
	"unclosed_verbatim_tag":    "a verbatim tag !< is not closed; close it with >",
	"tag_without_name":         "a tag has no name after its handle; name it, or take the tag out",
	"tag_escape":               "a tag holds a % not followed by two hexadecimal digits; write the digits, or take the % out",
	"unclosed_tag_handle":      "a tag's handle is not closed with !; close it",
	"tab_indentation":          "a tab stands in the indentation, and YAML indents with spaces only; indent with spaces",
	"not_text":                 "the file holds a control character or bytes that are not UTF-8; save the file as UTF-8 text and take the character out",
	"directive":                "a % directive is not one YAML reads (%YAML 1.1 or 1.2, %TAG with a handle and a prefix, each once); correct it or take it out",
	"no_document_start":        "a % directive is not followed by ---; write --- on the line after the directives",
	"too_deep":                 "the values nest deeper than the parser reads; nest them less deeply",
	"too_long":                 "the file is longer than the parser reads; split it into files",
	"other":                    "this is not well-formed YAML; correct it at this line, and read the parser's own message, printed on standard error",
}

// classOf is the class of a parser's message.
func classOf(problem string) string {
	if c, ok := syntaxClass[problem]; ok {
		return c
	}
	for _, p := range syntaxPrefix {
		if strings.HasPrefix(problem, p.prefix) {
			return p.class
		}
	}
	return "other"
}

// syntaxProblemAt is the line and the sentence of a parser's message given
// at a line of the text, given says whether the parser named a line. For
// the unclosed-block messages the parser gives the line where the block
// starts, counted from 0, and names none when that is the first line. The
// flow collection the line starts is read for the two cases the parsers
// word differently (flowClass).
func syntaxProblemAt(text string, line int, given bool, problem string) (int, string) {
	class := classOf(problem)
	if startsBlock[class] {
		if given {
			line++
		} else {
			line = 1
		}
	}
	switch class {
	case "unclosed_flow_mapping", "unclosed_flow_sequence", "no_space_after":
		lines := strings.Split(text, "\n")
		l, col := enclosing(lines, line)
		if c, at := flowClass(lines, l, col); c != "" {
			class, line = c, at
		}
	}
	return line, syntaxSentence[class]
}

// enclosing is the line and the column (counted in characters from 1) of
// the innermost [ or { still open where a line with none of its own
// starts, or the line itself and -1: one parser names the line where the
// collection starts, the other the line of the tag, and both are read from
// the collection. The file is read from its top, as flowClass reads a
// collection, so that a bracket inside a quoted value, a comment or a
// block scalar is not counted; outside [ ] and { } a bracket or a quote
// counts only where a value starts.
func enclosing(lines []string, line int) (int, int) {
	if line < 1 || line > len(lines) || openerAt(lines[line-1]) >= 0 {
		return line, -1
	}
	type at struct{ line, column int }
	var open []at
	quote := rune(0)
	block := -1     // the indentation a block scalar's lines are deeper than
	prev := rune(0) // the last character that is not a space
	word := rune(0) // the first character of the last word
	for l := 1; l < line; l++ {
		r := []rune(lines[l-1])
		indent := 0
		for indent < len(r) && isSpace(r[indent]) {
			indent++
		}
		if block >= 0 {
			if indent == len(r) || indent > block {
				continue
			}
			block = -1
		}
		if len(open) == 0 && quote == 0 {
			prev, word = 0, 0
		}
		for i := 0; i < len(r); i++ {
			c := r[i]
			if quote != 0 {
				switch {
				case quote == '"' && c == '\\':
					i++
				case c == quote && quote == '\'' && i+1 < len(r) && r[i+1] == '\'':
					i++
				case c == quote:
					quote = 0
					prev = c
				}
				continue
			}
			if isSpace(c) {
				continue
			}
			if c == '#' && (i == 0 || isSpace(r[i-1])) {
				break
			}
			starts := i == 0 || isSpace(r[i-1])
			if len(open) > 0 {
				starts = starts || strings.ContainsRune("[{,", r[i-1])
			}
			// A value starts after a key's : or ?, a -, [ { or , or after an
			// anchor or a tag.
			value := starts && (prev == 0 || strings.ContainsRune(":?-[{,", prev) ||
				((word == '&' || word == '!') && (i == 0 || isSpace(r[i-1]))))
			if starts {
				word = c
			}
			switch {
			case (c == '[' || c == '{') && (len(open) > 0 || value):
				open = append(open, at{l, i + 1})
			case (c == ']' || c == '}') && len(open) > 0:
				open = open[:len(open)-1]
			case (c == '"' || c == '\'') && value:
				quote = c
			case (c == '|' || c == '>') && value && len(open) == 0:
				block = indent
				i = len(r)
				continue
			}
			prev = c
		}
	}
	if len(open) == 0 {
		return line, -1
	}
	return open[len(open)-1].line, open[len(open)-1].column
}

// openerAt is where the first [ or { of a line is, before a comment; -1
// when there is none.
func openerAt(line string) int {
	r := []rune(line)
	end := commentAt(r)
	if end < 0 {
		end = len(r)
	}
	for i, c := range r[:end] {
		if c == '[' || c == '{' {
			return i
		}
	}
	return -1
}

// commentAt is where a comment starts on a line; -1 when it has none.
func commentAt(r []rune) int {
	for i, c := range r {
		if c == '#' && (i == 0 || isSpace(r[i-1])) {
			return i
		}
	}
	return -1
}

// flowProblem reads every flow collection of a tree that parsed for what
// the two builds' parsers read differently (flowClass), and returns the
// first as a problem at the line of the innermost collection that holds
// it; nil when there is none.
func flowProblem(text string, n *yaml.Node) *Problem {
	var lines []string
	var walk func(n *yaml.Node) *Problem
	walk = func(n *yaml.Node) *Problem {
		if n.Kind == yaml.MappingNode || n.Kind == yaml.SequenceNode {
			if lines == nil {
				lines = strings.Split(text, "\n")
			}
			// A block mapping starts where its first key does, so a key
			// written as [ ] or { } is found here too; the values after it
			// are read by walking on.
			if l, col, ok := flowStart(lines, n.Line, n.Column); ok {
				if c, line := flowClass(lines, l, col); c != "" {
					return &Problem{Line: line, Path: "/", Rule: "yaml_syntax", Message: syntaxSentence[c]}
				}
			}
		}
		for _, c := range n.Content {
			if p := walk(c); p != nil {
				return p
			}
		}
		return nil
	}
	return walk(n)
}

// flowStart is where the [ or { of a collection that starts at a line and a
// column is, past an anchor or a tag written before it; ok is false when the
// collection is not written in flow style. The text tells it, in both
// builds alike: the Swift build's parser does not keep a collection's style.
func flowStart(lines []string, line, column int) (int, int, bool) {
	l, i := line, column-1
	for l >= 1 && l <= len(lines) {
		r := []rune(lines[l-1])
		for i < len(r) && isSpace(r[i]) {
			i++
		}
		if i >= len(r) {
			l, i = l+1, 0
			continue
		}
		if r[i] == '&' || r[i] == '!' {
			for i < len(r) && !isSpace(r[i]) {
				i++
			}
			continue
		}
		return l, i + 1, r[i] == '[' || r[i] == '{'
	}
	return 0, 0, false
}

// flowClass reads the flow collection that starts at a line and a column
// (counted in characters from 1; -1 for its first [ or {, or the line
// itself when it has none) for what go.yaml.in/yaml/v3 and libyaml read
// differently, and returns its class and the line of the innermost
// collection that holds it, or "" when there is none:
//
//   - tag_runs_on: a tag followed by , [ ] { or } with no space between.
//     yaml.v3 reads them as part of the tag; libyaml ends the tag before a
//     , and refuses a ] or }.
//   - question_in_flow: a ? that is not a key's ? (at the start of an entry,
//     followed by a space). yaml.v3 ends a value at any ? in a flow
//     collection and starts a key; libyaml reads it as part of the value.
//
// Quoted values and comments are skipped.
func flowClass(lines []string, line, column int) (string, int) {
	if line < 1 || line > len(lines) {
		return "", line
	}
	start := column - 1
	if column < 0 {
		start = max(openerAt(lines[line-1]), 0)
	}
	var open []int  // the line of each collection still open
	prev := rune(0) // the last character that is not a space
	word := rune(0) // the first character of the word before the current one
	cur := rune(0)  // the first character of the current word
	quote := rune(0)
	for l := line; l <= len(lines); l++ {
		r := []rune(lines[l-1])
		i := 0
		if l == line {
			i = start
		}
		for ; i < len(r); i++ {
			c := r[i]
			if quote != 0 {
				switch {
				case quote == '"' && c == '\\':
					i++
				case c == quote && quote == '\'' && i+1 < len(r) && r[i+1] == '\'':
					i++
				case c == quote:
					quote = 0
					prev = c
				}
				continue
			}
			if isSpace(c) {
				if cur != 0 {
					word, cur = cur, 0
				}
				continue
			}
			// A value starts after [ { , or :, or after an anchor or a tag.
			if cur == 0 && !strings.ContainsRune("[]{},", c) {
				cur = c
			}
			token := (i == 0 || isSpace(r[i-1]) || strings.ContainsRune("[{,", r[i-1])) &&
				(prev == 0 || strings.ContainsRune("[{,:", prev) || (cur == c && (word == '&' || word == '!')))
			switch {
			case c == '#' && (i == 0 || isSpace(r[i-1])):
				i = len(r)
				continue
			case c == '[' || c == '{':
				open = append(open, l)
				word, cur = 0, 0
			case c == ']' || c == '}':
				if len(open) <= 1 {
					return "", line
				}
				open = open[:len(open)-1]
				word, cur = 0, 0
			case c == ',':
				word, cur = 0, 0
			case (c == '"' || c == '\'') && token:
				quote = c
			case c == '!' && token && !(i+1 < len(r) && r[i+1] == '<'):
				for j := i + 1; j < len(r) && !isSpace(r[j]); j++ {
					if strings.ContainsRune(",[]{}", r[j]) {
						return "tag_runs_on", innermost(open, l)
					}
				}
			case c == '?':
				if !(i+1 >= len(r) || isSpace(r[i+1])) || !(prev == 0 || strings.ContainsRune("[{,", prev)) {
					return "question_in_flow", innermost(open, l)
				}
			}
			prev = c
		}
		if len(open) == 0 {
			return "", line
		}
	}
	return "", line
}

// innermost is the line of the innermost open collection, or the given
// line when none is open.
func innermost(open []int, line int) int {
	if len(open) == 0 {
		return line
	}
	return open[len(open)-1]
}

func isSpace(c rune) bool { return c == ' ' || c == '\t' }

// lineOf reads the line a parser's error names, and its message without the
// line; 1, and given false, when it names none.
func lineOf(msg string) (line int, problem string, given bool) {
	if m := yamlLine.FindStringSubmatch(msg); m != nil {
		line, _ := strconv.Atoi(m[1])
		return line, m[2], true
	}
	return 1, strings.TrimPrefix(msg, "yaml: "), false
}
