package diagram

import (
	"math"
	"strings"
	"unicode/utf8"
)

// Measure is the width in points of a text at a size, in the bold face or
// the regular one, as the writer that draws it sets it.
type Measure func(text string, size float64, bold bool) float64

// Fit is the room a drawing should fit: its width, and the height of a
// page. A graph is laid out across or down, whichever fits it larger.
type Fit struct{ Width, Height float64 }

// Color is red, green and blue from 0 to 1.
type Color [3]float64

// Point is a point in points, y growing downward.
type Point struct{ X, Y float64 }

// Seg is one segment of a path: M moves, L draws a line, C a cubic curve
// through two control points, Z closes the path.
type Seg struct {
	Op  byte
	Pts []Point
}

// Path is a shape: filled, stroked or both.
type Path struct {
	Segs   []Seg
	Fill   *Color
	Stroke *Color
	Width  float64
	Dash   bool
}

// Text is one line of text at its baseline.
type Text struct {
	X, Y  float64
	Size  float64
	Bold  bool
	Color Color
	S     string
}

// Drawing is a laid-out diagram: its size, its paths in the order they
// are drawn, then its texts, and a description for a reader who cannot
// see it.
type Drawing struct {
	W, H  float64
	Paths []Path
	Texts []Text
	Names []string // the nodes, states, entities or participants, in order
	Kind  Kind
}

var (
	ink      = Color{0.15, 0.15, 0.18}
	soft     = Color{0.35, 0.35, 0.4}
	line     = Color{0.35, 0.38, 0.45}
	nodeFill = Color{0.93, 0.95, 0.99}
	nodeEdge = Color{0.42, 0.5, 0.68}
	headFill = Color{0.85, 0.89, 0.96}
	white    = Color{1, 1, 1}
	group    = Color{0.97, 0.97, 0.94}
	groupEdg = Color{0.7, 0.7, 0.6}
	noteFill = Color{1, 0.97, 0.82}
	noteEdge = Color{0.8, 0.72, 0.4}
)

const (
	nodeSize  = 8.0
	labelSize = 7.0
	lead      = 1.25
)

// Draw parses a Mermaid diagram and lays it out.
func Draw(src string, m Measure, fit Fit) (*Drawing, error) {
	d, err := Parse(src)
	if err != nil {
		return nil, err
	}
	return d.Draw(m, fit), nil
}

// Draw lays a parsed diagram out.
func (d *Diagram) Draw(m Measure, fit Fit) *Drawing {
	var out *Drawing
	if d.Kind == Sequence {
		out = d.sequence(m)
	} else {
		dirs := []string{d.Dir, "LR"}
		if d.Kind == State || d.Kind == Entity {
			dirs = []string{"TD", "LR"}
		}
		if dirs[0] == "LR" {
			dirs[1] = "TD"
		}
		best, bestScale := (*Drawing)(nil), -1.0
		for _, dir := range dirs {
			g := d.graph(m, dir, fit)
			if s := scaleFor(g, fit); s > bestScale+0.02 {
				best, bestScale = g, s
			}
		}
		out = best
	}
	out.Kind = d.Kind
	return out
}

// scaleFor is how much a drawing is scaled down to fit.
func scaleFor(g *Drawing, fit Fit) float64 {
	s := 1.0
	if fit.Width > 0 && g.W > fit.Width {
		s = fit.Width / g.W
	}
	if fit.Height > 0 && g.H*s > fit.Height {
		s = fit.Height / g.H
	}
	return s
}

// wrap breaks a text into lines no wider than max, at spaces, and a word
// wider than max where it fills the line.
func wrap(m Measure, s string, size float64, bold bool, max float64) []string {
	var out []string
	for _, para := range strings.Split(s, "\n") {
		words := strings.Fields(para)
		if len(words) == 0 {
			out = append(out, "")
			continue
		}
		cur := ""
		for _, w := range words {
			try := w
			if cur != "" {
				try = cur + " " + w
			}
			if m(try, size, bold) <= max || cur == "" && m(w, size, bold) <= max {
				cur = try
				continue
			}
			if cur != "" {
				out = append(out, cur)
				cur = ""
			}
			for m(w, size, bold) > max && utf8.RuneCountInString(w) > 1 {
				cut := 0
				for i := range w {
					if i > 0 && m(w[:i], size, bold) > max {
						break
					}
					cut = i
				}
				if cut == 0 {
					_, cut = utf8.DecodeRuneInString(w)
				} else if p := strings.LastIndexAny(w[:cut], "/-_.,:"); p > 0 {
					cut = p + 1
				}
				out = append(out, w[:cut])
				w = w[cut:]
			}
			cur = w
		}
		out = append(out, cur)
	}
	return out
}

// widest is the width of the widest line.
func widest(m Measure, lines []string, size float64, bold bool) float64 {
	w := 0.0
	for _, l := range lines {
		w = math.Max(w, m(l, size, bold))
	}
	return w
}

// centred adds lines of text centred on (cx, cy).
func (g *Drawing) centred(m Measure, lines []string, cx, cy, size float64, bold bool, c Color) {
	h := float64(len(lines)) * size * lead
	y := cy - h/2 + size*0.95
	for _, l := range lines {
		if l != "" {
			g.Texts = append(g.Texts, Text{X: cx - m(l, size, bold)/2, Y: y, Size: size, Bold: bold, Color: c, S: l})
		}
		y += size * lead
	}
}

func (g *Drawing) path(p Path) { g.Paths = append(g.Paths, p) }

func col(c Color) *Color { return &c }

// rect is a rectangle path.
func rect(x, y, w, h float64) []Seg {
	return []Seg{{'M', []Point{{x, y}}}, {'L', []Point{{x + w, y}}}, {'L', []Point{{x + w, y + h}}}, {'L', []Point{{x, y + h}}}, {'Z', nil}}
}

// polygon is a closed path through points.
func polygon(pts ...Point) []Seg {
	s := []Seg{{'M', []Point{pts[0]}}}
	for _, p := range pts[1:] {
		s = append(s, Seg{'L', []Point{p}})
	}
	return append(s, Seg{'Z', nil})
}

// kappa places the control points of a cubic curve that draws a quarter
// of a circle.
const kappa = 0.5523

// ellipse is an ellipse path around (cx, cy).
func ellipse(cx, cy, rx, ry float64) []Seg {
	kx, ky := rx*kappa, ry*kappa
	return []Seg{
		{'M', []Point{{cx + rx, cy}}},
		{'C', []Point{{cx + rx, cy + ky}, {cx + kx, cy + ry}, {cx, cy + ry}}},
		{'C', []Point{{cx - kx, cy + ry}, {cx - rx, cy + ky}, {cx - rx, cy}}},
		{'C', []Point{{cx - rx, cy - ky}, {cx - kx, cy - ry}, {cx, cy - ry}}},
		{'C', []Point{{cx + kx, cy - ry}, {cx + rx, cy - ky}, {cx + rx, cy}}},
		{'Z', nil},
	}
}

// rounded is a rectangle with corners of radius r.
func rounded(x, y, w, h, r float64) []Seg {
	r = math.Min(r, math.Min(w, h)/2)
	k := r * (1 - kappa)
	return []Seg{
		{'M', []Point{{x + r, y}}},
		{'L', []Point{{x + w - r, y}}},
		{'C', []Point{{x + w - k, y}, {x + w, y + k}, {x + w, y + r}}},
		{'L', []Point{{x + w, y + h - r}}},
		{'C', []Point{{x + w, y + h - k}, {x + w - k, y + h}, {x + w - r, y + h}}},
		{'L', []Point{{x + r, y + h}}},
		{'C', []Point{{x + k, y + h}, {x, y + h - k}, {x, y + h - r}}},
		{'L', []Point{{x, y + r}}},
		{'C', []Point{{x, y + k}, {x + k, y}, {x + r, y}}},
		{'Z', nil},
	}
}

// arrowHead is a filled arrowhead whose tip is at p, pointing along the
// direction from q to p.
func (g *Drawing) arrowHead(q, p Point, c Color) {
	dx, dy := p.X-q.X, p.Y-q.Y
	l := math.Hypot(dx, dy)
	if l == 0 {
		return
	}
	ux, uy := dx/l, dy/l
	const length, half = 6.0, 2.6
	bx, by := p.X-ux*length, p.Y-uy*length
	g.path(Path{Segs: polygon(p, Point{bx - uy*half, by + ux*half}, Point{bx + uy*half, by - ux*half}), Fill: col(c)})
}

// polyline is an open path through points.
func polyline(pts []Point) []Seg {
	s := []Seg{{'M', []Point{pts[0]}}}
	for _, p := range pts[1:] {
		s = append(s, Seg{'L', []Point{p}})
	}
	return s
}
