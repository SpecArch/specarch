package facts

import (
	"html"
	"regexp"
	"strings"

	"github.com/SpecArch/specarch/third_party/typescript-go/ast"
	"github.com/SpecArch/specarch/third_party/typescript-go/core"
	"github.com/SpecArch/specarch/third_party/typescript-go/parser"
	"github.com/SpecArch/specarch/third_party/typescript-go/scanner"
)

// The Vue reader the JavaScript reader splits single-file components
// with, as its dumps name it. Its version changes whenever what it reads
// from a component changes, and specarch extract refuses a dump another
// version split (ADR-116).
const (
	TemplateParserName    = "SpecArch Vue reader"
	TemplateParserVersion = "1"
)

// A Vue single-file component is read as Vue 3's compiler-sfc parses it:
// its top-level blocks, the <script> and <script setup> blocks' content
// as their offsets give it, and the <template> block parsed as HTML with
// Vue's directives and interpolations, with every position in UTF-16 code
// units as JavaScript counts them. The problems compiler-sfc counts are
// counted the same way: a block given twice, a tag not closed, an end tag
// with no start, and the tokenizer's errors in a tag.

// sfcDescriptor is a component's blocks.
type sfcDescriptor struct {
	script, scriptSetup *sfcBlock
	template            *vueNode // the root of the template's elements, or nil
	errors              int
}

type sfcBlock struct {
	start, end int // the content's offsets
	lang       string
}

// vueNode is a node of a template: the root (0), an element (1), text
// (2), a comment (3) or an interpolation (5), as Vue numbers them.
type vueNode struct {
	kind     int
	tag      string
	props    []*vueProp
	children []*vueNode
	content  string // text, or an interpolation's expression
	offset   int
	line     int
	column   int
}

// vueProp is an attribute (6) or a directive (7).
type vueProp struct {
	kind     int
	name     string
	rawName  string
	value    *string // an attribute's value
	arg      *string // a directive's argument
	exp      *string // a directive's expression
	modifier []string
}

var htmlVoid = setOf("area", "base", "br", "col", "embed", "hr", "img", "input", "link", "meta", "param", "source", "track", "wbr")

// readVue reads a component's script blocks as one TypeScript or
// JavaScript file in which every other character is a space and every
// line is where it is, so a fact's line and column are the .vue file's
// own.
func readVue(src string) *vueFile {
	u := units(src)
	d := parseSFC(u)
	// One element per character of the source, as the reader on Node
	// builds it, then each of a block's code units put in at its offset.
	var chars [][]uint16
	for _, r := range src {
		switch r {
		case '\n', '\r':
			chars = append(chars, []uint16{uint16(r)})
		default:
			chars = append(chars, []uint16{' '})
		}
	}
	lang := ""
	for _, b := range []*sfcBlock{d.script, d.scriptSetup} {
		if b == nil {
			continue
		}
		for i := b.start; i < b.end && i < len(u); i++ {
			if i < len(chars) {
				chars[i] = []uint16{u[i]}
			} else {
				chars = append(chars, []uint16{u[i]})
			}
		}
		if lang == "" {
			lang = b.lang
		}
	}
	var text []uint16
	for _, c := range chars {
		text = append(text, c...)
	}
	ext := map[string]string{"ts": ".ts", "tsx": ".tsx", "jsx": ".jsx"}[lang]
	if ext == "" {
		ext = ".js"
	}
	return &vueFile{text: fromUnits(text), sfc: d, ext: ext}
}

// vueParser reads a component's source in UTF-16 code units.
type vueParser struct {
	u        []uint16
	i        int
	errors   int
	newlines []int
	// lastInnerEnd is where the content of the element children last
	// closed ends: the offset of its end tag.
	lastInnerEnd int
}

func parseSFC(u []uint16) *sfcDescriptor {
	p := &vueParser{u: u}
	for i, c := range u {
		if c == '\n' {
			p.newlines = append(p.newlines, i)
		}
	}
	d := &sfcDescriptor{}
	var roots []*vueNode
	type rootBlock struct {
		node       *vueNode
		innerStart int
		innerEnd   int
		raw        bool
	}
	var blocks []rootBlock
	for p.i < len(u) {
		if u[p.i] != '<' {
			p.i++
			continue
		}
		if p.startsWith("<!--") {
			p.comment()
			continue
		}
		if p.i+1 < len(u) && u[p.i+1] == '/' {
			// An end tag at the top level with no start tag.
			end := p.indexOf('>', p.i)
			if end < 0 {
				p.i = len(u)
			} else {
				p.i = end + 1
			}
			p.errors++
			continue
		}
		if p.i+1 >= len(u) || !isTagStart(u[p.i+1]) {
			p.i++
			continue
		}
		el, selfClosing := p.startTag()
		if el == nil {
			break
		}
		roots = append(roots, el)
		if selfClosing {
			blocks = append(blocks, rootBlock{node: el, innerStart: p.i, innerEnd: p.i})
			continue
		}
		lang := attr(el, "lang")
		if el.tag == "template" && (lang == nil || *lang == "" || *lang == "html") {
			innerStart := p.i
			el.children = p.children("template", el)
			innerEnd := p.lastInnerEnd
			blocks = append(blocks, rootBlock{node: el, innerStart: innerStart, innerEnd: innerEnd})
			continue
		}
		innerStart := p.i
		end := p.rawUntil(el.tag)
		blocks = append(blocks, rootBlock{node: el, innerStart: innerStart, innerEnd: end, raw: true})
		if end < len(u) {
			el.children = []*vueNode{{kind: 2, content: fromUnits(u[innerStart:end])}}
		}
	}
	for _, b := range blocks {
		el := b.node
		empty := true
		if b.raw {
			empty = strings.TrimSpace(fromUnits(u[b.innerStart:b.innerEnd])) == ""
		} else {
			for _, c := range el.children {
				if c.kind != 2 || strings.TrimSpace(c.content) != "" {
					empty = false
				}
			}
		}
		if el.tag != "template" && empty && attr(el, "src") == nil {
			continue
		}
		block := &sfcBlock{start: b.innerStart, end: b.innerEnd}
		if l := attr(el, "lang"); l != nil && *l != "" {
			block.lang = *l
		}
		switch el.tag {
		case "template":
			if d.template == nil {
				if attr(el, "src") == nil {
					d.template = &vueNode{kind: 0, children: el.children}
				} else {
					d.template = &vueNode{kind: 0}
				}
				if attr(el, "functional") != nil {
					p.errors++
				}
			} else {
				p.errors++
			}
		case "script":
			setup := attr(el, "setup") != nil
			switch {
			case setup && d.scriptSetup == nil:
				d.scriptSetup = block
			case !setup && d.script == nil:
				d.script = block
			default:
				p.errors++
			}
			if setup && d.scriptSetup == block && attr(el, "src") != nil {
				p.errors++
				d.scriptSetup = nil
			}
		case "style":
			if attr(el, "vars") != nil {
				p.errors++
			}
		}
	}
	if d.template == nil && d.script == nil && d.scriptSetup == nil {
		p.errors++
	}
	d.errors = p.errors
	return d
}

func attr(el *vueNode, name string) *string {
	for _, a := range el.props {
		if a.kind == 6 && a.name == name {
			if a.value == nil {
				t := ""
				return &t
			}
			return a.value
		}
	}
	return nil
}

func isTagStart(c uint16) bool { return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' }

func isVueSpace(c uint16) bool { return c == ' ' || c == '\t' || c == '\n' || c == '\r' || c == '\f' }

func isEndOfTagSection(c uint16) bool { return c == '/' || c == '>' || isVueSpace(c) }

func (p *vueParser) startsWith(s string) bool {
	if p.i+len(s) > len(p.u) {
		return false
	}
	for k := 0; k < len(s); k++ {
		if p.u[p.i+k] != uint16(s[k]) {
			return false
		}
	}
	return true
}

func (p *vueParser) indexOf(c uint16, from int) int {
	for k := from; k < len(p.u); k++ {
		if p.u[k] == c {
			return k
		}
	}
	return -1
}

// position is a line and a column, from 1, of an offset.
func (p *vueParser) position(offset int) (int, int) {
	line, lineStart := 1, 0
	for _, n := range p.newlines {
		if n >= offset {
			break
		}
		line++
		lineStart = n + 1
	}
	return line, offset - lineStart + 1
}

func (p *vueParser) comment() {
	end := -1
	for k := p.i + 4; k+2 < len(p.u); k++ {
		if p.u[k] == '-' && p.u[k+1] == '-' && p.u[k+2] == '>' {
			end = k
			break
		}
	}
	if end < 0 {
		p.errors++
		p.i = len(p.u)
		return
	}
	p.i = end + 3
}

// startTag reads a start tag at p.i, which holds '<' and a letter, and
// reports whether it closes itself.
func (p *vueParser) startTag() (*vueNode, bool) {
	start := p.i
	p.i++
	nameStart := p.i
	for p.i < len(p.u) && !isEndOfTagSection(p.u[p.i]) {
		p.i++
	}
	el := &vueNode{kind: 1, tag: fromUnits(p.u[nameStart:p.i]), offset: start}
	el.line, el.column = p.position(start)
	for p.i < len(p.u) {
		c := p.u[p.i]
		switch {
		case c == '>':
			p.i++
			return el, false
		case c == '/':
			p.i++
			if p.i < len(p.u) && p.u[p.i] == '>' {
				p.i++
				return el, true
			}
			p.errors++
		case c == '<' && p.i+1 < len(p.u) && p.u[p.i+1] == '/':
			return el, false
		case isVueSpace(c):
			p.i++
		default:
			if c == '=' {
				p.errors++
			}
			p.attribute(el)
		}
	}
	p.errors++ // the file ends inside the tag
	return el, false
}

// attribute reads an attribute or a directive at p.i.
func (p *vueParser) attribute(el *vueNode) {
	start := p.i
	c := p.u[p.i]
	prop := &vueProp{kind: 6}
	var argStart, argEnd = -1, -1
	switch {
	case c == 'v' && p.i+1 < len(p.u) && p.u[p.i+1] == '-':
		prop.kind = 7
		p.i++
		for p.i < len(p.u) && p.u[p.i] != '=' && !isEndOfTagSection(p.u[p.i]) && p.u[p.i] != ':' && p.u[p.i] != '.' {
			p.i++
		}
		prop.name = fromUnits(p.u[start+2 : p.i])
		if p.i < len(p.u) && p.u[p.i] == ':' {
			p.i++
			argStart = p.i
			argEnd = p.dirArg()
		}
	case c == '.' || c == ':' || c == '@' || c == '#':
		prop.kind = 7
		prop.name = map[uint16]string{'.': "bind", ':': "bind", '@': "on", '#': "slot"}[c]
		if c == '.' {
			prop.modifier = append(prop.modifier, "prop")
		}
		p.i++
		argStart = p.i
		argEnd = p.dirArg()
	default:
		for p.i < len(p.u) && p.u[p.i] != '=' && !isEndOfTagSection(p.u[p.i]) {
			if q := p.u[p.i]; q == '"' || q == '\'' || q == '<' {
				p.errors++
			}
			p.i++
		}
		prop.name = fromUnits(p.u[start:p.i])
	}
	if prop.kind == 7 && prop.name == "" {
		p.errors++
		prop.kind = 6
		prop.name = fromUnits(p.u[start:p.i])
	}
	if prop.kind == 7 {
		if argStart >= 0 && argEnd > argStart {
			arg := fromUnits(p.u[argStart:argEnd])
			if strings.HasPrefix(arg, "[") {
				arg = strings.TrimSuffix(strings.TrimPrefix(arg, "["), "]")
			}
			prop.arg = &arg
		}
		for p.i < len(p.u) && p.u[p.i] == '.' {
			p.i++
			ms := p.i
			for p.i < len(p.u) && p.u[p.i] != '=' && p.u[p.i] != '.' && !isEndOfTagSection(p.u[p.i]) {
				p.i++
			}
			mod := fromUnits(p.u[ms:p.i])
			if prop.name == "slot" && prop.arg != nil {
				a := *prop.arg + "." + mod
				prop.arg = &a
			} else {
				prop.modifier = append(prop.modifier, mod)
			}
		}
	}
	prop.rawName = fromUnits(p.u[start:p.i])
	for _, other := range el.props {
		name := other.name
		if other.kind == 7 {
			name = other.rawName
		}
		if name == prop.rawName {
			p.errors++
		}
	}
	// After the name: white space, then = and a value, or none.
	k := p.i
	for k < len(p.u) && isVueSpace(p.u[k]) {
		k++
	}
	if k < len(p.u) && p.u[k] == '=' {
		p.i = k + 1
		for p.i < len(p.u) && isVueSpace(p.u[p.i]) {
			p.i++
		}
		value, quoted, ok := p.attrValue()
		if !ok {
			el.props = append(el.props, prop)
			return
		}
		if prop.kind == 6 {
			if prop.name == "class" {
				value = strings.TrimFunc(condenseVue(value), func(r rune) bool { return r < 0x80 && isVueSpace(uint16(r)) })
			}
			if !quoted && value == "" {
				p.errors++
			}
			prop.value = &value
		} else {
			prop.exp = &value
			if prop.name != "for" && strings.TrimSpace(value) != "" && !expressionParses(value, prop.name) {
				p.errors++
			}
		}
	}
	el.props = append(el.props, prop)
}

// dirArg reads a directive's argument, static or [dynamic], and returns
// where it ends.
func (p *vueParser) dirArg() int {
	for p.i < len(p.u) {
		c := p.u[p.i]
		if c == '[' {
			p.i++
			for p.i < len(p.u) && p.u[p.i] != ']' {
				if p.u[p.i] == '=' || isEndOfTagSection(p.u[p.i]) {
					p.errors++
					return p.i
				}
				p.i++
			}
			if p.i < len(p.u) {
				p.i++
			}
			continue
		}
		if c == '=' || c == '.' || isEndOfTagSection(c) {
			return p.i
		}
		p.i++
	}
	return p.i
}

// attrValue reads a quoted or an unquoted value, its entities decoded.
func (p *vueParser) attrValue() (string, bool, bool) {
	if p.i >= len(p.u) {
		return "", false, false
	}
	switch q := p.u[p.i]; q {
	case '"', '\'':
		end := p.indexOf(q, p.i+1)
		if end < 0 {
			p.errors++
			v := fromUnits(p.u[p.i+1:])
			p.i = len(p.u)
			return html.UnescapeString(v), true, true
		}
		v := fromUnits(p.u[p.i+1 : end])
		p.i = end + 1
		return html.UnescapeString(v), true, true
	case '>':
		return "", false, true
	}
	start := p.i
	for p.i < len(p.u) && !isVueSpace(p.u[p.i]) && p.u[p.i] != '>' {
		if c := p.u[p.i]; c == '"' || c == '\'' || c == '<' || c == '=' || c == '`' {
			p.errors++
		}
		p.i++
	}
	return html.UnescapeString(fromUnits(p.u[start:p.i])), false, true
}

func condenseVue(s string) string {
	return regexp.MustCompile(`[\t\r\n\f ]+`).ReplaceAllString(s, " ")
}

// rawUntil reads raw text up to the end tag of tag, and returns where
// the end tag starts.
func (p *vueParser) rawUntil(tag string) int {
	for k := p.i; k < len(p.u); k++ {
		if p.u[k] != '<' || k+1 >= len(p.u) || p.u[k+1] != '/' {
			continue
		}
		n := len(tag)
		if k+2+n > len(p.u) {
			break
		}
		if !strings.EqualFold(fromUnits(p.u[k+2:k+2+n]), tag) {
			continue
		}
		if k+2+n < len(p.u) && !isEndOfTagSection(p.u[k+2+n]) {
			continue
		}
		end := p.indexOf('>', k)
		if end < 0 {
			p.i = len(p.u)
		} else {
			p.i = end + 1
		}
		return k
	}
	p.errors++ // the element is not closed
	p.i = len(p.u)
	return len(p.u)
}

// children reads the content of an element up to its end tag: elements,
// text, comments and interpolations, as HTML with Vue's syntax.
func (p *vueParser) children(tag string, el *vueNode) []*vueNode {
	type open struct {
		el       *vueNode
		children []*vueNode
	}
	stack := []*open{{el: el}}
	textStart := p.i
	flushText := func(end int) {
		if end > textStart {
			top := stack[len(stack)-1]
			top.children = append(top.children, &vueNode{kind: 2, content: html.UnescapeString(fromUnits(p.u[textStart:end])), offset: textStart})
		}
	}
	closeTop := func() {
		top := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		top.el.children = top.children
		parent := stack[len(stack)-1]
		parent.children = append(parent.children, top.el)
	}
	for p.i < len(p.u) {
		c := p.u[p.i]
		switch {
		case c == '{' && p.startsWith("{{"):
			end := -1
			for k := p.i + 2; k+1 < len(p.u); k++ {
				if p.u[k] == '}' && p.u[k+1] == '}' {
					end = k
					break
				}
			}
			if end < 0 {
				p.errors++
				p.i = len(p.u)
				continue
			}
			flushText(p.i)
			inner, innerEnd := p.i+2, end
			for inner < innerEnd && isVueSpace(p.u[inner]) {
				inner++
			}
			for innerEnd > inner && isVueSpace(p.u[innerEnd-1]) {
				innerEnd--
			}
			exp := fromUnits(p.u[inner:innerEnd])
			if strings.Contains(exp, "&") {
				exp = html.UnescapeString(exp)
			}
			if strings.TrimSpace(exp) != "" && !expressionParses(exp, "") {
				p.errors++
			}
			top := stack[len(stack)-1]
			n := &vueNode{kind: 5, content: exp, offset: p.i}
			n.line, n.column = p.position(p.i)
			top.children = append(top.children, n)
			p.i = end + 2
			textStart = p.i
		case c == '<' && p.startsWith("<!--"):
			flushText(p.i)
			p.comment()
			textStart = p.i
		case c == '<' && p.i+1 < len(p.u) && p.u[p.i+1] == '/':
			flushText(p.i)
			endStart := p.i
			k := p.i + 2
			ns := k
			for k < len(p.u) && p.u[k] != '>' && !isVueSpace(p.u[k]) {
				k++
			}
			name := fromUnits(p.u[ns:k])
			gt := p.indexOf('>', k)
			if gt < 0 {
				p.i = len(p.u)
			} else {
				p.i = gt + 1
			}
			textStart = p.i
			if name == "" {
				p.errors++
				continue
			}
			found := -1
			for j := len(stack) - 1; j >= 0; j-- {
				if strings.EqualFold(stack[j].el.tag, name) {
					found = j
					break
				}
			}
			if found < 0 {
				p.errors++
				continue
			}
			for len(stack)-1 > found {
				p.errors++ // an element closed by an outer end tag
				closeTop()
			}
			if found == 0 {
				stack[0].el.children = stack[0].children
				p.lastInnerEnd = endStart
				return stack[0].children
			}
			closeTop()
		case c == '<' && p.i+1 < len(p.u) && isTagStart(p.u[p.i+1]):
			flushText(p.i)
			child, selfClosing := p.startTag()
			textStart = p.i
			top := stack[len(stack)-1]
			lower := strings.ToLower(child.tag)
			switch {
			case selfClosing || htmlVoid[lower]:
				top.children = append(top.children, child)
			case lower == "script" || lower == "style" || lower == "textarea" || lower == "title":
				inner := p.i
				end := p.rawUntil(child.tag)
				if end > inner {
					content := fromUnits(p.u[inner:end])
					if lower == "textarea" || lower == "title" {
						content = html.UnescapeString(content)
					}
					child.children = []*vueNode{{kind: 2, content: content, offset: inner}}
				}
				top.children = append(top.children, child)
				textStart = p.i
			default:
				stack = append(stack, &open{el: child})
			}
		default:
			p.i++
		}
	}
	flushText(p.i)
	// The file ends with elements not closed.
	for len(stack) > 1 {
		p.errors++
		closeTop()
	}
	p.errors++ // the block itself is not closed
	stack[0].el.children = stack[0].children
	p.lastInnerEnd = len(p.u)
	return stack[0].children
}

// expressionParses reports whether a directive's or an interpolation's
// expression parses, as Vue parses it when it compiles the template.
func expressionParses(src, directive string) bool {
	text := "(" + src + ")"
	switch directive {
	case "slot":
		text = "(" + src + ") => {}"
	case "on":
		if strings.Contains(src, ";") {
			text = src
		}
	}
	file := parser.ParseSourceFile(ast.SourceFileParseOptions{FileName: "/template.ts", Path: "/template.ts"}, text, core.ScriptKindTS)
	return len(file.Diagnostics()) == 0
}

// templateFacts writes the elements of a Vue template: every component,
// and the heading, form, controls and link of HTML, with their attributes
// and directives, the literal text they hold, the component they are in,
// and whether a v-for or a v-if is above them.
func (r *jsReader) templateFacts(file string, d *sfcDescriptor) {
	if d.template == nil {
		return
	}
	var walk func(n, parent *vueNode, loop, cond bool)
	walk = func(n, parent *vueNode, loop, cond bool) {
		if n.kind == 1 {
			has := func(name string) bool {
				for _, p := range n.props {
					if p.kind == 7 && p.name == name {
						return true
					}
				}
				return false
			}
			inLoop := loop || has("for")
			inCond := cond || has("if") || has("else-if") || has("else") || has("show")
			component := n.tag != "" && (n.tag[0] >= 'A' && n.tag[0] <= 'Z' || strings.Contains(n.tag, "-"))
			next := parent
			if component || intrinsicElements[n.tag] {
				attrs := []any{}
				for _, p := range n.props {
					attrs = append(attrs, templateAttr(p))
				}
				f := newObj("kind", "template", "file", file, "line", n.line, "column", n.column, "element", n.tag, "attributes", attrs)
				children := []any{}
				for _, c := range n.children {
					if c.kind == 2 && strings.TrimFunc(c.content, isJSSpace) != "" {
						children = append(children, newObj("text", strings.TrimFunc(collapse(c.content), isJSSpace)))
					}
					if c.kind == 5 && c.content != "" {
						children = append(children, newObj("expression", templateValue(c.content)))
					}
				}
				if len(children) > 0 {
					f.set("children", children)
				}
				if parent != nil {
					f.set("parent", newObj("line", parent.line, "column", parent.column))
					f.set("parentElement", parent.tag)
				}
				if inLoop {
					f.set("inLoop", true)
				}
				if inCond {
					f.set("inCondition", true)
				}
				r.facts = append(r.facts, f)
				if component {
					next = n
				}
			}
			for _, c := range n.children {
				walk(c, next, inLoop, inCond)
			}
			return
		}
		for _, c := range n.children {
			walk(c, parent, loop, cond)
		}
	}
	walk(d.template, nil, false, false)
}

func scannerStart(n *ast.Node, file *ast.SourceFile) int {
	return scanner.GetTokenPosOfNode(n, file, false)
}

func templateAttr(p *vueProp) *obj {
	if p.kind == 6 {
		if p.value != nil {
			return newObj("name", p.name, "value", newObj("string", *p.value))
		}
		return newObj("name", p.name, "value", newObj("boolean", true))
	}
	a := newObj("directive", p.name)
	if p.arg != nil {
		a.set("name", *p.arg)
	}
	if p.exp != nil {
		a.set("value", templateValue(*p.exp))
	}
	return a
}

// templateValue writes a template's expression by its syntax alone: the
// checker does not see it, so a name is not resolved.
func templateValue(src string) *obj {
	text := "(" + src + ")"
	file := parser.ParseSourceFile(ast.SourceFileParseOptions{FileName: "/template.ts", Path: "/template.ts"}, text, core.ScriptKindTS)
	stmts := file.Statements.Nodes
	if len(stmts) == 0 || !ast.IsExpressionStatement(stmts[0]) {
		return newObj("text", strings.TrimFunc(src, isJSSpace))
	}
	source := func(n *ast.Node) string {
		return text[scannerStart(n, file):n.End()]
	}
	var simple func(n *ast.Node, d int) *obj
	simple = func(n *ast.Node, d int) *obj {
		n = unwrap(n)
		if d > 8 {
			return newObj("text", source(n))
		}
		switch {
		case ast.IsStringLiteral(n) || ast.IsNoSubstitutionTemplateLiteral(n):
			return newObj("string", n.Text())
		case ast.IsNumericLiteral(n):
			return newObj("number", n.Text())
		case n.Kind == ast.KindTrueKeyword:
			return newObj("boolean", true)
		case n.Kind == ast.KindFalseKeyword:
			return newObj("boolean", false)
		case n.Kind == ast.KindNullKeyword:
			return newObj("null", true)
		case ast.IsIdentifier(n):
			return newObj("name", n.Text())
		case ast.IsPropertyAccessExpression(n):
			return newObj("member", newObj("object", simple(n.Expression(), d+1), "name", n.Name().Text()))
		case ast.IsTemplateExpression(n):
			t := n.AsTemplateExpression()
			parts := []any{}
			if t.Head.Text() != "" {
				parts = append(parts, newObj("text", t.Head.Text()))
			}
			for _, s := range nodes(t.TemplateSpans) {
				span := s.AsTemplateSpan()
				parts = append(parts, newObj("expression", simple(span.Expression, d+1)))
				if span.Literal.Text() != "" {
					parts = append(parts, newObj("text", span.Literal.Text()))
				}
			}
			return newObj("template", parts)
		case ast.IsCallExpression(n):
			args := []any{}
			for _, a := range n.Arguments() {
				args = append(args, simple(a, d+1))
			}
			return newObj("call", newObj("callee", simple(n.Expression(), d+1), "arguments", args))
		case ast.IsArrayLiteralExpression(n):
			els := []any{}
			for _, e := range n.Elements() {
				els = append(els, simple(e, d+1))
			}
			return newObj("array", els)
		}
		return newObj("text", source(n))
	}
	return simple(stmts[0].Expression(), 0)
}
