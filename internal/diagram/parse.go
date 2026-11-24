// Package diagram draws the Mermaid diagrams the documents carry for
// print: it reads the part of Mermaid that SpecArch's document targets
// write (flowchart, stateDiagram-v2, erDiagram and sequenceDiagram), lays
// each out in points with the metrics of the fonts it is set in, and
// returns the shapes and texts to draw. The pdf writer draws them; the
// Markdown keeps the Mermaid source for the viewers that draw it.
//
// A line it cannot read is an error that names the line, never a line
// dropped: the writer then prints the source with that reason.
package diagram

import (
	"fmt"
	"strings"
	"unicode"
)

// Kind is the kind of a diagram, from its first line.
type Kind int

const (
	Flowchart Kind = iota
	State
	Entity
	Sequence
)

// Shape is the outline of a node.
type Shape int

const (
	Rect          Shape = iota // id["text"]
	Rounded                    // id("text"), and every state
	Stadium                    // id(["text"])
	Circle                     // id(("text"))
	Cylinder                   // id[("text")]
	Subroutine                 // id[["text"]]
	Diamond                    // id{"text"}
	Hexagon                    // id{{"text"}}
	Parallelogram              // id[/"text"/]
	Trapezoid                  // id[/"text"\]
	Asymmetric                 // id>"text"]
	Start                      // [*] that an arrow leaves
	End                        // [*] that an arrow reaches
	Table                      // an entity with its attributes
)

// Diagram is one Mermaid diagram as read.
type Diagram struct {
	Kind   Kind
	Dir    string // LR or TD for a flowchart; empty for the others
	Nodes  []*Node
	Edges  []*Edge
	Groups []*Group

	Participants []*Participant
	Steps        []*Step

	byID       map[string]*Node
	groupNow   int   // the subgraph being read, -1 for none
	entityOpen *Node // the entity whose attributes are being read
}

// Node is a node of a flowchart, a state or an entity.
type Node struct {
	ID    string
	Label string
	Shape Shape
	Group int // index in Groups, -1 for none
	Attrs []Attr
	given bool // its label or shape was given, not only its id
}

// Attr is one attribute of an entity.
type Attr struct{ Type, Name, Keys string }

// Edge is an arrow, or a relation of an entity diagram.
type Edge struct {
	From, To string
	Label    string
	Dashed   bool
	Head     bool      // an arrowhead at To
	Card     [2]string // an entity relation's cardinality at From and at To
}

// Group is a subgraph.
type Group struct{ ID, Title string }

// Participant is a lifeline of a sequence diagram.
type Participant struct{ ID, Label string }

// Step is a message or a note of a sequence diagram, in order.
type Step struct {
	Note     bool
	From, To string // a note's first and last participant
	Text     string
	Dashed   bool
	Head     bool
	Place    string // a note's place: over, left of, right of
}

// Parse reads a Mermaid diagram of the kinds this package draws.
func Parse(src string) (*Diagram, error) {
	lines := strings.Split(strings.ReplaceAll(src, "\r\n", "\n"), "\n")
	first := -1
	for i, l := range lines {
		if t := strings.TrimSpace(l); t != "" && !strings.HasPrefix(t, "%%") {
			first = i
			break
		}
	}
	if first < 0 {
		return nil, fmt.Errorf("the diagram is empty")
	}
	head := strings.Fields(lines[first])
	d := &Diagram{byID: map[string]*Node{}}
	var read func(n int, line string) error
	switch head[0] {
	case "flowchart", "graph":
		d.Kind, d.Dir = Flowchart, "TD"
		if len(head) > 1 {
			switch head[1] {
			case "LR", "RL":
				d.Dir = "LR"
			case "TD", "TB", "BT":
			default:
				return nil, fmt.Errorf("line %d: %q is not a direction this writer draws (LR, RL, TD, TB, BT)", first+1, head[1])
			}
		}
		read = d.flowLine
	case "stateDiagram-v2", "stateDiagram":
		d.Kind = State
		read = d.stateLine
	case "erDiagram":
		d.Kind = Entity
		read = d.entityLine
	case "sequenceDiagram":
		d.Kind = Sequence
		read = d.sequenceLine
	default:
		return nil, fmt.Errorf("line %d: %q is not a diagram kind this writer draws (flowchart, stateDiagram-v2, erDiagram, sequenceDiagram)", first+1, head[0])
	}
	d.groupNow = -1
	for i := first + 1; i < len(lines); i++ {
		t := strings.TrimSpace(lines[i])
		if t == "" || strings.HasPrefix(t, "%%") {
			continue
		}
		if err := read(i+1, t); err != nil {
			return nil, fmt.Errorf("line %d, %q: %v", i+1, t, err)
		}
	}
	if d.entityOpen != nil {
		return nil, fmt.Errorf("the attributes of entity %s are not closed with }", d.entityOpen.ID)
	}
	if d.groupNow >= 0 {
		return nil, fmt.Errorf("subgraph %s is not closed with end", d.Groups[d.groupNow].ID)
	}
	if len(d.Nodes) == 0 && len(d.Participants) == 0 {
		return nil, fmt.Errorf("the diagram has nothing to draw")
	}
	return d, nil
}

// node is the node of an id, made on first mention.
func (d *Diagram) node(id string) *Node {
	if n := d.byID[id]; n != nil {
		return n
	}
	n := &Node{ID: id, Label: id, Group: d.groupNow}
	d.byID[id] = n
	d.Nodes = append(d.Nodes, n)
	return n
}

// label turns the text of a Mermaid label into the text drawn: a <br/> is
// a line break, and the entities Mermaid knows are their characters.
func label(s string) string {
	s = strings.TrimSpace(s)
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		s = s[1 : len(s)-1]
	}
	s = strings.NewReplacer("<br/>", "\n", "<br>", "\n", "<br />", "\n", "#quot;", `"`, "#amp;", "&", "#lt;", "<", "#gt;", ">", "#35;", "#", "#59;", ";").Replace(s)
	return s
}

// The flowchart.

// shapes are the node shapes by their opening and closing marks, the
// longer openings first.
var shapes = []struct {
	open, close string
	shape       Shape
}{
	{"([", "])", Stadium},
	{"((", "))", Circle},
	{"[(", ")]", Cylinder},
	{"[[", "]]", Subroutine},
	{"[/", "/]", Parallelogram},
	{"[/", `\]`, Trapezoid},
	{`[\`, `\]`, Parallelogram},
	{`[\`, "/]", Trapezoid},
	{"{{", "}}", Hexagon},
	{"{", "}", Diamond},
	{"(", ")", Rounded},
	{"[", "]", Rect},
	{">", "]", Asymmetric},
}

func (d *Diagram) flowLine(_ int, t string) error {
	f := strings.Fields(t)
	switch f[0] {
	case "subgraph":
		if d.groupNow >= 0 {
			return fmt.Errorf("a subgraph inside a subgraph is not drawn by this writer")
		}
		rest := strings.TrimSpace(strings.TrimPrefix(t, "subgraph"))
		id, title := rest, rest
		if i := strings.IndexAny(rest, "[ "); i >= 0 {
			id = rest[:i]
			title = strings.TrimSpace(rest[i:])
			if strings.HasPrefix(title, "[") && strings.HasSuffix(title, "]") {
				title = title[1 : len(title)-1]
			}
		}
		if id == "" {
			return fmt.Errorf("the subgraph has no name")
		}
		d.Groups = append(d.Groups, &Group{ID: id, Title: label(title)})
		d.groupNow = len(d.Groups) - 1
		return nil
	case "end":
		if d.groupNow < 0 {
			return fmt.Errorf("end closes no subgraph")
		}
		d.groupNow = -1
		return nil
	case "direction", "classDef", "class", "style", "linkStyle", "click":
		return fmt.Errorf("%s is not drawn by this writer", f[0])
	}
	rest := t
	var prev *Node
	var pending *Edge
	for {
		n, after, err := d.nodeRef(rest)
		if err != nil {
			return err
		}
		if pending != nil {
			pending.To = n.ID
			d.Edges = append(d.Edges, pending)
		}
		prev = n
		rest = strings.TrimSpace(after)
		if rest == "" {
			// A node named alone inside a subgraph belongs to it, as in
			// Mermaid, wherever it was first named.
			if pending == nil && d.groupNow >= 0 {
				n.Group = d.groupNow
			}
			return nil
		}
		e, after2, err := arrow(rest)
		if err != nil {
			return err
		}
		e.From = prev.ID
		pending = e
		rest = strings.TrimSpace(after2)
		if rest == "" {
			return fmt.Errorf("the arrow leads to no node")
		}
	}
}

// nodeRef reads a node's id and, when given, its shape and label.
func (d *Diagram) nodeRef(s string) (*Node, string, error) {
	i := 0
	for i < len(s) {
		r := rune(s[i])
		if r >= 0x80 {
			i++
			continue
		}
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' {
			i++
			continue
		}
		if r == '-' && i+1 < len(s) && s[i+1] != '-' && s[i+1] != '.' && s[i+1] != '>' && s[i+1] != ' ' {
			i++
			continue
		}
		if r == '.' && i+1 < len(s) && (unicode.IsLetter(rune(s[i+1])) || unicode.IsDigit(rune(s[i+1]))) {
			i++
			continue
		}
		break
	}
	if i == 0 {
		return nil, "", fmt.Errorf("a node's name is expected at %q", s)
	}
	id := s[:i]
	n := d.node(id)
	rest := s[i:]
	for _, sh := range shapes {
		if !strings.HasPrefix(rest, sh.open) {
			continue
		}
		body := rest[len(sh.open):]
		var text string
		if strings.HasPrefix(body, `"`) {
			q := strings.Index(body[1:], `"`)
			if q < 0 {
				return nil, "", fmt.Errorf("the label of %s has no closing quote", id)
			}
			text = body[1 : q+1]
			body = body[q+2:]
			if !strings.HasPrefix(body, sh.close) {
				continue
			}
		} else {
			c := strings.Index(body, sh.close)
			if c < 0 {
				continue
			}
			text = body[:c]
			body = body[c:]
		}
		n.Label, n.Shape, n.given = label(text), sh.shape, true
		return n, body[len(sh.close):], nil
	}
	if strings.HasPrefix(rest, "[") || strings.HasPrefix(rest, "(") || strings.HasPrefix(rest, "{") || strings.HasPrefix(rest, ">") {
		return nil, "", fmt.Errorf("the shape of %s is not one this writer draws", id)
	}
	return n, rest, nil
}

// arrow reads an arrow and its label, if any.
func arrow(s string) (*Edge, string, error) {
	e := &Edge{Head: true}
	switch {
	case strings.HasPrefix(s, "-.->"):
		e.Dashed, s = true, s[4:]
	case strings.HasPrefix(s, "-.-"):
		e.Dashed, e.Head, s = true, false, s[3:]
	case strings.HasPrefix(s, "==>"):
		s = s[3:]
	case strings.HasPrefix(s, "-->"):
		s = s[3:]
	case strings.HasPrefix(s, "---"):
		e.Head, s = false, s[3:]
	default:
		return nil, "", fmt.Errorf("an arrow (-->, -.->, ---) is expected at %q", s)
	}
	if strings.HasPrefix(s, "|") {
		c := strings.Index(s[1:], "|")
		if c < 0 {
			return nil, "", fmt.Errorf("the arrow's label has no closing |")
		}
		e.Label = label(s[1 : c+1])
		s = s[c+2:]
	}
	return e, s, nil
}

// The state diagram.

func (d *Diagram) stateLine(_ int, t string) error {
	from, rest, ok := strings.Cut(t, "-->")
	if !ok {
		return fmt.Errorf("a transition (from --> to : event) is expected")
	}
	to, event, _ := strings.Cut(rest, ":")
	from, to = strings.TrimSpace(from), strings.TrimSpace(to)
	if from == "" || to == "" || strings.ContainsAny(from+to, " {}") {
		return fmt.Errorf("a transition names one state on each side of -->")
	}
	if to == "[*]" && from == "[*]" {
		return fmt.Errorf("a transition from [*] to [*] draws nothing")
	}
	state := func(id string, end bool) *Node {
		if id == "[*]" {
			if end {
				n := d.node("[*]end")
				n.Label, n.Shape = "", End
				return n
			}
			n := d.node("[*]start")
			n.Label, n.Shape = "", Start
			return n
		}
		n := d.node(id)
		n.Shape = Rounded
		return n
	}
	f, g := state(from, false), state(to, true)
	d.Edges = append(d.Edges, &Edge{From: f.ID, To: g.ID, Label: label(event), Head: true})
	return nil
}

// The entity diagram.

var cards = []string{"||", "|o", "o|", "}o", "o{", "}|", "|{"}

func (d *Diagram) entityLine(_ int, t string) error {
	if d.entityOpen != nil {
		if t == "}" {
			d.entityOpen = nil
			return nil
		}
		f := strings.Fields(t)
		if len(f) < 2 {
			return fmt.Errorf("an attribute is a type and a name")
		}
		a := Attr{Type: f[0], Name: f[1]}
		rest := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(strings.TrimPrefix(t, f[0])), f[1]))
		if c := strings.Index(rest, `"`); c >= 0 {
			rest = strings.TrimSpace(rest[:c])
		}
		a.Keys = strings.Join(strings.Fields(strings.ReplaceAll(rest, ",", ", ")), " ")
		d.entityOpen.Attrs = append(d.entityOpen.Attrs, a)
		return nil
	}
	if strings.HasSuffix(t, "{") {
		id := strings.TrimSpace(strings.TrimSuffix(t, "{"))
		if id == "" || strings.ContainsAny(id, " :") {
			return fmt.Errorf("an entity block starts with one name and {")
		}
		n := d.node(id)
		n.Shape, n.given = Table, true
		d.entityOpen = n
		return nil
	}
	left, lbl, ok := strings.Cut(t, ":")
	f := strings.Fields(left)
	if !ok || len(f) != 3 {
		if len(f) == 1 && !ok {
			d.node(f[0]).Shape = Table
			return nil
		}
		return fmt.Errorf("a relation (A ||--o{ B : label) is expected")
	}
	rel := f[1]
	var line string
	var from, to string
	for _, c := range cards {
		if strings.HasPrefix(rel, c) {
			from = c
			break
		}
	}
	for _, c := range cards {
		if strings.HasSuffix(rel, c) {
			to = c
			break
		}
	}
	if from == "" || to == "" || len(rel) != len(from)+2+len(to) {
		return fmt.Errorf("%q is not a relation this writer draws", rel)
	}
	line = rel[len(from) : len(from)+2]
	if line != "--" && line != ".." {
		return fmt.Errorf("%q is not a relation this writer draws", rel)
	}
	a, b := d.node(f[0]), d.node(f[2])
	a.Shape, b.Shape = Table, Table
	d.Edges = append(d.Edges, &Edge{From: a.ID, To: b.ID, Label: label(lbl), Dashed: line == "..", Card: [2]string{from, to}})
	return nil
}

// The sequence diagram.

var messageArrows = []struct {
	mark         string
	dashed, head bool
}{
	{"-->>", true, true},
	{"->>", false, true},
	{"--x", true, true},
	{"-x", false, true},
	{"--)", true, true},
	{"-)", false, true},
	{"-->", true, false},
	{"->", false, false},
}

func (d *Diagram) participant(id string) *Participant {
	for _, p := range d.Participants {
		if p.ID == id {
			return p
		}
	}
	p := &Participant{ID: id, Label: id}
	d.Participants = append(d.Participants, p)
	return p
}

func (d *Diagram) sequenceLine(_ int, t string) error {
	f := strings.Fields(t)
	switch f[0] {
	case "participant", "actor":
		rest := strings.TrimSpace(strings.TrimPrefix(t, f[0]))
		id, as, ok := strings.Cut(rest, " as ")
		id = strings.TrimSpace(id)
		if id == "" || strings.Contains(id, " ") {
			return fmt.Errorf("a participant is one name, then as and its label")
		}
		p := d.participant(id)
		if ok {
			p.Label = label(as)
		}
		return nil
	case "Note", "note":
		rest := strings.TrimSpace(t[len(f[0]):])
		place := ""
		for _, pl := range []string{"over", "left of", "right of"} {
			if strings.HasPrefix(rest, pl+" ") {
				place, rest = pl, strings.TrimSpace(rest[len(pl):])
				break
			}
		}
		who, text, ok := strings.Cut(rest, ":")
		if place == "" || !ok {
			return fmt.Errorf("a note is Note over A,B: text, or left of or right of one")
		}
		names := strings.Split(who, ",")
		if len(names) > 2 || (place != "over" && len(names) != 1) {
			return fmt.Errorf("a note is over one or two participants, or left or right of one")
		}
		s := &Step{Note: true, Place: place, Text: label(text)}
		s.From = d.participant(strings.TrimSpace(names[0])).ID
		s.To = s.From
		if len(names) == 2 {
			s.To = d.participant(strings.TrimSpace(names[1])).ID
		}
		d.Steps = append(d.Steps, s)
		return nil
	case "autonumber", "activate", "deactivate", "loop", "alt", "else", "opt", "par", "and", "rect", "critical", "break", "end", "box", "create", "destroy":
		return fmt.Errorf("%s is not drawn by this writer", f[0])
	}
	head, text, ok := strings.Cut(t, ":")
	if !ok {
		return fmt.Errorf("a message is A->>B: text")
	}
	for _, a := range messageArrows {
		i := strings.Index(head, a.mark)
		if i < 0 {
			continue
		}
		from := strings.TrimSpace(head[:i])
		to := strings.TrimSpace(strings.TrimLeft(head[i+len(a.mark):], "+-"))
		if from == "" || to == "" || strings.Contains(from, " ") || strings.Contains(to, " ") {
			return fmt.Errorf("a message names one participant on each side of its arrow")
		}
		d.Steps = append(d.Steps, &Step{From: d.participant(from).ID, To: d.participant(to).ID, Text: label(text), Dashed: a.dashed, Head: a.head})
		return nil
	}
	return fmt.Errorf("a message's arrow (->>, -->>, ->, -->) is expected")
}
