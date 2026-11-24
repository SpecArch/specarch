package diagram

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"unicode/utf8"
)

// approx measures a text as half its size per character, which is near
// enough for a layout test.
func approx(s string, size float64, bold bool) float64 {
	w := float64(utf8.RuneCountInString(s)) * size * 0.5
	if bold {
		w *= 1.1
	}
	return w
}

var fenced = regexp.MustCompile("(?s)```mermaid\n(.*?)```")

// TestEveryGeneratedDiagramIsDrawn reads every Mermaid block of every
// Markdown file in the repository, the documents the generators wrote,
// the expected outputs of the tests and the examples' documents in every
// language among them, and draws each: a block this package cannot read
// would be printed as its source in a pdf, so none may be.
func TestEveryGeneratedDiagramIsDrawn(t *testing.T) {
	root := filepath.Join("..", "..")
	blocks := 0
	err := filepath.WalkDir(root, func(p string, e fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if e.IsDir() {
			switch e.Name() {
			case "third_party", "history", ".git", "node_modules":
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(p, ".md") {
			return nil
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		text := string(data)
		if !strings.Contains(text, "```mermaid") {
			return nil
		}
		for _, m := range fenced.FindAllStringSubmatch(text, -1) {
			blocks++
			g, err := Draw(m[1], approx, Fit{Width: 480, Height: 700})
			if err != nil {
				t.Errorf("%s: %v", p, err)
				continue
			}
			if g.W <= 0 || g.H <= 0 || len(g.Paths) == 0 {
				t.Errorf("%s: a block drew nothing:\n%s", p, m[1])
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if blocks < 100 {
		t.Fatalf("only %d Mermaid blocks found; the walk missed the generated documents", blocks)
	}
}

func TestParseFlowchartShapesAndLabels(t *testing.T) {
	d, err := Parse(`flowchart LR
  a["first<br/>line"] -->|"goes"| b(["second"])
  b -.-> c{"choice?"}
  subgraph zone_x["zone x"]
    c
    e[("store")]
  end
  c -->|plain label| e
`)
	if err != nil {
		t.Fatal(err)
	}
	if d.Dir != "LR" || len(d.Nodes) != 4 || len(d.Edges) != 3 || len(d.Groups) != 1 {
		t.Fatalf("got %+v", d)
	}
	if d.Nodes[0].Label != "first\nline" || d.Nodes[1].Shape != Stadium || d.Nodes[2].Shape != Diamond || d.Nodes[3].Shape != Cylinder {
		t.Errorf("shapes or labels wrong: %+v %+v %+v %+v", d.Nodes[0], d.Nodes[1], d.Nodes[2], d.Nodes[3])
	}
	if d.Edges[0].Label != "goes" || !d.Edges[1].Dashed || d.Edges[2].Label != "plain label" {
		t.Errorf("edges wrong: %+v %+v %+v", d.Edges[0], d.Edges[1], d.Edges[2])
	}
	if d.Nodes[3].Group != 0 || d.Nodes[2].Group != 0 || d.Nodes[0].Group != -1 {
		t.Errorf("the subgraph holds the wrong nodes")
	}
}

func TestParseEntityAndSequenceAndState(t *testing.T) {
	e, err := Parse(`erDiagram
  Member ||--o{ Loan : borrows
  Loan {
    uuid id PK
    uuid member FK
    date due
  }
`)
	if err != nil {
		t.Fatal(err)
	}
	if len(e.Nodes) != 2 || len(e.Nodes[1].Attrs) != 3 || e.Edges[0].Card != [2]string{"||", "o{"} || e.Nodes[1].Attrs[0].Keys != "PK" {
		t.Errorf("entity diagram read wrong: %+v %+v", e.Nodes[1], e.Edges[0])
	}
	s, err := Parse(`sequenceDiagram
  participant C as Client
  participant S as Library
  C->>S: GET /loans/{id}
  Note over C,S: in the envelope
  S-->>C: 200 Loan
`)
	if err != nil {
		t.Fatal(err)
	}
	if len(s.Participants) != 2 || s.Participants[1].Label != "Library" || len(s.Steps) != 3 || !s.Steps[2].Dashed || !s.Steps[1].Note {
		t.Errorf("sequence read wrong: %+v", s.Steps)
	}
	st, err := Parse(`stateDiagram-v2
  [*] --> open
  open --> paid : pay
  paid --> [*]
`)
	if err != nil {
		t.Fatal(err)
	}
	if len(st.Nodes) != 4 || st.Nodes[0].Shape != Start || st.Nodes[3].Shape != End || st.Edges[1].Label != "pay" {
		t.Errorf("state diagram read wrong: %+v", st.Nodes)
	}
}

// A line this package does not draw is an error naming the line, never
// a line left out.
func TestUnknownLineIsAnErrorNamingIt(t *testing.T) {
	for _, src := range []string{
		"flowchart LR\n  a --> b\n  classDef x fill:#f00\n",
		"sequenceDiagram\n  A->>B: hi\n  loop every minute\n  end\n",
		"pie\n  \"a\" : 1\n",
		"erDiagram\n  A {\n    string x\n",
	} {
		_, err := Parse(src)
		if err == nil {
			t.Errorf("no error for:\n%s", src)
		}
	}
	_, err := Parse("flowchart LR\n  a --> b\n  classDef x fill:#f00\n")
	if err == nil || !strings.Contains(err.Error(), "line 3") || !strings.Contains(err.Error(), "classDef") {
		t.Errorf("the error does not name the line and what it holds: %v", err)
	}
}

// The same source draws the same picture every time.
func TestDrawingIsTheSameEveryTime(t *testing.T) {
	src := "flowchart TD\n  a --> b\n  a --> c\n  b --> d\n  c --> d\n  d --> a\n  e[\"alone\"]\n"
	first, err := Draw(src, approx, Fit{Width: 480, Height: 700})
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 20; i++ {
		again, _ := Draw(src, approx, Fit{Width: 480, Height: 700})
		if len(again.Paths) != len(first.Paths) || again.W != first.W || again.H != first.H {
			t.Fatalf("run %d drew another picture", i)
		}
		for k := range again.Texts {
			if again.Texts[k] != first.Texts[k] {
				t.Fatalf("run %d placed a text elsewhere", i)
			}
		}
	}
}

// Nodes of one rank never overlap, and every node lies inside the
// drawing.
func TestNodesDoNotOverlap(t *testing.T) {
	src := "flowchart LR\n  a --> b\n  a --> c\n  a --> d\n  b --> e\n  c --> e\n  d -->|\"a long label here\"| e\n"
	d, _ := Parse(src)
	g := d.graph(approx, "LR", Fit{})
	if g.W <= 0 || g.H <= 0 {
		t.Fatal("empty drawing")
	}
	type box struct{ x0, y0, x1, y1 float64 }
	var boxes []box
	for _, p := range g.Paths {
		if p.Fill == nil || *p.Fill != nodeFill {
			continue
		}
		b := box{1e9, 1e9, -1e9, -1e9}
		for _, s := range p.Segs {
			for _, q := range s.Pts {
				b.x0, b.y0 = min(b.x0, q.X), min(b.y0, q.Y)
				b.x1, b.y1 = max(b.x1, q.X), max(b.y1, q.Y)
			}
		}
		if b.x0 < 0 || b.y0 < 0 || b.x1 > g.W || b.y1 > g.H {
			t.Errorf("a node lies outside the drawing: %+v in %vx%v", b, g.W, g.H)
		}
		boxes = append(boxes, b)
	}
	if len(boxes) != 5 {
		t.Fatalf("%d nodes drawn, want 5", len(boxes))
	}
	for i := range boxes {
		for j := i + 1; j < len(boxes); j++ {
			a, b := boxes[i], boxes[j]
			if a.x0 < b.x1 && b.x0 < a.x1 && a.y0 < b.y1 && b.y0 < a.y1 {
				t.Errorf("nodes %d and %d overlap: %+v %+v", i, j, a, b)
			}
		}
	}
}
