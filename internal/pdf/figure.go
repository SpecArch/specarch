package pdf

import (
	"fmt"
	"math"
	"strings"

	"github.com/SpecArch/specarch/internal/diagram"
)

// Figures are the descriptions of a drawn diagram for a reader who
// cannot see it, each with %s for the names it draws.
type Figures struct {
	Flowchart string
	State     string
	Entity    string
	Sequence  string
}

// figure draws a Mermaid block as a picture, scaled to the width and to
// one page, as a Figure of the structure tree with its description. It
// returns the reason when the block is not one internal/diagram draws,
// and draws nothing then.
func (b *book) figure(parent *elem, src string, x, avail float64) error {
	g, s, err := b.drawing(src, avail)
	if err != nil {
		return err
	}
	b.y += 4
	b.ensure(g.H*s + 4)
	e := b.add(parent, "Figure")
	e.alt = b.figureAlt(g)
	x0 := x + (avail-g.W*s)/2
	y0 := b.y
	e.bbox = &[4]float64{x0, pageH - y0 - g.H*s, x0 + g.W*s, pageH - y0}
	pt := func(p diagram.Point) string {
		return num(x0+p.X*s) + " " + num(pageH-(y0+p.Y*s))
	}
	for _, p := range g.Paths {
		var ops strings.Builder
		ops.WriteString("q ")
		if p.Fill != nil {
			fmt.Fprintf(&ops, "%s rg ", colour(rgb(*p.Fill)))
		}
		if p.Stroke != nil {
			fmt.Fprintf(&ops, "%s RG %s w ", colour(rgb(*p.Stroke)), num(math.Max(p.Width*s, 0.25)))
			if p.Dash {
				fmt.Fprintf(&ops, "[%s %s] 0 d ", num(2.5*s), num(2*s))
			}
		}
		for _, sg := range p.Segs {
			switch sg.Op {
			case 'M':
				fmt.Fprintf(&ops, "%s m ", pt(sg.Pts[0]))
			case 'L':
				fmt.Fprintf(&ops, "%s l ", pt(sg.Pts[0]))
			case 'C':
				fmt.Fprintf(&ops, "%s %s %s c ", pt(sg.Pts[0]), pt(sg.Pts[1]), pt(sg.Pts[2]))
			case 'Z':
				ops.WriteString("h ")
			}
		}
		switch {
		case p.Fill != nil && p.Stroke != nil:
			ops.WriteString("B Q")
		case p.Fill != nil:
			ops.WriteString("f Q")
		default:
			ops.WriteString("S Q")
		}
		o := op{kind: pathOp, text: ops.String()}
		b.mark(e, &o)
		b.cur.ops = append(b.cur.ops, o)
	}
	for _, t := range g.Texts {
		f := regular
		if t.Bold {
			f = bold
		}
		b.draw(e, x0+t.X*s, y0+t.Y*s, f, t.Size*s, rgb(t.Color), t.S)
	}
	b.y += g.H*s + 8
	return nil
}

// drawing lays a Mermaid block out for a width and gives the scale that
// fits it to that width and to one page.
func (b *book) drawing(src string, avail float64) (*diagram.Drawing, float64, error) {
	measure := func(s string, size float64, isBold bool) float64 {
		f := regular
		if isBold {
			f = bold
		}
		return b.m[f].width(s, size)
	}
	room := bottom - top - 20
	g, err := diagram.Draw(src, measure, diagram.Fit{Width: avail, Height: room})
	if err != nil {
		return nil, 0, err
	}
	s := 1.0
	if g.W > avail {
		s = avail / g.W
	}
	if g.H*s > room {
		s = room / g.H
	}
	return g, s, nil
}

// figureHeight is the height a block takes when it is a diagram drawn
// right after a heading, so the heading goes to the next page with it;
// else nothing.
func (b *book) figureHeight(bl *block, avail float64) float64 {
	if bl.Kind != codeBlock || bl.Lang != "mermaid" {
		return 0
	}
	g, s, err := b.drawing(bl.Code, avail)
	if err != nil {
		return 0
	}
	return g.H*s + 4
}

// figureAlt describes a drawn diagram: its kind and what it draws.
func (b *book) figureAlt(g *diagram.Drawing) string {
	f := b.opt.Texts.Figures
	format := map[diagram.Kind]string{diagram.Flowchart: f.Flowchart, diagram.State: f.State, diagram.Entity: f.Entity, diagram.Sequence: f.Sequence}[g.Kind]
	if format == "" {
		format = "%s"
	}
	names := make([]string, 0, len(g.Names))
	for _, n := range g.Names {
		if n = strings.Join(strings.Fields(n), " "); n != "" {
			names = append(names, n)
		}
	}
	return fmt.Sprintf(format, strings.Join(names, ", "))
}
